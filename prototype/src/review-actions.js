import {applyReview,reviewState,nextReviewAt} from './review-scheduler.js';

const scheduleSnapshot = state => state ? ({stage:state.stage,dueAt:state.dueAt,intervalDays:state.intervalDays,halfLifeDays:state.halfLifeDays,right:state.right,wrong:state.wrong,unknown:state.unknown,uncertain:state.uncertain,streak:state.streak}) : null;

export function recordReviewAttempt(question, record, {id,choices,correct,quality='independent',unknown=false,now=Date.now()}) {
  const outcome=unknown?'unknown':correct?(quality==='independent'?'right':'uncertain'):'wrong';
  const needsReview=record.wrong || outcome!=='right';
  const disputed=question.errata?.includes('标答存疑');
  const memory=needsReview ? (disputed ? {state:reviewState(question,record),metadata:{algorithm:'hlr-local-v1',outcome,quality,qualified:false,blocked:true,reason:'勘误标注标答存疑，仅保留作答，不自动更新复习间隔。'}} : applyReview(question,record,{correct,quality,unknown,now})) : null;
  const attempt={id,choices,correct,time:new Date(now).toISOString(),quality,outcome};
  if (memory) attempt.review={...memory.metadata,before:scheduleSnapshot(record.review),after:scheduleSnapshot(memory.state)};
  const updated={...record,attempts:[...record.attempts,attempt],wrong:record.wrong||outcome!=='right'};
  if(memory){updated.review=memory.state;memory.metadata.readyAt=nextReviewAt(question,updated);attempt.review.readyAt=memory.metadata.readyAt;}
  return {record:updated,attempt,memory};
}

export function undoReviewAttempt(records,questionId,attemptId,before) {
  const current=records[questionId];
  if(current?.attempts?.at(-1)?.id!==attemptId)return records;
  // Revert this answer/schedule, while preserving notes/drawings edited afterward.
  const outcome=current.attempts.at(-1).outcome;
  const producedMembership=before.wrong||outcome!=='right';
  const restored={...current,attempts:before.attempts,wrong:current.wrong===producedMembership?before.wrong:current.wrong};
  if(before.review)restored.review=before.review;
  else delete restored.review;
  return {...records,[questionId]:restored};
}

export function attemptOutcomeLabel(attempt) {
  if(attempt?.outcome==='unknown')return '不会';
  if(attempt?.outcome==='uncertain')return '正确（不确定）';
  if(attempt?.correct&&attempt?.review?.qualified===false)return '正确（仅练习，未延长日程）';
  return attempt?.correct?'正确':'错误';
}
