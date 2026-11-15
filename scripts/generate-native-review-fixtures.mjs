import {writeFileSync} from 'node:fs';
import {applyReview,isReviewDue,nextReviewAt} from '../prototype/src/review-scheduler.js';
const output=[];
for(const result of ['正确','错误','不会']){
 const q={history:{result},errata:''};let record={wrong:true,attempts:[]};let time=Date.parse('2026-11-09T08:00:00Z');
 for(const [outcome,gap]of [['wrong',0],['right',60000],['unknown',600000],['right',600000],['right',3600000],['right',86400000],['right',4*86400000],['uncertain',86400000],['right',600000],['right',3*86400000]]){
  time+=gap;const options={correct:['right','uncertain'].includes(outcome),unknown:outcome==='unknown',quality:outcome==='uncertain'?'uncertain':'independent',now:time};
  const response=applyReview(q,record,options);output.push({question:q,record:structuredClone(record),outcome,now:new Date(time).toISOString(),state:response.state,qualified:response.metadata.qualified,due:isReviewDue(q,record,time),next:nextReviewAt(q,record)});
  record={...record,review:response.state,attempts:[...record.attempts,{correct:options.correct,time:new Date(time).toISOString()}]};
 }
}
// Explicit due-time ladder parity, separate from the short/early retry cases.
{
 const question={history:{result:''},errata:''};let record={wrong:true,attempts:[]};let now=Date.parse('2026-11-11T15:30:00Z');
 for(let i=0;i<9;i++){
  const response=applyReview(question,record,{correct:true,now});
  output.push({question,record:structuredClone(record),outcome:'right',now:new Date(now).toISOString(),state:response.state,qualified:response.metadata.qualified,due:isReviewDue(question,record,now),next:nextReviewAt(question,record)});
  record={...record,review:response.state,attempts:[...record.attempts,{correct:true,time:new Date(now).toISOString()}]};now=Date.parse(response.state.dueAt);
 }
}
writeFileSync('internal/nativeapp/review-fixtures.json' ,JSON.stringify(output,null,2));
