export const PROGRESS_KEY = '408-study-glass-v1';
export const blankRecord = () => ({notes: '', strokes: [], attempts: [], wrong: false});

// Seed source membership once; stored changes always win on subsequent loads.
// Source snapshots live with the question and are never appended as new attempts.
export function mergeImportedRecords(questions, existing = {}) {
  const merged = {...existing};
  for (const q of questions) {
    const saved = existing[q.id];
    merged[q.id] = {
      ...blankRecord(), wrong: q.history?.inWrongBook || false,
      ...(saved && typeof saved === 'object' ? saved : {}),
      notes: typeof saved?.notes === 'string' ? saved.notes : '',
      strokes: Array.isArray(saved?.strokes) ? saved.strokes : [],
      attempts: Array.isArray(saved?.attempts) ? saved.attempts : [],
    };
  }
  return merged;
}

export function selectedAnswers(question, choice) {
  return question.kind === 'multi_blank' ? (Array.isArray(choice) ? choice : []) : (typeof choice === 'string' ? [choice] : []);
}

export function gradeQuestion(question, choice) {
  const selected = selectedAnswers(question, choice);
  if (selected.length !== question.answerChoices.length || selected.some(v => !/^[ABCD]$/.test(v))) {
    return {complete: false, correct: null};
  }
  return {complete: true, correct: selected.every((v, i) => v === question.answerChoices[i])};
}

export function formatAnswers(question, selected) {
  const values = Array.isArray(selected) ? selected : [];
  return question.kind === 'multi_blank' ? values.map((value, i) => `${['①','②','③'][i]}${value || '—'}`).join('') : (values[0] || '未选');
}
