import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import {questions} from '../src/questions.js';
import {mergeImportedRecords,gradeQuestion} from '../src/progress.js';

test('all source questions, errata and independent history categories survive import',()=>{
  assert.equal(questions.length,1576);
  assert.equal(new Set(questions.map(q=>q.id)).size,1576);
  assert.equal(questions.filter(q=>q.errata).length,30);
  assert.equal(questions.filter(q=>q.history.inWrongBook).length,249);
  assert.equal(questions.filter(q=>q.history.result==='正确').length,1136);
  assert.equal(questions.filter(q=>q.history.result==='错误').length,235);
  assert.equal(questions.filter(q=>q.history.result==='不会').length,205);
  assert.deepEqual(questions.reduce((count,q)=>({...count,[q.subject]:(count[q.subject]||0)+1}),{}),{'数据结构':411,'计算机组成原理':373,'操作系统':417,'计算机网络':375});
});

test('import is idempotent, never counts a snapshot as a new attempt, and preserves local mastery/notes',()=>{
  const seeded=mergeImportedRecords(questions);
  assert.equal(Object.values(seeded).filter(r=>r.wrong).length,249);
  assert.equal(Object.values(seeded).reduce((n,r)=>n+r.attempts.length,0),0);
  const q=questions.find(q=>q.history.inWrongBook);
  seeded[q.id]={...seeded[q.id],wrong:false,notes:'保留笔记',attempts:[{choices:['C'],correct:true}]};
  seeded['old-demo-question']={notes:'旧原型笔记'};
  const merged=mergeImportedRecords(questions,seeded);
  assert.deepEqual(merged,mergeImportedRecords(questions,merged));
  assert.equal(merged[q.id].wrong,false);
  assert.equal(merged[q.id].notes,'保留笔记');
  assert.equal(merged[q.id].attempts.length,1);
  assert.equal(merged['old-demo-question'].notes,'旧原型笔记');
});

test('wrong-book flags are authoritative even when source outcomes differ',()=>{
  const seeded=mergeImportedRecords(questions);
  const correctInBook=questions.filter(q=>q.history.result==='正确'&&q.history.inWrongBook);
  assert.equal(correctInBook.length,3);
  assert.ok(correctInBook.every(q=>seeded[q.id].wrong));
  const wrongOutside=questions.filter(q=>q.history.result==='错误'&&!q.history.inWrongBook);
  assert.equal(wrongOutside.length,1);
  assert.equal(seeded[wrongOutside[0].id].wrong,false);
});

test('multi-blank questions require all blanks and retain incomplete historical responses without regrading',()=>{
  const q=questions.find(q=>q.id==='relax-ds-07-046');
  assert.deepEqual(gradeQuestion(q,['C','B']),{complete:true,correct:true});
  assert.deepEqual(gradeQuestion(q,['B','C']),{complete:true,correct:false});
  assert.equal(gradeQuestion(q,['C']).complete,false);
  assert.equal(gradeQuestion(q,['C','']).complete,false);
  assert.equal(q.history.chosen,'C');
  assert.equal(q.history.result,'正确');
  assert.equal(questions.filter(q=>q.kind==='multi_blank').length,5);
});

test('rich source restores tables, tuple options and image-only choices, without silently applying errata',()=>{
  const tuple=questions.find(q=>q.id==='relax-cn-03-088');
  assert.equal(tuple.options[0],'<H1, 1>、<H3, 2>');
  assert.equal(tuple.options[3],'<H3, 2>');
  assert.equal(questions.filter(q=>q.kind==='figure_options').length,3);
  assert.ok(questions.some(q=>q.text.includes('<table>')||q.explanation.includes('<table>')));
  const disputed=questions.find(q=>q.id==='relax-co-05-066');
  assert.equal(disputed.answer,'D');
  assert.ok(disputed.errata.includes('应选 C'));
  assert.equal(gradeQuestion(disputed,'D').correct,true);
});

test('all imported source rows remain traceable and the original workbook is archived',()=>{
  for(const [i,q] of questions.entries()){
    assert.equal(q.source.row,i+2);
    assert.equal(q.source.slug,q.id);
    assert.equal(q.source.sheet,'题目');
    assert.equal(q.history.timestamp,null);
  }
  assert.ok(fs.existsSync(new URL('../data/relax-1000-27/relax-1000-27.xlsx',import.meta.url)));
});

test('re-importing preserves a stored review plan as well as local notes and membership',()=>{
  const q=questions.find(q=>q.history.inWrongBook);
  const review={algorithm:'hlr-local-v1',dueAt:'2026-11-12T00:00:00+08:00',weights:[1,1.7,-1.2]};
  const saved={[q.id]:{notes:'保留',wrong:true,attempts:[],strokes:[],review}};
  const restored=mergeImportedRecords(questions,JSON.parse(JSON.stringify(saved)));
  assert.deepEqual(restored[q.id].review,review);
});
