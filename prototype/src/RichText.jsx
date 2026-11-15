import {useState} from 'react';
import Markdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import remarkMath from 'remark-math';
import rehypeRaw from 'rehype-raw';
import rehypeSanitize from 'rehype-sanitize';
import rehypeKatex from 'rehype-katex';
import 'katex/dist/katex.min.css';

export function prepareRichText(value = '') {
  const slots = [];
  const hold = text => { slots.push(text); return `\uE000${slots.length - 1}\uE001`; };
  const formulaText = formula => formula.replace(/(?<!\\)#/g, '\\#');
  const text = String(value)
    .replace(/\\\(([\s\S]*?)\\\)/g, (_, formula) => hold(`$${formulaText(formula)}$`))
    .replace(/\\\[([\s\S]*?)\\\]/g, (_, formula) => hold(`\n$$\n${formulaText(formula)}\n$$\n`))
    .replace(/<\/?(?:table|thead|tbody|tr|td|th|p|br|sup|sub|strong|em|b|i|code|pre)\s*\/?>/gi, hold)
    // Literal comparison signs/tuples are text, not HTML; MIPS $registers are not math.
    .replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/\$/g, '\\$');
  return text.replace(/\uE000(\d+)\uE001/g, (_, index) => slots[Number(index)])
    .replaceAll('/practice/collection-assets/relax-1000-27/', '/assets/relax-1000-27/');
}

function QuestionImage({src, alt}) {
  const [failed, setFailed] = useState(false);
  return failed ? <span className="missing-image" role="status">原题图片暂不可用：{src?.split('/').at(-1)}</span>
    : <img src={src} alt={alt || `原题图 ${src?.split('/').at(-1) || ''}`} loading="lazy" onError={() => setFailed(true)} />;
}

export function RichText({text, className = ''}) {
  return <div className={`rich-text ${className}`}><Markdown
    remarkPlugins={[remarkGfm, remarkMath]}
    rehypePlugins={[rehypeRaw, rehypeSanitize, [rehypeKatex, {strict: false, throwOnError: false, trust: false}]]}
    components={{
      img: QuestionImage,
      table: ({children}) => <div className="table-scroll"><table>{children}</table></div>,
      a: ({href, children}) => <a href={href} target="_blank" rel="noreferrer">{children}</a>,
    }}
  >{prepareRichText(text)}</Markdown></div>;
}
