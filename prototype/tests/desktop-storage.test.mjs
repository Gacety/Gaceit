import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';

test('desktop storage serializes large snapshots and reports a durable save', async()=>{
 const source=await readFile(new URL('../src/storage.js',import.meta.url),'utf8');
 const memory=new Map(), handlers={}, requests=[];
 globalThis.location={search:'?desktop=test-token',pathname:'/'};
 globalThis.history={replaceState(){}};
 globalThis.sessionStorage={getItem:k=>memory.get(k),setItem:(k,v)=>memory.set(k,v)};
 globalThis.window={addEventListener:(event,fn)=>handlers[event]=fn};
 let release;
 globalThis.fetch=async(path,options)=>{
  if(options.method==='PUT'){
   requests.push(options);
   if(requests.length===1)await new Promise(resolve=>release=resolve);
   return {ok:true};
  }
  return {ok:true,json:async()=>({'408-theme-v1':'light'})};
 };
 const code=source.replace("import {useSyncExternalStore} from 'react';",'const useSyncExternalStore=()=>{};');
 const module=await import('data:text/javascript;base64,'+Buffer.from(code).toString('base64'));
 await module.initializeStorage();
 module.storage.setItem('408-study-glass-v1',JSON.stringify({notes:'笔记'.repeat(50000)}));
 await new Promise(r=>setTimeout(r,240));
 assert.equal(requests.length,1);
 assert.equal(requests[0].keepalive,false,'large requests must not hit the keepalive 64KB browser cap');
 module.storage.setItem('408-theme-v1','dark');
 release();
 await new Promise(r=>setTimeout(r,50));
 assert.equal(requests.length,2);
 assert.equal(JSON.parse(requests[1].body)['408-theme-v1'],'dark');
 assert.equal(module.storage.getItem('408-theme-v1'),'dark');
});
