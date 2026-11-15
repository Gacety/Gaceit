"""Read-only XLSX import; recover formatting only from exactly matching local source."""
from pathlib import Path
import hashlib
import html
import json
import re
from collections import Counter
import shutil
import openpyxl

ROOT = Path(__file__).resolve().parents[1]
SOURCE = Path('/Users/gacety/Documents/deepseek-harness/408/codebrick-relax/relax-1000-27.xlsx')
DETAILS = SOURCE.with_name('details.jsonl')
OUT = ROOT / 'data' / 'relax-1000-27'
OUT.mkdir(parents=True, exist_ok=True)

def plain(value):
    text = str(value or '')
    text = re.sub(r'<table>.*?</table>', '[表格题干，详见 MD 版]', text, flags=re.S)
    text = re.sub(r'<br\s*/?>', '\n', text)
    text = re.sub(r'</?p>', '\n', text)
    return html.unescape(re.sub(r'<[^>]+>', '', text)).strip()

def save(path, data):
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + '\n')

book = openpyxl.load_workbook(SOURCE, data_only=True)
sheet = book['题目']
headers = [cell.value for cell in sheet[1]]
rows = [dict(zip(headers, values)) for values in sheet.iter_rows(min_row=2, values_only=True)]
assert len(rows) == len({row['slug'] for row in rows}) == 1576
details = {q['slug']: q for q in (json.loads(line) for line in DETAILS.read_text().splitlines())}
questions = []
assets = set()
special = []
for row_number, row in enumerate(rows, start=2):
    slug = row['slug']
    rich = details.get(slug)
    assert rich, f'Missing rich source: {slug}'
    fields = {'题干': rich['stem'], '解析': rich['analysis'], '本站勘误': rich.get('errata')}
    fields.update({letter: (rich.get('options') or {}).get(letter, '') for letter in 'ABCD'})
    for column, value in fields.items():
        assert plain(value) == (row[column] or ''), f'Rich source mismatch: {slug}/{column}'
    assert rich['answer'] == row['答案'], f'Answer mismatch: {slug}'
    assert (rich.get('chosen') or '') == (row['你的作答'] or ''), f'History mismatch: {slug}'
    assert bool(rich.get('inWrongBook')) == (row['错题本'] == '是')
    source_result = '不会' if rich.get('dontKnow') else ('正确' if rich.get('correct') else '错误')
    assert source_result == row['结果']
    answer_choices = re.findall(r'[ABCD]', row['答案'])
    assert answer_choices and len(answer_choices) == (1 if rich['kind'] != 'multi_blank' else len(answer_choices))
    q = {
        'id': slug, 'subject': row['科目'], 'chapter': row['章节名'], 'chapterNumber': row['章'],
        'title': f"{row['章节名']} · 第 {row['原书题号']} 题", 'kind': rich['kind'], 'kindLabel': row['题型'],
        'text': rich['stem'], 'options': [(rich.get('options') or {}).get(k, '') for k in 'ABCD'],
        'answer': row['答案'], 'answerChoices': answer_choices, 'explanation': rich['analysis'],
        'errata': rich.get('errata') or '', 'ocrNotes': rich.get('ocrNotes') or [],
        'history': {'chosen': row['你的作答'] or '', 'result': row['结果'], 'inWrongBook': row['错题本'] == '是',
                    'durationMs': rich.get('durationMs'), 'wrongManual': rich.get('wrongManual'),
                    'timestamp': None},
        'siteRate': row['全站正确率'],
        'source': {'workbook': SOURCE.name, 'sheet': '题目', 'row': row_number, 'slug': slug,
                   'page': row['原书页'], 'number': row['原书题号'], 'collection': 'relax-1000-27',
                   'richTextSource': DETAILS.name, 'richTextVerified': True},
        'workbookRow': row,
    }
    # Preserve image paths and all raw source content, do not infer missing options.
    for value in [q['text'], q['explanation'], q['errata'], *q['options']]:
        assets.update(re.findall(r'!\[[^\]]*\]\((/practice/collection-assets/relax-1000-27/[^)]+)\)', value))
    if q['kind'] != 'choice' or any(not option for option in q['options']):
        special.append(slug)
    questions.append(q)

manifest = {
    'collection': 'relax-1000-27', 'title': 'relax 1000题（27版）', 'count': len(questions),
    'workbook': str(SOURCE), 'sha256': hashlib.sha256(SOURCE.read_bytes()).hexdigest(),
    'richSource': str(DETAILS), 'richSourceSha256': hashlib.sha256(DETAILS.read_bytes()).hexdigest(),
    'richSourceVerification': '1576 IDs and every normalized stem, A–D option, analysis, erratum, answer and history field matched the workbook.',
    'subjects': dict(Counter(q['subject'] for q in questions)),
    'kinds': dict(Counter(q['kindLabel'] for q in questions)),
    'results': dict(Counter(q['history']['result'] for q in questions)),
    'wrongBook': sum(q['history']['inWrongBook'] for q in questions),
    'errata': sum(bool(q['errata']) for q in questions),
    'ocrNotes': sum(bool(q['ocrNotes']) for q in questions),
    'sourceTables': sum(len(re.findall(r'<table>', s)) for q in questions for s in [q['text'], q['explanation'], *q['options']]),
    'assets': sorted(assets), 'specialQuestionIds': special,
    'historyCaveats': [
        'The workbook summary groups errors and dont-know results as 440; import retains 235 errors and 205 dont-know separately.',
        'Wrong-book membership is preserved independently (249), including correct answers manually retained and errors not in the wrong book.',
        'The workbook contains one snapshot per question, not a complete attempt history; no original attempt dates are supplied.',
        'Some historical multi-blank responses contain only one letter or a mismatched blank count; original choice and result remain unchanged.',
        'Errata are annotations, not instructions to automatically rewrite the answer key or original analysis.',
    ],
    'workbookStatistics': list(book['统计'].values),
}
save(OUT / 'questions.json', questions)
(ROOT / 'src' / 'relax-data.json').write_text(json.dumps([{key:value for key,value in q.items() if key != 'workbookRow'} for q in questions],ensure_ascii=False,separators=(',',':'))+'\n')
save(OUT / 'manifest.json', manifest)
save(OUT / 'workbook-rows.json', rows)
shutil.copy2(SOURCE, OUT / SOURCE.name)
print(json.dumps({k:v for k,v in manifest.items() if k in ['count','subjects','kinds','results','wrongBook','errata','ocrNotes','sourceTables']}, ensure_ascii=False, indent=2))
print(f'assets: {len(assets)}; source workbook unchanged; output: {OUT}')
