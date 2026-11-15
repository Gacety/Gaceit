/**
 * Local HLR adaptation: p=2^(-elapsedDays/h), h=2^(weights·features).
 * Based on Settles & Meeder (ACL 2016), https://aclanthology.org/P16-1174/.
 * Cold-start coefficients and the relearning/date policies below are app-specific,
 * not Duolingo-trained parameters, SM-20, or a LECTOR semantic model.
 */
export const REVIEW_VERSION = 1;
export const REVIEW_ALGORITHM = 'hlr-local-v1';
export const STUDY_ZONE = 'Asia/Taipei';
export const DAY = 86400000;
export const RELEARN_DELAY = 10 * 60 * 1000;
export const MIN_SPACED_GAP = 12 * 60 * 60 * 1000;
export const TARGET_RECALL = .85;
const LN2 = Math.log(2);
const PRIOR_WEIGHTS = [1, 1.7, -1.2];
const clamp = (value, min, max) => Math.min(max, Math.max(min, value));
const DAY_FORMATTER = new Intl.DateTimeFormat('en-CA', {timeZone: STUDY_ZONE, year:'numeric',month:'2-digit',day:'2-digit'});

export function studyDay(timestamp = Date.now()) {
  const parts = DAY_FORMATTER.formatToParts(new Date(timestamp));
  return ['year','month','day'].map(type => parts.find(part => part.type === type).value).join('-');
}

export function dayStart(timestamp, offset = 0) {
  const start = Date.parse(`${studyDay(timestamp)}T00:00:00+08:00`);
  return start + offset * DAY;
}

export function validReview(value) {
  return value?.version === REVIEW_VERSION && value.algorithm === REVIEW_ALGORITHM &&
    ['new','learning','relearning','review'].includes(value.stage) &&
    Array.isArray(value.weights) && value.weights.length === 3 && value.weights.every(Number.isFinite) &&
    ['right','wrong','unknown','uncertain','streak','reps','lapses','trained','priorRight','priorWrong','priorUnknown','intervalDays'].every(key => Number.isFinite(value[key]) && value[key] >= 0) &&
    (value.lastReviewAt === null || Number.isFinite(Date.parse(value.lastReviewAt))) &&
    (value.dueAt === null || Number.isFinite(Date.parse(value.dueAt)));
}

export function initialReview(question, record = {}) {
  // Timeless imported snapshots and pre-scheduler attempts are only weak priors.
  // They never acquire invented dates or become spaced-review observations.
  const attempts = record.attempts || [];
  const priorRight = (question.history?.result === '正确' ? .25 : 0) + Math.min(1, attempts.filter(a=>a.correct===true).length * .25);
  const priorWrong = (question.history?.result === '错误' ? .5 : 0) + Math.min(1, attempts.filter(a=>a.correct===false).length * .25);
  return {
    version:REVIEW_VERSION,algorithm:REVIEW_ALGORITHM,stage:'new',weights:[...PRIOR_WEIGHTS],
    // An undated source 'dont know' flag is not proof of a failed retrieval.
    priorRight,priorWrong,priorUnknown:0,
    right:0,wrong:0,unknown:0,uncertain:0,streak:0,reps:0,lapses:0,trained:0,
    lastReviewAt:null,dueAt:null,lastOutcome:null,intervalDays:0,halfLifeDays:null,
  };
}

export function reviewState(question, record = {}) {
  return validReview(record.review) ? record.review : initialReview(question,record);
}

export function memoryFeatures(state) {
  return [1, Math.sqrt(state.right + state.priorRight), Math.sqrt(state.wrong + state.unknown + .5 * state.uncertain + state.priorWrong + state.priorUnknown)];
}

export function halfLife(state) {
  const x = memoryFeatures(state);
  const exponent = clamp(state.weights.reduce((sum,weight,i)=>sum+weight*x[i],0),-6,12);
  return clamp(2 ** exponent, 15/1440, 3650);
}

export function recallProbability(state, elapsedDays) {
  return clamp(2 ** (-Math.max(0,elapsedDays)/halfLife(state)), .0001, .9999);
}

function latestExposure(state,record) {
  if(!state.lastReviewAt)return null;
  let last=Date.parse(state.lastReviewAt);
  for(const attempt of record.attempts||[]){
    const time=Date.parse(attempt.time);
    // Once a model exists, any later dated practice is a real exposure, even
    // when the question was temporarily removed from the scheduled wrong book.
    if(Number.isFinite(time))last=Math.max(last,time);
  }
  return last;
}

export function nextReviewAt(question,record) {
  const state=reviewState(question,record);
  if(!state.dueAt)return null;
  const exposure=latestExposure(state,record);
  const cooldown=state.stage==='review'?MIN_SPACED_GAP:RELEARN_DELAY;
  return new Date(Math.max(Date.parse(state.dueAt),exposure===null?0:exposure+cooldown)).toISOString();
}

export function isReviewDue(question, record, now = Date.now()) {
  if (!record?.wrong) return false;
  if (question.errata?.includes('标答存疑')) return false;
  const readyAt=nextReviewAt(question,record);
  return !readyAt || Date.parse(readyAt) <= now;
}

export function reviewCounts(questions, records, now = Date.now()) {
  const today=studyDay(now);
  let due=0,uninitialized=0,laterToday=0,scheduled=0,reviewedToday=0,total=0,blocked=0;
  for (const q of questions) {
    const record=records[q.id];
    if (validReview(record?.review) && record.review.lastReviewAt && studyDay(Date.parse(record.review.lastReviewAt))===today) reviewedToday++;
    if (!record?.wrong) continue;
    total++;
    if(q.errata?.includes('标答存疑')){blocked++;continue;}
    const state=reviewState(q,record);
    if (state.stage==='new') uninitialized++;
    if (isReviewDue(q,record,now)) due++;
    else {
      scheduled++;
      const readyAt=nextReviewAt(q,record);
      if (readyAt && studyDay(Date.parse(readyAt))===today) laterToday++;
    }
  }
  return {today,total,due,uninitialized,laterToday,scheduled,reviewedToday,blocked};
}

function fitWeights(state, elapsedDays, observedRecall) {
  const h=halfLife(state),p=recallProbability(state,elapsedDays),features=memoryFeatures(state);
  // Gradient of squared recall loss. No artificial half-life training target is
  // inferred from an individual binary answer; regularize toward cold priors.
  const derivative=2*(p-observedRecall)*LN2*LN2*p*(elapsedDays/h);
  const rate=.2/Math.sqrt(1+state.trained);
  return state.weights.map((weight,i)=>{
    const gradient=clamp(derivative*features[i]+.01*(weight-PRIOR_WEIGHTS[i]),-1,1);
    const next=weight-rate*gradient;
    return i===0 ? clamp(next,-4,8) : i===1 ? clamp(next,.25,3) : clamp(next,-4,-.25);
  });
}

export function applyReview(question, record, {correct,quality='independent',unknown=false,now=Date.now()} = {}) {
  const before=reviewState(question,record);
  const outcome=unknown?'unknown':correct ? (quality==='independent'?'right':'uncertain') : 'wrong';
  const successful=outcome==='right';
  const exposure=latestExposure(before,record);
  const elapsed=exposure===null?null:Math.max(0,now-exposure);
  const elapsedDays=elapsed===null ? null : elapsed/DAY;
  const probability=elapsedDays===null ? null : recallProbability(before,elapsedDays);
  const metadata={schedulePolicy:'spaced-3-7-14-30-v1',algorithm:REVIEW_ALGORITHM,outcome,quality,elapsedDays,recallBefore:probability,qualified:true,reason:''};

  if (question.errata?.includes('标答存疑')) {
    metadata.qualified=false;metadata.blocked=true;metadata.reason='标答存疑，仅记录作答，不自动排期。';
    return {state:before,metadata};
  }
  // A quick correction after seeing the answer is practice, not evidence of
  // stronger long-term memory. Repeated immediate failures do not add lapses.
  const shortRepeat=elapsed!==null && elapsed<RELEARN_DELAY;
  const previousNegative=['wrong','unknown','uncertain'].includes(before.lastOutcome);
  if ((shortRepeat && (successful || previousNegative)) ||
      (successful && ((before.dueAt && Date.parse(before.dueAt)>now) || before.stage==='review' && (!isReviewDue(question,{...record,wrong:true,review:before},now) || elapsed<MIN_SPACED_GAP)))) {
    metadata.qualified=false;
    metadata.reason=shortRepeat?'短间隔重答仅记为练习，不延长长期复习间隔；冷却后再测。':'提前练习不延长既定间隔，冷却后再测可避免高估记忆。';
    return {state:before,metadata};
  }

  const after={...before,weights:[...before.weights],reps:before.reps+1,lastReviewAt:new Date(now).toISOString(),lastOutcome:outcome};
  if (elapsed!==null && elapsed>=MIN_SPACED_GAP && outcome!=='uncertain') {
    after.weights=fitWeights(before,elapsedDays,successful?1:0);
    after.trained++;
  }
  if (!successful) {
    after[outcome==='wrong'?'wrong':outcome==='unknown'?'unknown':'uncertain']++;
    after.streak=0;
    if (before.stage==='review' && outcome!=='uncertain') after.lapses++;
    after.stage=before.reps ? 'relearning' : 'learning';
    after.intervalDays=0;
    after.dueAt=new Date(now+RELEARN_DELAY).toISOString();
    metadata.reason=outcome==='uncertain'?'答对但不确定，先安排短期重学。':unknown?'本次标记不会，安排短期重学。':'本次答错，缩短间隔并安排重学。';
    if(outcome==='uncertain'){after.intervalDays=1;after.dueAt=new Date(now+DAY).toISOString();metadata.reason='答对但不确定或使用提示，安排一天后再次独立回忆。';}
  } else {
    after.right++;
    after.streak++;
    after.stage='review';
    const ladder=[3,7,14,30,60,120,240,365];
    after.intervalDays=before.stage!=='review'?3:(ladder.find(n=>n>before.intervalDays)??365);
    after.dueAt=new Date(now+after.intervalDays*DAY).toISOString();
    metadata.reason=`独立答出，安排约 ${after.intervalDays} 天后复习。`;
  }
  after.halfLifeDays=halfLife(after);
  return {state:after,metadata};
}

export function reviewLabel(question, record, now = Date.now()) {
  const state=reviewState(question,record);
  if (!record?.wrong) return '未纳入错题复习';
  if (question.errata?.includes('标答存疑')) return '标答存疑 · 待核对';
  if (state.stage==='new') return '待建档 · 今日复习';
  if (isReviewDue(question,record,now)) return state.stage==='review'?'已到期':'到期重学';
  return state.stage==='review'?'巩固中':'等待重学';
}

export function formatReviewTime(timestamp, now=Date.now()) {
  if (!timestamp) return '今天（待建档）';
  const time=Date.parse(timestamp),day=studyDay(time),today=studyDay(now);
  if (day===today) return `今天 ${new Intl.DateTimeFormat('zh-CN',{timeZone:STUDY_ZONE,hour:'2-digit',minute:'2-digit',hour12:false}).format(time)}`;
  if (day===studyDay(dayStart(now,1))) return '明天';
  return new Intl.DateTimeFormat('zh-CN',{timeZone:STUDY_ZONE,year:'numeric',month:'numeric',day:'numeric'}).format(time);
}
