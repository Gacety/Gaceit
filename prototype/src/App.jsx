import {useState,useRef,useEffect} from 'react';
import {ArrowLeft,ArrowRight,ArrowCounterClockwise,CheckCircle,WarningCircle,PencilSimple,Eraser,Trash,BookOpen,CaretDown,X,NotePencil,ArrowUUpLeft,List,Shuffle,Sun,Moon,DownloadSimple,UploadSimple} from '@phosphor-icons/react';
import {storage, useStorageStatus, exportBackup, importBackup} from './storage.js';
import {questions,subjects} from './questions';
import {RichText} from './RichText';
import {PROGRESS_KEY,blankRecord,mergeImportedRecords,gradeQuestion,selectedAnswers,formatAnswers} from './progress';
import {ROUND_KEY,shuffle,questionPool,drawQuestions,orderQuestions,sanitizeRound} from './study-rounds';
import {isReviewDue,reviewCounts,reviewState,reviewLabel,formatReviewTime,nextReviewAt} from './review-scheduler.js';
import {recordReviewAttempt,undoReviewAttempt,attemptOutcomeLabel} from './review-actions.js';
const KEY=PROGRESS_KEY;
const blank=blankRecord;
function load(){try{const value=JSON.parse(storage.getItem(KEY)||'{}');return mergeImportedRecords(questions,value.version===1&&value.records?value.records:{});}catch{return mergeImportedRecords(questions);}}
function loadRound(){try{return sanitizeRound(JSON.parse(storage.getItem(ROUND_KEY)||'{}'),questions);}catch{return sanitizeRound({},questions);}}
const letters=['A','B','C','D'];
function Scratchpad({strokes,onChange}){
 const ref=useRef(null),live=useRef(null),[tool,setTool]=useState('pen'),[color,setColor]=useState('#4e6b3a'),[history,setHistory]=useState([]);
 function paint(){const el=ref.current;if(!el)return;const r=el.getBoundingClientRect(),d=window.devicePixelRatio||1;el.width=Math.round(r.width*d);el.height=Math.round(r.height*d);const c=el.getContext('2d');c.setTransform(d,0,0,d,0,0);for(const s of [...strokes,...(live.current?[live.current]:[])]){c.globalCompositeOperation=s.tool==='eraser'?'destination-out':'source-over';c.strokeStyle=s.color;c.fillStyle=s.color;c.lineWidth=s.tool==='eraser'?24:2.6;c.lineCap='round';c.lineJoin='round';c.beginPath();s.points.forEach(([x,y],i)=>i?c.lineTo(x*r.width,y*r.height):c.moveTo(x*r.width,y*r.height));if(s.points.length===1){const [x,y]=s.points[0];c.arc(x*r.width,y*r.height,s.tool==='eraser'?12:1.3,0,Math.PI*2);c.fill();}else c.stroke();}}
 useEffect(()=>{paint();const o=new ResizeObserver(paint);o.observe(ref.current);return()=>o.disconnect();},[strokes]);
 const point=e=>{const r=ref.current.getBoundingClientRect();return[(e.clientX-r.left)/r.width,(e.clientY-r.top)/r.height];};
 const finish=()=>{if(live.current){setHistory([...history,strokes]);onChange([...strokes,live.current]);live.current=null;}};
 return <div className="scratch-wrap"><div className="paper-grid"/><canvas ref={ref} aria-label="草稿纸演示画布，支持鼠标与触控笔" onPointerDown={e=>{if(e.pointerType==='mouse'&&e.button!==0)return;e.currentTarget.setPointerCapture(e.pointerId);live.current={tool,color,points:[point(e)]};paint();}} onPointerMove={e=>{if(live.current){live.current.points.push(point(e));paint();}}} onPointerUp={finish} onPointerCancel={finish}/>{!strokes.length&&<div className="paper-hint"><PencilSimple size={28} weight="light"/><span>把思路写在这里</span><small>草稿纸演示 · 支持鼠标与触控笔</small></div>}<div className="drawing-toolbar glass" role="toolbar" aria-label="草稿工具"><button className={'tool '+(tool==='pen'?'active':'')} aria-label="画笔" title="画笔" aria-pressed={tool==='pen'} onClick={()=>setTool('pen')}><PencilSimple/></button><button className={'tool '+(tool==='eraser'?'active':'')} aria-label="橡皮" title="橡皮" aria-pressed={tool==='eraser'} onClick={()=>setTool('eraser')}><Eraser/></button><span className="toolbar-divider"/><button className="tool" aria-label="撤销笔画" title="撤销（包括清空）" disabled={!history.length} onClick={()=>{onChange(history.at(-1));setHistory(history.slice(0,-1));}}><ArrowUUpLeft/></button><button className="tool" aria-label="清空草稿" title="清空（可撤销）" disabled={!strokes.length} onClick={()=>{setHistory([...history,strokes]);onChange([]);}}><Trash/></button><span className="toolbar-divider"/>{['#4e6b3a','#d28b65'].map(c=><button key={c} aria-label={c==='#4e6b3a'?'墨绿色画笔':'杏色画笔'} aria-pressed={color===c} className={'ink-swatch '+(c===color?'chosen':'')} style={{backgroundColor:c}} onClick={()=>{setColor(c);setTool('pen');}}/>)}</div></div>;
}
export function App() {
  const [records,setRecords] = useState(load);
  const diskStatus=useStorageStatus();
  const backupInput=useRef(null);
  const [theme,setTheme]=useState(()=>storage.getItem('408-theme-v1')||(matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light'));
  useEffect(()=>{document.documentElement.dataset.theme=theme;storage.setItem('408-theme-v1',theme);},[theme]);
  const [reviewNow,setReviewNow] = useState(()=>Date.now());
  const [reviewScope,setReviewScope] = useState(()=>loadRound().wrongScope);
  const [mode,setMode] = useState(() => loadRound().mode);
  const [wrongOrder,setWrongOrder] = useState(() => {const ids=loadRound().wrongIds;return ids.length?ids:shuffle(questions.filter(q=>records[q.id]?.wrong).map(q=>q.id));});
  const [randomOrder,setRandomOrder] = useState(() => loadRound().randomIds);
  const [drawSettings,setDrawSettings] = useState(() => loadRound().settings);
  const [drawError,setDrawError] = useState('');
  const randomCursor = useRef(loadRound().randomCurrentId);
  const [subject,setSubject] = useState('全部学科');
  const [chapter,setChapter] = useState('全部章节');
  const [id,setId] = useState(() => {try {const round=loadRound();const saved=round.currentId||storage.getItem('408-study-location-v1');return questions.some(q=>q.id===saved)?saved:questions[0].id;} catch {return questions[0].id;}});
  const [choice,setChoice] = useState(null), [resultState,setResult] = useState(null);
  const [recallQuality,setRecallQuality] = useState('independent');
  const [message,setMessage] = useState(''), [saved,setSaved] = useState('saved');
  const [filterOpen,setFilterOpen] = useState(false);
  const [activeTool,setActiveTool] = useState(null);
  const scratchOpen=activeTool==='scratch', notesOpen=activeTool==='notes';
  const [directoryOpen,setDirectoryOpen] = useState(false), [search,setSearch] = useState('');
  const [directoryFilter,setDirectoryFilter] = useState('all'), [directoryLimit,setDirectoryLimit] = useState(60);
  const noteRef=useRef(null);
  const q=questions.find(question=>question.id===id)||questions[0], record=records[q.id]||blank();
  const result=resultState?.questionId===id?resultState:null;
  const reviewStats=reviewCounts(questions,records,reviewNow);
  const dueMode=mode==='wrong'&&reviewScope==='due';
  const wrongPool=questions.filter(question=>records[question.id]?.wrong && (reviewScope==='all'||isReviewDue(question,records[question.id],reviewNow)));
  const wrongPoolWithFeedback=result&&record.wrong&&!wrongPool.some(question=>question.id===id)?[...wrongPool,q]:wrongPool;
  const ordered = mode==='wrong'
    ? orderQuestions(wrongPoolWithFeedback,wrongOrder)
    : mode==='random' ? orderQuestions(questions,randomOrder) : questions;
  const visible=ordered.filter(question=>(subject==='全部学科'||question.subject===subject)&&(chapter==='全部章节'||question.chapter===chapter));
  const drawPool=questionPool(questions,records,{...drawSettings,now:reviewNow});
  const index=visible.findIndex(question=>question.id===id);
  const wrongCount=questions.filter(question=>records[question.id]?.wrong).length;
  const nextScheduled=questions.filter(question=>records[question.id]?.wrong&&!question.errata.includes('标答存疑')).map(question=>nextReviewAt(question,records[question.id])).filter(Boolean).map(value=>Date.parse(value)).filter(value=>value>reviewNow).sort((a,b)=>a-b)[0];
  const currentReadyAt=nextReviewAt(q,record);
  const wrongPoolKey=wrongPool.map(question=>question.id).join('|');
  const noteQuestions=questions.filter(question=>records[question.id]?.notes?.trim());
  const chapters=[...new Set(ordered.filter(question=>subject==='全部学科'||question.subject===subject).map(question=>question.chapter))];
  const matchedQuestions=visible.filter(question=>{
    if(directoryFilter==='errata'&&!question.errata)return false;
    if(directoryFilter==='wrong'&&!records[question.id]?.wrong)return false;
    if(directoryFilter==='incorrect'&&question.history.result!=='错误')return false;
    if(directoryFilter==='unknown'&&question.history.result!=='不会')return false;
    const term=search.trim().toLowerCase();
    return !term||`${question.id} ${question.title} ${question.text} p${question.source.page} ${question.source.number}`.toLowerCase().includes(term);
  });
  const update=patch=>setRecords(previous=>({...previous,[q.id]:{...(previous[q.id]||blank()),...patch}}));
  useEffect(()=>{try{storage.setItem(KEY,JSON.stringify({version:1,records}));setSaved('saved');}catch{setSaved('error');}},[records]);
  useEffect(()=>{const refresh=()=>setReviewNow(Date.now());const timer=setInterval(refresh,30000);document.addEventListener('visibilitychange',refresh);return()=>{clearInterval(timer);document.removeEventListener('visibilitychange',refresh);};},[]);
  useEffect(()=>{if(mode!=='wrong')return;const missing=wrongPool.map(question=>question.id).filter(questionId=>!wrongOrder.includes(questionId));if(missing.length)setWrongOrder(previous=>[...previous,...shuffle(missing.filter(questionId=>!previous.includes(questionId)))]);},[mode,wrongPoolKey]);
  useEffect(()=>{
    if(mode==='random'&&randomOrder.includes(id))randomCursor.current=id;
    try{storage.setItem(ROUND_KEY,JSON.stringify({mode,currentId:id,wrongIds:wrongOrder,wrongScope:reviewScope,randomIds:randomOrder,randomCurrentId:randomCursor.current,settings:drawSettings}));}catch{setSaved('error');}
  },[mode,id,wrongOrder,randomOrder,drawSettings,reviewScope]);
  useEffect(()=>{setChoice(null);setResult(null);setRecallQuality('independent');setMessage('');try{storage.setItem('408-study-location-v1',id);}catch{}},[id,mode]);
  useEffect(()=>{if(mode!=='notes'&&visible.length&&index<0)setId(visible[0].id);},[subject,chapter,mode,records,wrongOrder,randomOrder,wrongPoolKey]);
  useEffect(()=>{setDirectoryLimit(60);},[search,directoryFilter,subject,chapter,mode]);
  useEffect(()=>{if(message){const timeout=setTimeout(()=>setMessage(''),3500);return()=>clearTimeout(timeout);}},[message]);
  function resetView(){setSubject('全部学科');setChapter('全部章节');setFilterOpen(false);setDirectoryOpen(false);setDirectoryFilter('all');setSearch('');setChoice(null);setResult(null);setRecallQuality('independent');setDrawError('');}
  function shuffleWrongRound(scope=reviewScope){const now=Date.now();setReviewNow(now);const ids=shuffle(questions.filter(question=>records[question.id]?.wrong&&(scope==='all'||isReviewDue(question,records[question.id],now))).map(question=>question.id));setWrongOrder(ids);resetView();if(ids.length)setId(ids[0]);}
  function switchReviewScope(scope){setReviewScope(scope);shuffleWrongRound(scope);}
  function openMode(next){
    setMode(next);resetView();
    if(next==='wrong'){setReviewScope('due');shuffleWrongRound('due');}
    if(next==='random'&&randomOrder.length)setId(randomOrder.includes(randomCursor.current)?randomCursor.current:randomOrder[0]);
  }
  function startRandomRound(){
    if(!drawPool.length){setDrawError('这个范围暂时没有可抽取的题目，请换一个范围。');return;}
    try{const ids=drawQuestions(drawPool,drawSettings.count);setRandomOrder(ids);resetView();setId(ids[0]);randomCursor.current=ids[0];}
    catch(error){setDrawError(error.message);}
  }
  function setWrongMembership(wrong){
    if(!wrong&&mode==='wrong'){const next=visible[index+1]||visible[index-1];if(next)setId(next.id);}
    update({wrong});
  }
  function navigate(direction){if(visible[index+direction])setId(visible[index+direction].id);}
  function choose(letter,blankIndex=0){
    if(q.kind==='multi_blank')setChoice(previous=>{const answers=Array.isArray(previous)?[...previous]:Array(q.answerChoices.length).fill('');answers[blankIndex]=letter;return answers;});
    else setChoice(letter);
    setMessage('');
  }
  function submit(unknown=false){
    if(result)return;
    const grade=unknown?{complete:true,correct:null}:gradeQuestion(q,choice);
    if(!grade.complete){setMessage(q.kind==='multi_blank'?'请为每一空选择答案，再提交核对。':'先选一个答案，再提交核对。');return;}
    const now=Date.now(),attemptId=crypto.randomUUID(),choices=unknown?[]:selectedAnswers(q,choice),previous=record.attempts.at(-1);
    if(unknown)setChoice(null);
    const response=recordReviewAttempt(q,record,{id:attemptId,choices,correct:grade.correct,quality:recallQuality,unknown,now});
    setRecords(current=>({...current,[q.id]:response.record}));
    setReviewNow(now);
    setResult({questionId:q.id,correct:grade.correct,unknown,previous,memory:response.memory,beforeRecord:record,attemptId});
  }
  function undoSubmission(){
    if(!result)return;
    setRecords(current=>undoReviewAttempt(current,q.id,result.attemptId,result.beforeRecord));setResult(null);setChoice(null);setReviewNow(Date.now());setMessage('本次作答与复习安排已撤销，笔记和草稿保留。');
  }
  function continueAfterAnswer(){
    if(index<visible.length-1)navigate(1);
    else if(dueMode)shuffleWrongRound('due');
    else if(mode==='random')startRandomRound();
    else{setResult(null);setChoice(null);}
  }
  function openQuestion(question){setId(question.id);setDirectoryOpen(false);}
  const saveLabel=saved==='error'||diskStatus==='error'?'保存失败，请导出备份':diskStatus==='saving'?'正在保存…':storage.desktop?'已保存到本地文件':'已保存到浏览器';
  const historyIncomplete=q.kind==='multi_blank'&&(q.history.chosen.match(/[ABCD]/g)||[]).length!==q.answerChoices.length;
  const optionKeys=['A','B','C','D'];
  function answerButtons(blankIndex=0){
    return <div className={'answers'+(q.options.some(option=>option.length>55)?' long-options':'')} role="radiogroup" aria-label={q.kind==='multi_blank'?`第 ${blankIndex+1} 空答案`:'选择答案'} onKeyDown={event=>{
      if(result||!['ArrowLeft','ArrowRight','ArrowUp','ArrowDown'].includes(event.key))return;
      event.preventDefault();const next=(Number(event.target.closest('[data-index]')?.dataset.index||0)+(['ArrowRight','ArrowDown'].includes(event.key)?1:3))%4;
      choose(optionKeys[next],blankIndex);event.currentTarget.querySelectorAll('[role=radio]')[next].focus();
    }}>{q.options.map((option,optionIndex)=>{
      const letter=optionKeys[optionIndex], selected=q.kind==='multi_blank'?choice?.[blankIndex]===letter:choice===letter;
      const expected=q.answerChoices[blankIndex]===letter;
      return <button key={letter} className={`answer ${selected?'selected ':''}${result&&expected?'correct ':''}${result&&selected&&!expected?'incorrect':''}`} role="radio" data-index={optionIndex} tabIndex={selected||(!selectedAnswers(q,choice)[blankIndex]&&optionIndex===0)?0:-1} aria-checked={selected} disabled={!!result} onClick={()=>choose(letter,blankIndex)}>
        <span className="answer-letter">{letter}</span><RichText text={option||`见题图中的 ${letter} 选项`}/>{result&&expected&&<CheckCircle size={20}/>}</button>;
    })}</div>;
  }
  return <div className={`app-shell ${((scratchOpen||notesOpen)&&mode!=='notes')?'with-tools':''}`}>
    <header className="topbar">

      <a className="brand" href="#" aria-label="408 研习首页" onClick={event=>{event.preventDefault();openMode('practice');}}>408<span>研习</span></a>
      <span className="brand-divider"/>
      <nav aria-label="学习导航">{[['practice','章节练习'],['wrong','错题复习'],['random','随机抽题'],['notes','我的笔记']].map(([key,label])=><button key={key} className={`nav-link ${mode===key?'selected':''}`} aria-current={mode===key?'page':undefined} onClick={()=>openMode(key)}>{label}{key==='wrong'&&<span className="count">{reviewStats.due}</span>}</button>)}</nav>
      <div className="desktop-tools">
        <button className="tool" aria-label={theme==='light'?'切换 Dracula 主题':'切换浅色主题'} title={theme==='light'?'Dracula 主题':'浅色纸张'} onClick={()=>setTheme(theme==='light'?'dark':'light')}>{theme==='light'?<Moon size={20}/>:<Sun size={20}/>}</button>
        <button className="tool" aria-label="导出学习备份" title="导出学习备份" onClick={exportBackup}><DownloadSimple size={20}/></button>
        <button className="tool" aria-label="导入学习备份" title="导入学习备份" onClick={()=>backupInput.current.click()}><UploadSimple size={20}/></button>
        <input ref={backupInput} type="file" accept="application/json,.json" hidden onChange={async e=>{const file=e.target.files[0];if(file){try{await importBackup(file);location.reload();}catch(err){setMessage(err.message);}}e.target.value='';}}/>
      </div>
      <div className={`save-status ${saved==='error'?'save-error':''}`} role="status">{saved==='error'?<WarningCircle size={19}/>:<CheckCircle size={19}/>}<span>{saveLabel}</span></div>
    </header>
    {mode==='wrong'&&<section className="review-overview workbook" aria-labelledby="review-title">
      <div className="review-overview-heading"><div><h1 id="review-title">今日错题复习</h1><p>{reviewStats.today} · 台北时间 · 到期题随机排列</p></div><span className="review-completed">今天已处理 {reviewStats.reviewedToday} 题</span></div>
      <div className="review-scope-row"><div className="review-tabs" role="group" aria-label="复习范围"><button aria-pressed={reviewScope==='due'} className={reviewScope==='due'?'active':''} onClick={()=>switchReviewScope('due')}>今日到期 <strong>{reviewStats.due}</strong></button><button aria-pressed={reviewScope==='all'} className={reviewScope==='all'?'active':''} onClick={()=>switchReviewScope('all')}>全部错题 <strong>{wrongCount}</strong></button></div><span className="review-scheduled">已安排 {reviewStats.scheduled} 题{reviewStats.laterToday>0&&` · 今天稍后 ${reviewStats.laterToday} 题`}{reviewStats.blocked>0&&` · 待核对 ${reviewStats.blocked} 题`}</span></div>
      {reviewStats.uninitialized>0&&<p className="review-onboarding">原表未提供复习日期；{reviewStats.uninitialized} 道未建档错题先列入今日，作答后自动安排下次时间。</p>}
      {reviewScope==='all'&&<p className="review-onboarding">可浏览和提前练习全部错题；提前答对会保留原复习时间，答错则重新安排。</p>}
    </section>}
    {mode==='random'&&<section className="random-picker workbook" aria-labelledby="random-title">
      <div className="random-heading"><div><h1 id="random-title"><Shuffle size={25}/>随机抽题</h1><p>每轮不重复；重新抽取会开始新一轮。</p></div><span className="pool-count">可抽 {drawPool.length} 题</span></div>
      <form className="random-controls" onSubmit={event=>{event.preventDefault();startRandomRound();}}>
        <label>学科<select aria-label="随机抽题学科" value={drawSettings.subject} onChange={event=>{setDrawSettings({...drawSettings,subject:event.target.value});setDrawError('');}}>{['全部学科',...subjects].map(value=><option key={value}>{value}</option>)}</select></label>
        <label>题目范围<select aria-label="随机抽题范围" value={drawSettings.range} onChange={event=>{setDrawSettings({...drawSettings,range:event.target.value});setDrawError('');}}><option value="all">全部题库</option><option value="due">今日到期错题</option><option value="wrong">全部错题本</option><option value="unpracticed">本机未练过</option></select></label>
        <label>题量<input aria-label="随机抽题题量" inputMode="numeric" type="number" min="1" max={drawPool.length||1} step="1" value={drawSettings.count} onChange={event=>{setDrawSettings({...drawSettings,count:event.target.value});setDrawError('');}}/></label>
        <button className="primary" type="submit" disabled={!drawPool.length}><Shuffle size={18}/>{randomOrder.length?'重新抽取':'抽取并开始'}</button>
      </form>
      {drawSettings.range==='unpracticed'&&<p className="random-help">“本机未练过”只看此刷题器的新作答记录，不更改导入的原表历史。</p>}
      {drawError&&<p className="draw-error" role="alert">{drawError}</p>}
    </section>}
    {mode==='notes'?<main className="workbook library">
      <div className="library-head"><div><h1>我的笔记</h1><p>留住每一次想通的瞬间。</p></div><span>{noteQuestions.length} 篇随题笔记</span></div>
      {noteQuestions.length?<div className="note-list">{noteQuestions.map(question=><button className="note-entry" key={question.id} onClick={()=>{setId(question.id);openMode('practice');setActiveTool('notes');setTimeout(()=>noteRef.current?.focus(),50);}}><div><span>{question.subject} / {question.chapter} · 原书 p{question.source.page}</span><h2>{question.title}</h2><p>{records[question.id].notes}</p></div><ArrowRight size={24}/></button>)}</div>:<div className="empty-state"><NotePencil size={48} weight="light"/><h2>给思路留个位置</h2><p>打开本题笔记，记下理解与疑问，<br/>笔记会与题目一起保存在这里。</p><button className="primary" onClick={()=>openMode('practice')}>开始练习 <ArrowRight size={18}/></button></div>}
    </main>:!visible.length&&dueMode?<main className="workbook library review-empty">
      <div className="empty-state"><CheckCircle size={48} weight="light"/><h2>{reviewStats.due?'当前筛选没有到期题':'当前到期题已处理完'}</h2><p>{reviewStats.due?'切换到全部学科，继续今天的复习。':nextScheduled?`下次可复习：${formatReviewTime(new Date(nextScheduled).toISOString(),reviewNow)}。`:'练习中答错或标记不会的题，会按计划出现在这里。'}<br/>{reviewStats.laterToday>0?`今天稍后还有 ${reviewStats.laterToday} 题，时间到了会自动加入。`:'已安排的题不会因为答对一次就从错题本消失。'}</p><button className="primary" onClick={()=>reviewStats.due?shuffleWrongRound('due'):switchReviewScope('all')}>{reviewStats.due?'显示全部到期题':'查看全部错题'}<ArrowRight size={18}/></button></div>
    </main>:!visible.length?<main className="workbook library">
      <div className="library-head"><h1>{mode==='wrong'?'错题重做':mode==='random'?'随机抽题':'章节练习'}</h1><span>relax 1000题（27版）</span></div>
      <div className="empty-state"><BookOpen size={48} weight="light"/><h2>{mode==='random'?(randomOrder.length?'当前筛选没有本轮题目':'抽一组题，开始这一轮练习'):mode==='wrong'?(wrongCount?'当前筛选没有待重做的题':'暂时没有待重做的错题'):'当前筛选没有题目'}</h2><p>{mode==='random'?'选择上方的学科、范围和题量，再点击“抽取并开始”。':'原表的历史记录会保留，新的作答单独记录。'}</p>{mode==='random'&&randomOrder.length>0&&<button className="primary" onClick={resetView}>返回本轮全部题目 <ArrowRight size={18}/></button>}{mode!=='random'&&<button className="primary" onClick={()=>openMode(mode==='wrong'&&wrongCount?'wrong':'practice')}>{mode==='wrong'&&wrongCount?'查看全部错题':'去章节练习'}<ArrowRight size={18}/></button>}</div>
    </main>:<main className={`workbook imported-workbook study-layout ${scratchOpen||notesOpen?'tools-open':''}`}>
      <div className="question-column">
      {(mode==='wrong'||mode==='random')&&<div className="round-toolbar"><span><Shuffle size={16}/>{mode==='wrong'?(dueMode?'今日到期随机复习':'全部错题随机练习'):'本轮随机抽题'} · {mode==='wrong'?(dueMode?reviewStats.due:wrongCount):randomOrder.length} 题<span className="round-note">{saved==='error'?'顺序未保存':'顺序已保存'}</span></span>{mode==='wrong'&&<button onClick={()=>shuffleWrongRound()}><ArrowCounterClockwise size={15}/>重新随机排序</button>}</div>}
      <div className="question-meta">
        <div className="breadcrumb"><div className="subject-picker"><button className="subject-button" aria-label="选择练习学科" aria-expanded={filterOpen} onClick={()=>setFilterOpen(!filterOpen)}>{q.subject}<CaretDown size={14}/></button>{filterOpen&&<div className="subject-menu glass">{['全部学科',...subjects].map(value=><button key={value} className={subject===value?'active':''} onClick={()=>{setSubject(value);setChapter('全部章节');setFilterOpen(false);}}>{value}{subject===value&&<CheckCircle size={16}/>}</button>)}</div>}</div><span className="slash">/</span><span>{q.chapter}</span>{mode==='wrong'&&<span className="redo-label"><ArrowCounterClockwise size={14}/>错题重做</span>}</div>
        <div className="meta-actions"><button className="directory-button" aria-expanded={directoryOpen} onClick={()=>setDirectoryOpen(!directoryOpen)}><List size={18}/>题目目录</button><span className="progress">{dueMode?`待复习 ${reviewStats.due}`:<>{Math.max(index,0)+1}<span> / {visible.length}</span></>}</span></div>
      </div>
      {directoryOpen&&<section className="question-directory" aria-label="题目目录">
        <div className="directory-controls"><label>章节<select aria-label="筛选章节" value={chapter} onChange={event=>setChapter(event.target.value)}><option>全部章节</option>{chapters.map(value=><option key={value}>{value}</option>)}</select></label><label>记录<select aria-label="筛选记录" value={directoryFilter} onChange={event=>setDirectoryFilter(event.target.value)}><option value="all">全部题目</option><option value="errata">含勘误 · 30 题</option><option value="wrong">当前错题本</option><option value="incorrect">原表答错 · 235 题</option><option value="unknown">原表不会 · 205 题</option></select></label><label className="directory-search">查找<input aria-label="搜索题目" value={search} onChange={event=>setSearch(event.target.value)} placeholder="输入题干、题号、slug 或 p页码"/></label><button className="tool" aria-label="关闭题目目录" onClick={()=>setDirectoryOpen(false)}><X size={20}/></button></div>
        <div className="directory-summary">找到 {matchedQuestions.length} 题 · 原表错题本 249 题 · 勘误 30 条</div>
        <div className="directory-list">{matchedQuestions.slice(0,directoryLimit).map(question=><button key={question.id} className={question.id===id?'current':''} onClick={()=>openQuestion(question)}><span><strong>{question.subject} · {question.title}</strong><small>{question.id} · p{question.source.page} · {question.text.replace(/!\[[^\]]*\]\([^)]*\)/g,'').slice(0,65)}</small></span><span className="directory-badges">{question.errata&&<small>勘误</small>}{records[question.id]?.wrong&&<small>错题本</small>}<ArrowRight size={17}/></span></button>)}</div>
        {!matchedQuestions.length&&<p className="no-matches">没有匹配的题目，试试清空关键词或切换筛选。</p>}
        {matchedQuestions.length>directoryLimit&&<button className="load-more" onClick={()=>setDirectoryLimit(directoryLimit+60)}>再显示 60 题</button>}
      </section>}
      <section className="question-section" aria-labelledby="question-title">
        <div className="question-heading"><div className="title-row"><span className="demo-tag source-tag">{q.kindLabel}</span><h1 id="question-title">{q.title}</h1></div><div className="study-tool-buttons" role="group" aria-label="学习工具">
        <button aria-label={scratchOpen?'收起草稿纸':'打开草稿纸'} aria-expanded={scratchOpen} aria-controls="scratch-panel" onClick={()=>setActiveTool(tool=>tool==='scratch'?null:'scratch')}><PencilSimple size={18}/>草稿纸</button>
        <button aria-label={notesOpen?'收起本题笔记':'打开本题笔记'} aria-expanded={notesOpen} aria-controls="notes-panel" onClick={()=>setActiveTool(tool=>tool==='notes'?null:'notes')}><NotePencil size={18}/>本题笔记{record.notes.trim()&&<span className="notes-indicator">有笔记</span>}</button>
      </div></div>
        <div className="source-line"><span>relax 1000题（27版） · 原书 p{q.source.page} 第 {q.source.number} 题</span><span>{q.id}</span><button onClick={()=>{setWrongMembership(!record.wrong);setMessage(record.wrong?'已移出当前错题本，原表记录保留。':'已加入当前错题本。');}}>{record.wrong?'移出错题本':'加入错题本'}</button></div>
        {record.wrong&&<div className="memory-status"><span>{reviewLabel(q,record,reviewNow)}</span><span>{q.errata.includes('标答存疑')?'暂不参与自动排期':currentReadyAt?`下次：${formatReviewTime(currentReadyAt,reviewNow)}`:'本次作答后建立复习计划'}</span></div>}
        {(q.errata||q.ocrNotes.length>0)&&<div className="source-notices">{q.errata&&<details className="errata"><summary><WarningCircle size={16}/>本站勘误 · 本题作答前建议查看</summary><RichText text={q.errata}/></details>}{q.ocrNotes.length>0&&<details className="ocr-notice"><summary>录入备注 · {q.ocrNotes.length} 条</summary>{q.ocrNotes.map((note,i)=><p key={i}>{note}</p>)}</details>}</div>}
        <RichText className="question-text" text={q.text}/>
        {q.kind==='multi_blank'?<div className="multi-answers">{q.answerChoices.map((_,blankIndex)=><fieldset key={blankIndex}><legend>第 {blankIndex+1} 空</legend>{answerButtons(blankIndex)}</fieldset>)}</div>:answerButtons()}
        {!result&&<div className="recall-controls"><div className="recall-quality" role="group" aria-label="本次回忆把握"><span>本次回忆</span><button aria-pressed={recallQuality==='independent'} className={recallQuality==='independent'?'active':''} onClick={()=>setRecallQuality('independent')}>独立作答</button><button aria-pressed={recallQuality==='uncertain'} className={recallQuality==='uncertain'?'active':''} onClick={()=>setRecallQuality('uncertain')}>不确定 / 用过提示</button></div><button className="dont-know-button" onClick={()=>submit(true)}>不会，先看解析</button></div>}
        {result&&<div className={`answer-review ${result.correct?'is-correct':'is-wrong'}`} role="status"><div className="review-heading">{result.correct?<CheckCircle size={21}/>:<WarningCircle size={21}/>}<strong>{result.unknown?`本次标记不会。表内答案：${q.answer}`:result.correct?'与表内答案一致。':`表内答案：${q.answer}，已加入错题。`}</strong><button className="text-button" onClick={()=>{setChoice(null);setResult(null);}}>再做一次</button></div>
          {q.errata&&<div className="review-caveat">本题附有勘误，请对照上方说明；未自动改写表内答案或原解析。</div>}
          {result.memory&&<section className="schedule-feedback" aria-label="本次复习安排"><div className="schedule-feedback-heading"><strong>{result.memory.metadata.blocked?'暂不自动排期':result.memory.metadata.qualified?'下次复习':'练习记录已保存'}</strong><span>{(result.memory.metadata.readyAt||result.memory.state.dueAt)?formatReviewTime(result.memory.metadata.readyAt||result.memory.state.dueAt,reviewNow):'待核对'}</span></div><p>{result.memory.metadata.reason}</p><details className="memory-details"><summary>查看复习依据</summary><p>独立答对 {result.memory.state.right} 次；答错 {result.memory.state.wrong} 次；不会 {result.memory.state.unknown} 次；不确定 {result.memory.state.uncertain} 次。当前连续独立答出 {result.memory.state.streak} 次。</p><p>本地 HLR 半衰期模型：{result.memory.state.trained?`已用 ${result.memory.state.trained} 条真实间隔记录调整本题模型。`:'当前为冷启动估计，尚无足够的真实间隔记录。'}原表无日期记录仅作初始难度参考，不伪造复习日期。</p></details></section>}
          <RichText className="explanation" text={q.explanation}/>
          <details className="history-details" open><summary>原表作答记录</summary><dl><div><dt>你的作答</dt><dd>{q.history.chosen||'未选'}</dd></div><div><dt>原表结果</dt><dd>{q.history.result}</dd></div><div><dt>原表错题本</dt><dd>{q.history.inWrongBook?'是':'否'}</dd></div><div><dt>本次作答</dt><dd>{formatAnswers(q,selectedAnswers(q,choice))}</dd></div></dl>{historyIncomplete&&<p>原表多空题作答缺空或空数不同，以上结果按原表保留，没有重新判分。</p>}<small>来源：{q.source.workbook} · 题目表第 {q.source.row} 行。原表未提供作答日期；历史快照不计作本机新作答。</small></details>
          {result.previous&&<p className="local-attempt">上次本机作答：{formatAnswers(q,result.previous.choices)} · {attemptOutcomeLabel(result.previous)}</p>}
          <button className="undo-review-button" onClick={undoSubmission}>撤销本次作答与安排</button>
        </div>}
      </section>
      <footer className="question-footer"><button className="page-button" onClick={()=>navigate(-1)} disabled={index<=0}><ArrowLeft size={23}/><span>上一题</span></button><button className="page-button" onClick={()=>navigate(1)} disabled={index>=visible.length-1}><span>下一题</span><ArrowRight size={23}/></button><div className="submit-area">{!result?<button className="primary submit" onClick={()=>submit()}>{mode==='wrong'?'提交并核对':'提交答案'}</button>:<button className="primary submit" onClick={continueAfterAnswer}>{index<visible.length-1?'继续下一题':dueMode?(reviewStats.due?'继续今日复习':'完成今日复习'):mode==='random'?'再抽一组':'重新作答'}<ArrowRight size={18}/></button>}</div></footer>
      </div>
      <aside className="writing-sidebar" aria-label="草稿与笔记" hidden={!scratchOpen&&!notesOpen}>
      <section id="scratch-panel" className="tool-panel scratch-section" hidden={!scratchOpen} aria-labelledby="scratch-title">
        <div className="pane-heading"><h2 id="scratch-title">草稿纸</h2><span className="demo-caption">轻量演示</span><button className="panel-close tool" aria-label="关闭草稿纸" onClick={()=>setActiveTool(null)}><X size={18}/></button></div>
        <Scratchpad key={q.id} strokes={record.strokes} onChange={strokes=>update({strokes})}/>
      </section>
      <section id="notes-panel" className="tool-panel notes-section" hidden={!notesOpen} aria-labelledby="notes-title">
        <div className="pane-heading"><h2 id="notes-title">本题笔记</h2><span className="note-save"><CheckCircle size={17}/><span>{saved==='error'||diskStatus==='error'?'未保存':diskStatus==='saving'?'保存中':'已保存'}</span></span><button className="panel-close tool" aria-label="关闭本题笔记" onClick={()=>setActiveTool(null)}><X size={18}/></button></div>
        <textarea ref={noteRef} className="note-input" aria-label="本题笔记" placeholder={'记下这题的思路…\n\n也可以留一个待解决的疑问。'} value={record.notes} onChange={event=>update({notes:event.target.value})}/>
        <div className="note-footer">随题保存，下次回来接着想。<span>{record.notes.length} 字</span></div>
      </section>
      </aside>
    </main>}
    <div className="app-footnote"><span>relax 1000题（27版） · {questions.length} 题 · 30 条勘误</span><span>本机新作答与原表记录分别保留</span></div>
    {message&&<div className="toast glass" role="alert">{message}<button aria-label="关闭提示" onClick={()=>setMessage('')}><X size={16}/></button></div>}
  </div>;
}
