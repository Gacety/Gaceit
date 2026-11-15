// Build-time typography only. The application embeds SVG paths and runs no JS.
const fs=require('fs'),crypto=require('crypto');
const base=process.env.MATHJAX_DIR||'/tmp/408-native-math/node_modules/mathjax-full';
const {mathjax}=require(base+'/js/mathjax.js');
const {TeX}=require(base+'/js/input/tex.js');
const {SVG}=require(base+'/js/output/svg.js');
const {liteAdaptor}=require(base+'/js/adaptors/liteAdaptor.js');
const {RegisterHTMLHandler}=require(base+'/js/handlers/html.js');
const {AllPackages}=require(base+'/js/input/tex/AllPackages.js');
const adaptor=liteAdaptor();RegisterHTMLHandler(adaptor);
const document=mathjax.document('',{InputJax:new TeX({packages:AllPackages}),OutputJax:new SVG({fontCache:'none'})});
const bankPath=process.argv[2]||process.env.QUESTIONS_JSON||'prototype/src/relax-data.json';
const questions=JSON.parse(fs.readFileSync(bankPath,'utf8'));
const values=new Set();
for(const q of questions)for(const src of [q.text,...(q.options||[]),q.answer||'',q.rubric||'',q.explanation||'',q.errata||'',...(q.ocrNotes||[])])if(src)for(const match of src.matchAll(/\\\(([\s\S]*?)\\\)|\\\[([\s\S]*?)\\\]/g))values.add((match[1]??match[2]).trim());
const manifest={},errors=[];
for(const tex of values){
 try{
  const node=document.convert(tex.replace(/(^|[^\\])#/g,'$1\\#'),{display:false,em:100,ex:50,containerWidth:10000});
  const svg=adaptor.firstChild(node);
  if(adaptor.outerHTML(svg).includes('data-mjx-error'))throw Error('MathJax parse error');
  const vb=adaptor.getAttribute(svg,'viewBox').split(/\s+/).map(Number),id=crypto.createHash('sha256').update(tex).digest('hex').slice(0,20);
  adaptor.setAttribute(svg,'width',String(vb[2]));adaptor.setAttribute(svg,'height',String(vb[3]));
  fs.writeFileSync('native-assets/math/'+id+'.svg',adaptor.outerHTML(svg));
  manifest[tex]={file:id+'.svg',width:vb[2]/1000,height:vb[3]/1000,depth:Math.max(0,vb[1]+vb[3])/1000};
 }catch(e){errors.push({tex,error:String(e)});}
}
fs.writeFileSync('native-assets/math/manifest.json',JSON.stringify(manifest));
fs.writeFileSync('native-assets/math/report.json',JSON.stringify({total:values.size,rendered:Object.keys(manifest).length,errors},null,2));
console.log({total:values.size,rendered:Object.keys(manifest).length,errors:errors.length});
