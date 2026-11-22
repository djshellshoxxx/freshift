/* FreshSift client-side HTTP(S) crawler. CORS applies unless optional extension is installed. */
(function(root){
'use strict';
const MAX_BYTES=8388608;
const lines=x=>String(x||'').split(/[\n\r,]+/).map(s=>s.trim()).filter(Boolean);
const exts=x=>lines(x).map(s=>s.toLowerCase().startsWith('.')?s.toLowerCase():'.'+s.toLowerCase());
const host=u=>new URL(u).hostname.toLowerCase();
const local=h=>h==='localhost'||h.endsWith('.localhost')||h.endsWith('.local')||h==='::1'||/^127\./.test(h)||/^10\./.test(h)||/^192\.168\./.test(h)||/^169\.254\./.test(h)||/^172\.(1[6-9]|2\d|3[01])\./.test(h)||/^0\./.test(h)||/^fc|^fd|^fe80:/i.test(h);
const pathMatch=(url,ends)=>ends.some(ext=>new URL(url).pathname.toLowerCase().endsWith(ext));
function urlOf(input,base){
 let u;try{u=new URL(input,base)}catch{u=new URL('https://'+input)}
 if(!['http:','https:'].includes(u.protocol))throw Error('Browser mode cannot fetch '+u.protocol+' URLs (FTP requires Go)');
 if(u.username||u.password)throw Error('Credential-bearing URL rejected');
 u.hash='';return u.href;
}
function pattern(text,regex,caseSensitive){
 if(text.length>250)throw Error('Browser regex/string limited to 250 characters');
 if(regex&&(/\\[1-9]/.test(text)||/\([^)]*[+*][^)]*\)[+*{]/.test(text)))throw Error('Complex/backtracking regex requires Go RE2 engine');
 const escaped=Array.from(text).map(ch=>'.*+?^$()[]{}|\\'.includes(ch)?'\\'+ch:ch).join('');
 return new RegExp(regex?text:escaped,caseSensitive?'g':'gi');
}
function visible(text){return String(text).replace(/<head\b[\s\S]*?<\/head>/gi,' ').replace(/<(script|style|template|svg|noscript)\b[^>]*>[\s\S]*?<\/\1>/gi,' ').replace(/<!--[\s\S]*?-->/g,' ').replace(/<[^>]+>/g,' ').replace(/&nbsp;/gi,' ').replace(/&amp;/gi,'&').replace(/&lt;/gi,'<').replace(/&gt;/gi,'>')}
function references(body,url,type){
 const out=[],add=(value,asset)=>{
  if(!value||/^(?:#|mailto:|javascript:|data:|blob:)/i.test(value))return;
  try{out.push({url:urlOf(value.replaceAll('&amp;','&'),url),asset})}catch{}
 };
 if(/html|xml/i.test(type)||/<(?:a|script|link|img)\b/i.test(body.slice(0,2000))){
  const tags=/<(a|area|link|script|img|source|video|audio|iframe)\b[^>]*>/gi;let m;
  while((m=tags.exec(body))){
   const at=/\b(?:href|src|poster)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))/gi;let a;
   while((a=at.exec(m[0])))add(a[1]||a[2]||a[3],!['a','area','iframe'].includes(m[1].toLowerCase()));
  }
 }
 if(/css/i.test(type)||/\.css(?:\?|$)/i.test(url)){
  const urls=/(?:url\s*\(\s*|@import\s+)(?:"([^"]+)"|'([^']+)'|([^)\s;]+))/gi;let m;
  while((m=urls.exec(body)))add(m[1]||m[2]||m[3],true);
 }
 if(/javascript|ecmascript/i.test(type)||/\.(?:m?js)(?:\?|$)/i.test(url)){
  const js=/(?:import\s+(?:[^;\n]*?\s+from\s+)?|export\s+[^;\n]*?\s+from\s+)['"]([^'"]+)['"]/g;let m;
  while((m=js.exec(body)))add(m[1],true);
 }
 return out;
}
async function nativeFetch(url,opt={}){
 if(root.FreshSiftExtension?.ready)return root.FreshSiftExtension.fetch(url,opt);
 const ctl=new AbortController(),abort=()=>ctl.abort();
 opt.signal?.addEventListener('abort',abort,{once:true});
 if(opt.signal?.aborted)ctl.abort();
 const timer=setTimeout(()=>ctl.abort(),Math.max(2000,Number(opt.timeoutSeconds||12)*1000));
 try{
  const r=await fetch(url,{method:opt.method||'GET',headers:opt.headers||{},credentials:'omit',mode:'cors',redirect:'follow',cache:'no-store',signal:ctl.signal});
  if(r.type==='opaque')throw Error('CORS: opaque response');
  if((opt.method||'GET')==='HEAD')return {url:r.url,status:r.status,type:r.headers.get('content-type')||'',body:''};
  if(Number(r.headers.get('content-length'))>MAX_BYTES)throw Error('Body exceeds 8 MiB');
  let bytes;
  if(r.body?.getReader){
   const reader=r.body.getReader(),chunks=[];let size=0;
   while(true){const p=await reader.read();if(p.done)break;size+=p.value.byteLength;
    if(size>MAX_BYTES){await reader.cancel();throw Error('Body exceeds 8 MiB')}
    chunks.push(p.value);
   }
   bytes=new Uint8Array(size);let pos=0;for(const c of chunks){bytes.set(c,pos);pos+=c.length}
  }else{const b=await r.arrayBuffer();if(b.byteLength>MAX_BYTES)throw Error('Body exceeds 8 MiB');bytes=new Uint8Array(b)}
  const type=r.headers.get('content-type')||'',encoder=/(html|xml|json|javascript|css|text)/i.test(type)?'utf-8':'latin1';
  return {url:r.url,status:r.status,type,body:new TextDecoder(encoder).decode(bytes),size:bytes.byteLength};
 }finally{clearTimeout(timer);opt.signal?.removeEventListener('abort',abort)}
}
function robotsRules(text){
 let active=false,rules=[];
 for(const line of String(text).split(/\r?\n/)){
  const m=/^\s*(user-agent|allow|disallow)\s*:\s*([^#]*)/i.exec(line);if(!m)continue;
  const field=m[1].toLowerCase(),value=m[2].trim();
  if(field==='user-agent')active=value==='*';
  else if(active&&value)rules.push({allow:field==='allow',prefix:value.split('*')[0]});
 }return rules;
}
class BrowserEngine {
 constructor({fetchResource=nativeFetch,onUpdate=()=>{}}={}){this.transport=fetchResource;this.onUpdate=onUpdate;this.reset()}
 reset(){
  this.running=false;this.paused=false;this.cancelled=false;this.queue=[];this.seen=new Set();this.hostCounts=new Map();this.robots=new Map();
  this.matches=[];this.urls=new Map();this.logs=[];this.signal=null;
  this.stats={status:'idle',startedAt:'',finishedAt:'',elapsedSeconds:0,pagesScanned:0,pagesPerMinute:0,bytesScanned:0,textBytes:0,MBScanned:0,textMB:0,matches:0,matchedPages:0,filesFound:0,discovered:0,queued:0,skipped:0,truncatedPages:0,errors:0,activeWorkers:0,lastUrl:'',lastError:''};
 }
 snapshot(){
  const stats={...this.stats};stats.queued=this.queue.length;
  if(stats.startedAt){stats.elapsedSeconds=((stats.finishedAt?Date.parse(stats.finishedAt):Date.now())-Date.parse(stats.startedAt))/1000;stats.pagesPerMinute=stats.elapsedSeconds>0?stats.pagesScanned*60/stats.elapsedSeconds:0}
  stats.MBScanned=stats.bytesScanned/1048576;stats.textMB=stats.textBytes/1048576;
  return {stats,urls:[...this.urls.values()],matches:this.matches.slice(-500),logs:[...this.logs]};
 }
 allResults(){return [...this.matches]}
 log(msg){this.logs.push(new Date().toISOString().slice(11,19)+' '+msg);if(this.logs.length>200)this.logs.shift();this.onUpdate(this.snapshot())}
 error(label,e){this.stats.errors++;this.stats.lastError=String(e?.message||e);this.log(label+': '+this.stats.lastError)}
 allow(url){
  const c=this.cfg,u=new URL(url),h=u.hostname.toLowerCase();
  if(local(h)&&!c.allowPrivate)return false;
  if(pathMatch(url,exts(c.excludeFileTypes)))return false;
  const tlds=exts(c.allowedTLDs);
  if(tlds.length&&(!h.includes('.')||!tlds.some(t=>h.endsWith(t))))return false;
  const allowed=lines(c.allowDomains),denied=lines(c.excludeDomains);
  if(allowed.length&&!allowed.some(x=>h===x||h.endsWith('.'+x)))return false;
  if(denied.some(x=>h===x||h.endsWith('.'+x)))return false;
  if(c.urlContains&&!url.includes(c.urlContains))return false;
  if(c.urlExcludes&&url.includes(c.urlExcludes))return false;
  if(u.pathname.split('/').filter(Boolean).length>Number(c.maxPathSegments||50))return false;
  return true;
 }
 enqueue(raw,source='manual',depth=0,asset=false,observedAt=''){
  let url;try{url=urlOf(raw)}catch(e){this.log('Skipped '+String(raw).slice(0,80)+': '+e.message);return}
  if(this.seen.has(url)||!this.allow(url)||this.seen.size>=this.cfg.maxPages*10)return;
  this.seen.add(url);this.queue.push({url,source,depth,asset,observedAt});this.stats.discovered++;
 }
 async fetch(url,method='GET',headers={}){
  return this.transport(url,{method,headers,timeoutSeconds:this.cfg.timeoutSeconds,signal:this.signal?.signal,allowPrivate:!!this.cfg.allowPrivate});
 }
 async mayCrawl(url){
  if(!this.cfg.respectRobots)return true;
  const u=new URL(url),origin=u.origin;
  if(!this.robots.has(origin))this.robots.set(origin,(async()=>{
   try{const r=await this.fetch(origin+'/robots.txt');if(r.status===404)return [];
    if(r.status>=400)throw Error('HTTP '+r.status);return robotsRules(r.body);
   }catch(e){this.log('robots.txt blocked/failed for '+origin+'; skipping targets until disabled: '+e.message);return null}
  })());
  const rules=await this.robots.get(origin);if(rules===null)return false;
  let length=-1,allow=true;
  for(const r of rules){if(u.pathname.startsWith(r.prefix)&&r.prefix.length>=length){length=r.prefix.length;allow=r.allow}}
  return allow;
 }
 addMatch(c,term,matched,before,after,kind,status){
  this.stats.matches++;if(kind==='file')this.stats.filesFound++;
  const obj={id:this.stats.matches,url:c.url,host:host(c.url),source:c.source,pattern:term,matched,before,after,context:before+matched+after,area:kind==='file'?'File URL':'Source',kind,status,foundAt:new Date().toISOString(),observedAt:c.observedAt};
  if(this.matches.length<50000)this.matches.push(obj);
  if(!this.urls.has(c.url)){this.urls.set(c.url,{url:c.url,source:c.source,count:0,lastMatch:obj.foundAt});this.stats.matchedPages++}
  this.urls.get(c.url).count++;this.urls.get(c.url).lastMatch=obj.foundAt;
 }
 fileMatch(c){return pathMatch(c.url,exts(this.cfg.fileTypes))}
 async verify(c){
  let r;try{r=await this.fetch(c.url,'HEAD');if(r.status===405||r.status===501)r=await this.fetch(c.url,'GET',{'Range':'bytes=0-0'})}
  catch(e){this.error(c.url,e);return}
  if(r.status>=200&&r.status<400){const suffix=exts(this.cfg.fileTypes).find(x=>new URL(c.url).pathname.toLowerCase().endsWith(x))||'';
   this.addMatch(c,'',suffix,'','','file',r.status)}
 }
 async handle(c){
  const u=new URL(c.url),h=u.hostname,count=this.hostCounts.get(h)||0;
  if(count>=this.cfg.maxPagesPerHost){this.stats.skipped++;return}
  this.hostCounts.set(h,count+1);
  if(!await this.mayCrawl(c.url)){this.stats.skipped++;return}
  if(this.cfg.searchMode==='files'&&this.fileMatch(c)){await this.verify(c);return}
  let r;try{r=await this.fetch(c.url)}catch(e){this.error(c.url,e);return}
  if(r.status<200||r.status>=400){this.error(c.url,Error('HTTP '+r.status));return}
  if(!this.allow(r.url||c.url)){this.stats.skipped++;return}
  if(this.cfg.status2xxOnly&&r.status>299)return;
  const body=String(r.body||''),type=r.type||'',isHTML=/html/i.test(type)||/<html\b/i.test(body.slice(0,512));
  this.stats.pagesScanned++;this.stats.bytesScanned+=Number(r.size)||body.length;this.stats.textBytes+=body.length;this.stats.lastUrl=c.url;
  if(this.cfg.searchMode!=='files'&&(this.cfg.searchMode==='source'||isHTML||/text\/plain/i.test(type))){
   const hay=this.cfg.searchMode==='visible'&&isHTML?visible(body):body;let count=0;
   for(const p of this.patterns){p.re.lastIndex=0;let m;
    while((m=p.re.exec(hay))&&count++<200){
     const a=m.index,b=a+m[0].length;this.addMatch(c,p.text,m[0],hay.slice(Math.max(0,a-100),a),hay.slice(b,b+100),'text',r.status);
     if(!m[0])p.re.lastIndex++;
    }
   }
  }
  if(this.cfg.searchMode==='visible')return;
  for(const l of references(body,r.url||c.url,type)){
   const asset=l.asset||pathMatch(l.url,exts(this.cfg.fileTypes))||/\.(?:css|js|mjs|json|db|png|jpe?g|gif|svg|txt|pdf|xml)$/i.test(new URL(l.url).pathname);
   if(!asset&&c.depth>=this.cfg.depth)continue;
   if(!asset&&!this.cfg.followExternal&&host(l.url)!==h)continue;
   this.enqueue(l.url,'linked',c.depth+(asset?0:1),asset,c.observedAt);
  }
 }
 async discover(){
  for(const p of this.cfg.providers||[]){
   try{
    if(p==='hn'||p==='urlscan'){
     const endpoint=p==='hn'?'https://hn.algolia.com/api/v1/search_by_date?tags=story&hitsPerPage=100':'https://urlscan.io/api/v1/search/?q='+encodeURIComponent(this.cfg.sourceQuery||'date:>now-24h')+'&size=100';
     const r=await this.fetch(endpoint);if(r.status!==200)throw Error('HTTP '+r.status);
     const json=JSON.parse(r.body);let n=0;
     for(const item of (p==='hn'?json.hits:json.results)||[]){
      const url=p==='hn'?item.url:item.page?.url;if(url&&n++<(this.cfg.discoveryLimit||100))this.enqueue(url,p,0,false,p==='hn'?item.created_at:item.task?.time);
     }
    }else if(p==='sitemap'||p==='rss'){
     for(const raw of lines(this.cfg.seeds).slice(0,20)){
      let endpoint;try{endpoint=new URL(p==='sitemap'?'/sitemap.xml':'/feed.xml',urlOf(raw)).href}catch{continue}
      const r=await this.fetch(endpoint);if(r.status!==200)continue;
      const rx=p==='sitemap'?/<loc>\s*([^<]+)\s*<\/loc>/gi:/<link>\s*(https?:\/\/[^<]+)\s*<\/link>/gi;let m,n=0;
      while((m=rx.exec(r.body))&&n++<(this.cfg.discoveryLimit||100))this.enqueue(m[1],p);
     }
    }else this.log(p+' provider unavailable in pure browser mode; use Go for authenticated provider queries');
   }catch(e){this.error('Discovery '+p,e)}
  }
 }
 async worker(){
  this.stats.activeWorkers++;
  try{while(!this.cancelled){
   while(this.paused&&!this.cancelled)await new Promise(r=>setTimeout(r,75));
   if(this.cancelled)break;
   const c=this.queue.shift();if(!c)break;
   try{await this.handle(c)}catch(e){this.error(c.url,e)}
   this.onUpdate(this.snapshot());
  }}finally{this.stats.activeWorkers--}
 }
 async scanLocal(files){
  for(const file of files){
   if(this.cancelled)break;
   const url='local-file://input/'+encodeURIComponent(file.name);
   const c={url,source:'local-file',observedAt:''};
   if(Number(file.size)>8388608){this.error(file.name,Error('Local file exceeds 8 MiB'));continue}
   if(exts(this.cfg.excludeFileTypes).some(x=>file.name.toLowerCase().endsWith(x)))continue;
   if(this.cfg.searchMode==='files'){
    const ext=exts(this.cfg.fileTypes).find(x=>file.name.toLowerCase().endsWith(x));
    if(ext)this.addMatch(c,'',ext,'','','file',200);
    continue;
   }
   try{
    const bytes=new Uint8Array(await file.arrayBuffer());
    if(bytes.byteLength>8388608)throw Error('Local file exceeds 8 MiB');
    const useText=/\.(?:html?|xml|css|m?js|txt|json|csv|svg)$/i.test(file.name);
    const text=new TextDecoder(useText?'utf-8':'latin1').decode(bytes);
    const hay=this.cfg.searchMode==='visible'?visible(text):text;
    this.stats.pagesScanned++;this.stats.bytesScanned+=bytes.byteLength;this.stats.textBytes+=text.length;
    let found=0;
    for(const p of this.patterns){p.re.lastIndex=0;let m;
     while((m=p.re.exec(hay))&&found++<200){
      const a=m.index,b=a+m[0].length;
      this.addMatch(c,p.text,m[0],hay.slice(Math.max(0,a-100),a),hay.slice(b,b+100),'text',200);
      if(!m[0])p.re.lastIndex++;
     }
    }
    this.onUpdate(this.snapshot());
   }catch(e){this.error(file.name,e)}
  }
 }
 async start(config,localFiles=[]){
  if(this.running)throw Error('A scan is already running');
  this.reset();
  this.cfg={searchMode:'source',seeds:'',patterns:'',depth:0,concurrency:6,maxPages:1000,maxPagesPerHost:80,timeoutSeconds:12,respectRobots:true,status2xxOnly:true,providers:[],...config};
  if(!['source','visible','files'].includes(this.cfg.searchMode))throw Error('Unsupported search mode');
  if(this.cfg.searchMode==='files'&&!exts(this.cfg.fileTypes).length)throw Error('Enter file extensions');
  if(this.cfg.searchMode!=='files'&&!lines(this.cfg.patterns).length)throw Error('Enter search phrase');
  this.patterns=this.cfg.searchMode==='files'?[]:lines(this.cfg.patterns).map(text=>({text,re:pattern(text,!!this.cfg.regex,!!this.cfg.caseSensitive)}));
  if(!lines(this.cfg.seeds).length&&!(this.cfg.providers||[]).length&&!localFiles.length)throw Error('Enter targets, local files or enable discovery');
  this.stats.status='running';this.stats.startedAt=new Date().toISOString();this.running=true;this.signal=new AbortController();
  this.log('Browser crawl started (direct requests need CORS or optional extension).');
  try{
   await this.scanLocal(localFiles);
   for(const seed of lines(this.cfg.seeds))this.enqueue(seed);
   await this.discover();
   await Promise.all(Array.from({length:Math.max(1,Math.min(16,Number(this.cfg.concurrency)||6))},()=>this.worker()));
   this.stats.status=this.cancelled?'cancelled':'completed';
  }finally{this.running=false;this.stats.finishedAt=new Date().toISOString();this.onUpdate(this.snapshot())}
  return this.snapshot();
 }
 pause(){if(this.running){this.paused=true;this.stats.status='paused';this.onUpdate(this.snapshot())}}
 resume(){if(this.running){this.paused=false;this.stats.status='running';this.onUpdate(this.snapshot())}}
 stop(){if(this.running){this.cancelled=true;this.paused=false;this.signal?.abort();this.stats.status='stopping';this.onUpdate(this.snapshot())}}
}
const exports={BrowserEngine,visible,references,pattern};
root.FreshSiftBrowserEngine=exports;if(typeof module!=='undefined'&&module.exports)module.exports=exports;
})(typeof window!=='undefined'?window:globalThis);


/* Page-side connector for the optional user-installed extension. */
(function(root){
 if(typeof document==='undefined'||typeof root.postMessage!=='function')return;
 let seq=0;const pending=new Map();const origin=root.location?.origin;
 if(!origin)return;
 const bridge={
  ready:false,
  fetch(url,opt={}){
   return new Promise((resolve,reject)=>{
    const id=++seq;
    const release=()=>{const p=pending.get(id);if(!p)return;clearTimeout(p.timer);opt.signal?.removeEventListener('abort',p.abort);pending.delete(id)};
    const abort=()=>{release();reject(Error('Browser scan cancelled'))};
    if(opt.signal?.aborted){reject(Error('Browser scan cancelled'));return}
    const timer=setTimeout(()=>{release();reject(Error('Extension fetch timed out'))},
      Math.max(5000,(Number(opt.timeoutSeconds)||12)*1000+5000));
    pending.set(id,{resolve,reject,timer,abort});
    opt.signal?.addEventListener('abort',abort,{once:true});
    root.postMessage({freshsift:'request',id,url,method:opt.method||'GET',
      headers:opt.headers||{},allowPrivate:!!opt.allowPrivate,
      timeoutSeconds:opt.timeoutSeconds||12},origin);
   });
  }
 };
 root.FreshSiftExtension=bridge;
 root.addEventListener('message',event=>{
  if(event.source!==root||event.origin!==origin)return;
  const data=event.data;
  if(data?.freshsift==='ready'){bridge.ready=true;return}
  if(data?.freshsift!=='response'||!Number.isSafeInteger(data.id))return;
  const p=pending.get(data.id);if(!p)return;
  clearTimeout(p.timer);pending.delete(data.id);
  if(data.result?.error)p.reject(Error(data.result.error));else p.resolve(data.result);
 });
 root.postMessage({freshsift:'ping'},origin);
})(typeof window!=='undefined'?window:globalThis);
