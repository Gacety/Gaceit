import { jsx, jsxs } from "react/jsx-runtime";
import { useState } from "react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import rehypeRaw from "rehype-raw";
import rehypeSanitize from "rehype-sanitize";
import rehypeKatex from "rehype-katex";
function prepareRichText(value = "") {
  const slots = [];
  const hold = (text2) => {
    slots.push(text2);
    return `\uE000${slots.length - 1}\uE001`;
  };
  const formulaText = (formula) => formula.replace(/(?<!\\)#/g, "\\#");
  const text = String(value).replace(/\\\(([\s\S]*?)\\\)/g, (_, formula) => hold(`$${formulaText(formula)}$`)).replace(/\\\[([\s\S]*?)\\\]/g, (_, formula) => hold(`
$$
${formulaText(formula)}
$$
`)).replace(/<\/?(?:table|thead|tbody|tr|td|th|p|br|sup|sub|strong|em|b|i|code|pre)\s*\/?>/gi, hold).replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/\$/g, "\\$");
  return text.replace(/\uE000(\d+)\uE001/g, (_, index) => slots[Number(index)]).replaceAll("/practice/collection-assets/relax-1000-27/", "/assets/relax-1000-27/");
}
function QuestionImage({ src, alt }) {
  const [failed, setFailed] = useState(false);
  return failed ? /* @__PURE__ */ jsxs("span", { className: "missing-image", role: "status", children: [
    "\u539F\u9898\u56FE\u7247\u6682\u4E0D\u53EF\u7528\uFF1A",
    src?.split("/").at(-1)
  ] }) : /* @__PURE__ */ jsx("img", { src, alt: alt || `\u539F\u9898\u56FE ${src?.split("/").at(-1) || ""}`, loading: "lazy", onError: () => setFailed(true) });
}
function RichText({ text, className = "" }) {
  return /* @__PURE__ */ jsx("div", { className: `rich-text ${className}`, children: /* @__PURE__ */ jsx(
    Markdown,
    {
      remarkPlugins: [remarkGfm, remarkMath],
      rehypePlugins: [rehypeRaw, rehypeSanitize, [rehypeKatex, { strict: false, throwOnError: false, trust: false }]],
      components: {
        img: QuestionImage,
        table: ({ children }) => /* @__PURE__ */ jsx("div", { className: "table-scroll", children: /* @__PURE__ */ jsx("table", { children }) }),
        a: ({ href, children }) => /* @__PURE__ */ jsx("a", { href, target: "_blank", rel: "noreferrer", children })
      },
      children: prepareRichText(text)
    }
  ) });
}
export {
  RichText,
  prepareRichText
};
