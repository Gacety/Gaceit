import {useSyncExternalStore} from 'react';
const params=new URLSearchParams(location.search);
const token=params.get('desktop')||sessionStorage.getItem('408-desktop-token');
if(params.has('desktop')){sessionStorage.setItem('408-desktop-token',token);history.replaceState(null,'',location.pathname);}
let values={}, status='saved', revision=0, writing=false, timer;
const listeners=new Set();
function notify(next){status=next;for(const listener of listeners)listener();}
export function useStorageStatus(){return useSyncExternalStore(fn=>{listeners.add(fn);return()=>listeners.delete(fn);},()=>status);}
const headers={'Content-Type':'application/json','X-408-Token':token||''};
export async function initializeStorage(){
 if(!token)return;
 try{
  const response=await fetch('/api/state',{headers});
  if(!response.ok)throw new Error('读取学习记录失败');
  values=await response.json();
 }catch(err){
  document.getElementById('root').textContent='无法读取本地学习记录。请关闭窗口后重新打开；原文件不会被覆盖。';
  throw err;
 }
}
async function flush(){
 if(!token||writing)return;
 writing=true;
 const version=revision;
 try{
  const body=JSON.stringify(values);
  const response=await fetch('/api/state',{method:'PUT',headers,body,keepalive:new Blob([body]).size<60000});
  if(!response.ok)throw new Error('保存失败');
  notify(version===revision?'saved':'saving');
 }catch{notify('error');}
 finally{writing=false;if(version!==revision)void flush();}
}
export const storage={
 desktop:!!token,
 getItem(key){return token?values[key]??null:localStorage.getItem(key);},
 setItem(key,value){
  if(!token){localStorage.setItem(key,value);return;}
  if(values[key]===String(value))return;
  values[key]=String(value);revision++;notify('saving');clearTimeout(timer);timer=setTimeout(flush,180);
 },
};
window.addEventListener('beforeunload',event=>{if(token&&status!=='saved'){event.preventDefault();event.returnValue='';void flush();}});
window.addEventListener('pagehide',()=>{if(token&&status!=='saved')void flush();});
function snapshot(){return token?{...values}:Object.fromEntries(Object.keys(localStorage).filter(k=>k.startsWith('408-')).map(k=>[k,localStorage.getItem(k)]));}
export function exportBackup(){
 const blob=new Blob([JSON.stringify({format:'408-study-backup',version:1,data:snapshot()},null,2)],{type:'application/json'});
 const url=URL.createObjectURL(blob),a=document.createElement('a');a.href=url;a.download=`408-研习-${new Date().toISOString().slice(0,10)}.json`;a.click();setTimeout(()=>URL.revokeObjectURL(url),1000);
}
export async function importBackup(file){
 const backup=JSON.parse(await file.text());
 if(backup.format!=='408-study-backup'||backup.version!==1||!backup.data||Array.isArray(backup.data))throw new Error('这不是有效的 408 学习备份。');
 for(const [key,value]of Object.entries(backup.data)){
  if(!key.startsWith('408-')||typeof value!=='string')throw new Error('备份含有无效的记录。');
  if(!['408-theme-v1','408-study-location-v1'].includes(key)){try{JSON.parse(value);}catch{throw new Error('备份含有损坏的记录。');}}
 }
 if(!window.confirm('导入会覆盖备份中同名的学习记录。建议先导出当前备份。继续导入？'))return;
 if(token){
  while(writing)await new Promise(r=>setTimeout(r,30));
  clearTimeout(timer);
  const next={...values,...backup.data};
  const response=await fetch('/api/state',{method:'PUT',headers,body:JSON.stringify(next)});
  if(!response.ok)throw new Error('导入未成功，当前记录未修改。');
  values=next;
 }else{for(const [key,value]of Object.entries(backup.data))localStorage.setItem(key,value);}
}
