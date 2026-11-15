import test from 'node:test';
import assert from 'node:assert/strict';
import {shuffle,questionPool,drawQuestions,orderQuestions,sanitizeRound} from '../src/study-rounds.js';
import {applyReview} from '../src/review-scheduler.js';

const questions = [
  {id:'a',subject:'数据结构'}, {id:'b',subject:'数据结构'},
  {id:'c',subject:'操作系统'}, {id:'d',subject:'操作系统'},
];
const records = {a:{wrong:true,attempts:[]},b:{wrong:false,attempts:[{}]},c:{wrong:true,attempts:[{}]},d:{wrong:false,attempts:[]}};

test('shuffle preserves every member once, can change order, and does not mutate the original queue',()=>{
  const input=['a','b','c','d'];
  const output=shuffle(input,()=>0);
  assert.deepEqual(input,['a','b','c','d']);
  assert.notDeepEqual(output,input);
  assert.deepEqual([...output].sort(),input);
  assert.deepEqual(shuffle([],()=>0),[]);
  assert.deepEqual(shuffle(['a'],()=>0),['a']);
});

test('draw respects subject, live wrong membership and locally unpracticed scope',()=>{
  assert.deepEqual(questionPool(questions,records,{subject:'数据结构',range:'wrong'}).map(q=>q.id),['a']);
  assert.deepEqual(questionPool(questions,records,{subject:'全部学科',range:'unpracticed'}).map(q=>q.id),['a','d']);
  const selected=drawQuestions(questions,3,()=>0);
  assert.equal(new Set(selected).size,3);
  assert.ok(selected.every(id=>questions.some(q=>q.id===id)));
  assert.equal(drawQuestions(questions,4,()=>.999).length,4);
});

test('invalid draw sizes never silently exceed the pool or create duplicate questions',()=>{
  for(const count of ['',0,-1,1.5,5,Infinity,'oops']) assert.throws(()=>drawQuestions(questions,count),RangeError);
  assert.throws(()=>drawQuestions([],1),RangeError);
});

test('an existing round remains in order when wrong questions are mastered; remaining questions are not reshuffled',()=>{
  const saved=['c','a'];
  const before=orderQuestions(questions.filter(q=>records[q.id]?.wrong),saved);
  assert.deepEqual(before.map(q=>q.id),saved);
  const changed={...records,c:{...records.c,wrong:false}};
  assert.deepEqual(orderQuestions(questions.filter(q=>changed[q.id]?.wrong),saved).map(q=>q.id),['a']);
  assert.deepEqual(saved,['c','a']);
});

test('persisted rounds restore the exact sampled order, cursor and settings while rejecting stale IDs',()=>{
  const saved={mode:'random',currentId:'a',randomCurrentId:'a',randomIds:['c','a','c','missing'],wrongIds:['a','c'],settings:{subject:'数据结构',range:'wrong',count:'2'}};
  const restored=sanitizeRound(JSON.parse(JSON.stringify(saved)),questions);
  assert.deepEqual(restored.randomIds,['c','a']);
  assert.equal(restored.currentId,'a');
  assert.equal(restored.randomCurrentId,'a');
  assert.equal(restored.settings.range,'wrong');
  assert.equal(restored.settings.count,'2');
  assert.equal(sanitizeRound({mode:'bad',randomIds:'bad'},questions).mode,'practice');
});

test('the due-only draw excludes future reviews and paused questions, while keeping undated new wrongs',()=>{
  const now=Date.parse('2026-11-09T09:00:00+08:00');
  const review=applyReview(questions[0],{...records.a,notes:''},{correct:true,now}).state;
  const changed={...records,a:{...records.a,review}};
  assert.deepEqual(questionPool(questions,changed,{subject:'全部学科',range:'due',now}).map(q=>q.id),['c']);
  assert.deepEqual(questionPool(questions,changed,{subject:'全部学科',range:'wrong',now}).map(q=>q.id),['a','c']);
});
