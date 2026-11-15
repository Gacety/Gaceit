import test from 'node:test';
import assert from 'node:assert/strict';
import {initialReview,applyReview,isReviewDue,reviewCounts,halfLife,recallProbability,studyDay,dayStart,nextReviewAt,RELEARN_DELAY,DAY} from '../src/review-scheduler.js';
import {recordReviewAttempt,undoReviewAttempt} from '../src/review-actions.js';

const NOW=Date.parse('2026-11-09T09:00:00+08:00');
const question={id:'test',history:{result:'错误'}};
const base=()=>({wrong:true,notes:'原笔记',strokes:[],attempts:[]});
const answer=(record,correct,now=NOW,quality='independent')=>{
  const response=applyReview(question,record,{correct,now,quality});
  return {...record,review:response.state};
};

test('undated imports are due now, with no invented last-review timestamp',()=>{
  const state=initialReview(question,base());
  assert.equal(state.lastReviewAt,null);
  assert.equal(state.right,0);
  assert.equal(state.wrong,0);
  assert.equal(isReviewDue(question,base(),NOW),true);
  assert.equal(isReviewDue(question,{...base(),wrong:false},NOW),false);
  assert.deepEqual(reviewCounts([question],{test:base()},NOW),{today:'2026-11-09',total:1,due:1,uninitialized:1,laterToday:0,scheduled:0,reviewedToday:0,blocked:0});
});

test('an undated source dont-know flag is retained without inventing a failed memory trial',()=>{
  const state=initialReview({...question,history:{result:'不会'}},base());
  assert.equal(state.unknown,0);
  assert.equal(state.wrong,0);
  assert.equal(state.priorUnknown,0);
  assert.equal(state.lastReviewAt,null);
});

test('HLR uses an exponential forgetting curve and changes half-life with successes/failures',()=>{
  const state=initialReview(question,base()),h=halfLife(state);
  assert.ok(Math.abs(recallProbability(state,h)-.5)<1e-10);
  assert.ok(halfLife({...state,right:3})>h);
  assert.ok(halfLife({...state,wrong:3})<h);
});

test('first independent correct answer schedules three days later and remains in the wrong book',()=>{
  const record=answer(base(),true);
  assert.equal(record.wrong,true);
  assert.equal(record.review.stage,'review');
  assert.equal(record.review.intervalDays,3);
  assert.equal(studyDay(Date.parse(record.review.dueAt)),'2026-11-12');
  assert.equal(isReviewDue(question,record,NOW),false);
  assert.equal(isReviewDue(question,record,NOW+3*DAY),true);
});

test('spaced successes extend interval using prior performance and train only timed recall',()=>{
  let record=answer(base(),true), previous=record.review.intervalDays;
  for(let i=0;i<6;i++){
    const next=Math.max(Date.parse(record.review.dueAt),Date.parse(record.review.lastReviewAt)+DAY);
    record=answer(record,true,next);
    assert.ok(record.review.intervalDays>=previous);
    previous=record.review.intervalDays;
  }
  assert.ok(record.review.intervalDays>1);
  assert.equal(record.review.trained,6);
  assert.ok(record.review.weights.every(Number.isFinite));
});

test('a lapse shortens recall strength and schedules ten-minute relearning',()=>{
  let record=answer(base(),true);
  record=answer(record,true,NOW+2*DAY);
  const previousHalfLife=halfLife(record.review),failedAt=NOW+5*DAY;
  record=answer(record,false,failedAt);
  assert.equal(record.review.stage,'relearning');
  assert.equal(record.review.lapses,1);
  assert.equal(record.review.streak,0);
  assert.equal(Date.parse(record.review.dueAt),failedAt+RELEARN_DELAY);
  assert.ok(halfLife(record.review)<previousHalfLife);
  assert.equal(isReviewDue(question,record,failedAt+RELEARN_DELAY-1),false);
  assert.equal(isReviewDue(question,record,failedAt+RELEARN_DELAY),true);
});

test('an immediate correction after seeing the answer keeps stored stability and the target date unchanged',()=>{
  const failed=answer(base(),false);
  const corrected=applyReview(question,failed,{correct:true,now:NOW+60000});
  assert.equal(corrected.metadata.qualified,false);
  assert.deepEqual(corrected.state,failed.review);
  const repeatedFailure=applyReview(question,failed,{correct:false,now:NOW+60000});
  assert.equal(repeatedFailure.metadata.qualified,false);
  assert.deepEqual(repeatedFailure.state,failed.review);
});

test('a correct relearning answer after the cooldown graduates to three-day consolidation',()=>{
  const failed=answer(base(),false);
  const corrected=answer(failed,true,NOW+RELEARN_DELAY);
  assert.equal(corrected.review.stage,'review');
  assert.equal(corrected.review.intervalDays,3);
  assert.equal(corrected.review.trained,0);
});

test('guessed correct answers do not become independent successes',()=>{
  const response=applyReview(question,base(),{correct:true,quality:'uncertain',now:NOW});
  assert.equal(response.state.right,0);
  assert.equal(response.state.wrong,0);
  assert.equal(response.state.uncertain,1);
  assert.equal(response.state.stage,'learning');
  assert.equal(Date.parse(response.state.dueAt),NOW+DAY);
});

test('the spacing ladder is stable while HLR still reflects prior failures',()=>{
  const initial=initialReview(question,base());
  const state={...initial,stage:'review',right:3,reps:3,intervalDays:1,lastOutcome:'right',lastReviewAt:new Date(NOW-2*DAY).toISOString(),dueAt:new Date(NOW-DAY).toISOString()};
  const strong=applyReview(question,{...base(),review:state},{correct:true,now:NOW});
  const fragile=applyReview(question,{...base(),review:{...state,wrong:5,reps:8}},{correct:true,now:NOW});
  assert.equal(strong.state.intervalDays,3);
  assert.equal(fragile.state.intervalDays,3);
  assert.ok(halfLife(strong.state)>halfLife(fragile.state));
});

test('unknown is kept distinct from incorrect answers while scheduling relearning',()=>{
  const response=applyReview(question,base(),{correct:null,unknown:true,now:NOW});
  assert.equal(response.state.unknown,1);
  assert.equal(response.state.wrong,0);
  assert.equal(response.metadata.outcome,'unknown');
});

test('early extra practice does not inflate the existing review schedule',()=>{
  const record=answer(base(),true);
  const response=applyReview(question,record,{correct:true,now:NOW+3*3600000});
  assert.equal(response.metadata.qualified,false);
  assert.deepEqual(response.state,record.review);
});

test('recent extra exposure is not misread as a later long-gap successful recall',()=>{
  const original=answer(base(),true,NOW-2*DAY);
  original.review={...original.review,dueAt:new Date(NOW+3600000).toISOString()};
  const early=recordReviewAttempt(question,original,{id:'early',choices:['A'],correct:true,now:NOW});
  assert.equal(early.memory.metadata.qualified,false);
  assert.equal(early.record.review.dueAt,original.review.dueAt);
  assert.equal(isReviewDue(question,early.record,NOW+3600000),false);
  const again=recordReviewAttempt(question,early.record,{id:'again',choices:['A'],correct:true,now:NOW+3600000});
  assert.equal(again.memory.metadata.qualified,false);
  assert.equal(again.record.review.right,original.review.right);
  assert.ok(Date.parse(nextReviewAt(question,again.record))>=NOW+13*3600000);
});

test('late-night review uses Taipei dates and does not create a one-hour spaced success',()=>{
  const late=Date.parse('2026-11-09T23:30:00+08:00');
  const record=answer(base(),true,late);
  assert.equal(studyDay(late),'2026-11-09');
  assert.equal(isReviewDue(question,record,late+3600000),false);
  assert.ok(Date.parse(record.review.dueAt)-late>=12*3600000);
  assert.equal(isReviewDue(question,record,Date.parse(record.review.dueAt)),true);
});

test('undo restores membership, attempts and schedule while preserving newer notes and drawings',()=>{
  const before=base();
  const changed=recordReviewAttempt(question,before,{id:'attempt-1',choices:['B'],correct:false,now:NOW});
  const records={test:{...changed.record,notes:'后来补的笔记',strokes:[{points:[[.1,.2]]}]}};
  const restored=undoReviewAttempt(records,'test','attempt-1',before);
  assert.deepEqual(restored.test.attempts,[]);
  assert.equal(restored.test.review,undefined);
  assert.equal(restored.test.wrong,true);
  assert.equal(restored.test.notes,'后来补的笔记');
  assert.equal(restored.test.strokes.length,1);
  assert.equal(undoReviewAttempt(records,'test','not-the-latest',before),records);
});

test('undo does not reverse a separate manual removal from the wrong book',()=>{
  const before=base();
  const response=recordReviewAttempt(question,before,{id:'manual-test',choices:['A'],correct:true,now:NOW});
  const restored=undoReviewAttempt({test:{...response.record,wrong:false}},'test','manual-test',before);
  assert.equal(restored.test.wrong,false);
  assert.equal(restored.test.attempts.length,0);
});

test('reloaded review state retains its exact due time, counts and weights',()=>{
  const record=answer(base(),true);
  const restored=JSON.parse(JSON.stringify(record));
  assert.deepEqual(restored,record);
  assert.equal(isReviewDue(question,restored,NOW+3*DAY),true);
});

test('a disputed answer key remains in the wrong book but does not generate unreliable automatic review grades',()=>{
  const disputed={...question,errata:'本站复核认为此题标答存疑，应选 C'};
  const response=recordReviewAttempt(disputed,base(),{id:'disputed',choices:['D'],correct:true,now:NOW});
  assert.equal(response.record.wrong,true);
  assert.equal(response.memory.metadata.qualified,false);
  assert.equal(response.memory.metadata.blocked,true);
  assert.equal(response.record.review.reps,0);
  assert.equal(isReviewDue(disputed,response.record,NOW),false);
  assert.equal(reviewCounts([disputed],{test:response.record},NOW).blocked,1);
});


test('qualified independent recalls follow 3,7,14,30 days after reload',()=>{
 let record=base(),now=NOW;
 for(const days of [3,7,14,30,60,120,240,365,365]) {
  const {state,metadata}=applyReview(question,record,{correct:true,now});
  assert.equal(metadata.qualified,true);
  assert.equal(state.intervalDays,days);
  assert.equal(Date.parse(state.dueAt)-now,days*DAY);
  record=JSON.parse(JSON.stringify({...record,review:state}));
  now=Date.parse(state.dueAt);
 }
});
