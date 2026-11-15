export const ROUND_KEY = '408-study-round-v1';
export const DEFAULT_DRAW = {subject: '全部学科', range: 'all', count: '20'};

export function shuffle(items, random = Math.random) {
  const result = [...items];
  for (let i = result.length - 1; i > 0; i--) {
    const j = Math.floor(random() * (i + 1));
    [result[i], result[j]] = [result[j], result[i]];
  }
  return result;
}

export function questionPool(questions, records, settings) {
  return questions.filter(q =>
    (settings.subject === '全部学科' || q.subject === settings.subject) &&
    (settings.range !== 'wrong' || records[q.id]?.wrong) &&
    (settings.range !== 'unpracticed' || !records[q.id]?.attempts?.length) &&
    (settings.range !== 'due' || isReviewDue(q,records[q.id],settings.now ?? Date.now()))
  );
}

export function drawQuestions(pool, requested, random = Math.random) {
  const count = Number(requested);
  if (!Number.isInteger(count) || count < 1 || count > pool.length) {
    throw new RangeError(`请填写 1～${pool.length} 之间的整数题量。`);
  }
  return shuffle(pool.map(q => q.id), random).slice(0, count);
}

export function orderQuestions(questions, ids) {
  const map = new Map(questions.map(q => [q.id, q]));
  return [...new Set(ids)].map(id => map.get(id)).filter(Boolean);
}

export function sanitizeRound(value, questions) {
  const valid = new Set(questions.map(q => q.id));
  const ids = list => Array.isArray(list) ? [...new Set(list)].filter(id => valid.has(id)) : [];
  const mode = ['practice', 'wrong', 'random', 'notes'].includes(value?.mode) ? value.mode : 'practice';
  const randomIds = ids(value?.randomIds);
  return {
    mode, wrongIds: ids(value?.wrongIds), randomIds,wrongScope:value?.wrongScope==='all'?'all':'due',
    currentId: valid.has(value?.currentId) ? value.currentId : null,
    randomCurrentId: randomIds.includes(value?.randomCurrentId) ? value.randomCurrentId : randomIds[0],
    settings: {
      subject: ['全部学科', ...new Set(questions.map(q => q.subject))].includes(value?.settings?.subject) ? value.settings.subject : DEFAULT_DRAW.subject,
      range: ['all', 'wrong', 'unpracticed','due'].includes(value?.settings?.range) ? value.settings.range : DEFAULT_DRAW.range,
      count: typeof value?.settings?.count === 'string' ? value.settings.count : DEFAULT_DRAW.count,
    },
  };
}
import {isReviewDue} from './review-scheduler.js';
