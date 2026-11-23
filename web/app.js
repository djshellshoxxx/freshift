/* FreshSift unified CDL dashboard: GitHub Pages client or local Go engine. No trackers. */
'use strict';
const $ = id => document.getElementById(id);
const configIDs=['seeds','patterns','regex','caseSensitive','searchMode','fileTypes','recentHours','depth','maxPathSegments','maxExtraDirs','siteMinutes','budgetMode','maxPages','maxPagesPerHost','concurrency','perHostConcurrency','timeoutSeconds','discoveryLimit','sourceQuery','searxngURL','engine-google','engine-bing','engine-duckduckgo','engine-brave','engine-mojeek','engine-startpage','engine-yandex','followExternal','respectRobots','allowPrivate','allowDomains','excludeDomains','urlContains','urlExcludes','enableFTP','excludeFileTypes','allowedTLDs','status2xxOnly','src-urlscan','src-hn','src-brave','src-searxng','src-sitemap','src-rss','src-ct','enumerateSubdomains','subdomainUseBuiltIn','subdomainUseCT','subdomainWords','subdomainLimit','subdomainGuess','discoverDNSRecords','dnsOnly','reverseDNSPages','reverseDNSPageDepth','reverseDNSSubdomains','reverseDNSSubdomainDepth','perpetual','rescanMinutes'];
const sources=['urlscan','hn','brave','searxng','sitemap','rss','ct'];
const searchEngineOptions=[
 ['google','Google','https://www.google.com/search?q='],
 ['bing','Bing','https://www.bing.com/search?q='],
 ['duckduckgo','DuckDuckGo','https://duckduckgo.com/?q='],
 ['brave','Brave','https://search.brave.com/search?q='],
 ['mojeek','Mojeek','https://www.mojeek.com/search?q='],
 ['startpage','Startpage','https://www.startpage.com/sp/search?query='],
 ['yandex','Yandex','https://yandex.com/search/?text=']
];
const sampleData={
 stats:{status:'completed',elapsedSeconds:77,pagesScanned:128,pagesPerMinute:99.7,bytesScanned:20753218,MBScanned:20.75,textMB:18.75,matches:42,matchedPages:18,discovered:192,queued:0,skipped:12,errors:2,activeWorkers:0,lastUrl:'https://example.org/blog/updates'},
 urls:[{url:'https://example.org/release-notes',source:'urlscan',count:8,lastMatch:new Date().toISOString()},{url:'https://docs.example.org/guide/changelog',source:'linked',count:5,lastMatch:new Date().toISOString()},{url:'https://example.net/blog/updates',source:'hn',count:4,lastMatch:new Date().toISOString()},{url:'https://status.example.com/service',source:'sitemap',count:3,lastMatch:new Date().toISOString()},{url:'https://example.org/about/team',source:'urlscan',count:2,lastMatch:new Date().toISOString()}],
 matches:[{id:1,url:'https://example.org/release-notes',source:'urlscan',pattern:'version 2.4',matched:'version 2.4',before:'After several development iterations we have published',after:'with improved compatibility and updated documentation for all users.',area:'Visible text',foundAt:new Date().toISOString()},{id:2,url:'https://docs.example.org/guide/changelog',source:'linked',pattern:'version 2.4',matched:'version 2.4',before:'The software changelog now includes details for',after:'including improvements to indexing performance and export formats.',area:'Visible text',foundAt:new Date().toISOString()},{id:3,url:'https://example.net/blog/updates',source:'hn',pattern:'example phrase',matched:'example phrase',before:'Our community discussion contains this particular',after:'alongside detailed technical notes and benchmark information.',area:'Visible text',foundAt:new Date().toISOString()},{id:4,url:'https://status.example.com/service',source:'sitemap',pattern:'version 2.4',matched:'version 2.4',before:'The current status dashboard has moved to',after:'and all associated checks are reporting normal activity.',area:'Visible text',foundAt:new Date().toISOString()}],
 logs:['02:18:02  DEMO PREVIEW — fictitious example data','02:18:03  urlscan: 70 recently scanned URLs','02:18:04  Hacker News: 40 recently submitted URLs','02:18:05  Starting 24 workers for 110 candidate URLs','02:19:19  Demo complete — no actual websites were scanned']
};
const pagesMode=document.querySelector?.('meta[name="freshsift-frontend"]')?.content==='pages';
let backendBase=pagesMode?(localStorage.getItem('freshsift-engine-url')||''):'';
let apiToken=pagesMode&&typeof sessionStorage!=='undefined'?(sessionStorage.getItem('freshsift-engine-token')||''):'';
let connected=!pagesMode;
let current={stats:{status:'idle'},urls:[],matches:[],logs:[]};let focusURL='';let sortNewest=true;let toastTimer=null;let demoMode=new URLSearchParams(location.search).has('demo');let isBusy=false;let lastStatus='';
let browserLocalFiles=[];
const browserPerpetualKey='freshsift-perpetual-browser-active-v1';
const browserEngine=pagesMode&&window.FreshSiftBrowserEngine?new window.FreshSiftBrowserEngine.BrowserEngine({
 onUpdate:state=>{if(!connected&&!demoMode){current=state;render()}}
}):null;
const value=id=>$(id).value;
const checked=id=>$(id).checked;
const number=id=>Number(value(id));
const fmt=n=>new Intl.NumberFormat(undefined,{maximumFractionDigits:0}).format(n||0);
const escapeDomain=s=>{try{return new URL(s).hostname}catch{return s}};
function makeConfig(){return{
 seeds:value('seeds'),patterns:value('patterns'),regex:checked('regex'),caseSensitive:checked('caseSensitive'),searchMode:value('searchMode'),fileTypes:value('fileTypes'),recentHours:number('recentHours'),depth:number('depth'),maxPathSegments:number('maxPathSegments'),maxExtraDirs:number('maxExtraDirs'),siteMinutes:number('siteMinutes'),budgetMode:value('budgetMode'),maxPages:number('maxPages'),maxPagesPerHost:number('maxPagesPerHost'),concurrency:number('concurrency'),perHostConcurrency:number('perHostConcurrency'),timeoutSeconds:number('timeoutSeconds'),discoveryLimit:number('discoveryLimit'),enumerateSubdomains:checked('enumerateSubdomains'),subdomainUseBuiltIn:checked('subdomainUseBuiltIn'),subdomainUseCT:checked('subdomainUseCT'),subdomainWords:value('subdomainWords'),subdomainLimit:number('subdomainLimit'),subdomainGuess:checked('subdomainGuess'),discoverDNSRecords:checked('discoverDNSRecords'),dnsOnly:checked('dnsOnly'),reverseDNSPages:checked('reverseDNSPages'),reverseDNSPageDepth:number('reverseDNSPageDepth'),reverseDNSSubdomains:checked('reverseDNSSubdomains'),reverseDNSSubdomainDepth:number('reverseDNSSubdomainDepth'),sourceQuery:value('sourceQuery'),searxngURL:value('searxngURL'),searchEngines:searchEngineOptions.filter(([id])=>checked('engine-'+id)).map(([id])=>id),followExternal:checked('followExternal'),respectRobots:checked('respectRobots'),enableFTP:checked('enableFTP'),excludeFileTypes:value('excludeFileTypes'),allowedTLDs:value('allowedTLDs'),allowPrivate:checked('allowPrivate'),allowDomains:value('allowDomains'),excludeDomains:value('excludeDomains'),urlContains:value('urlContains'),urlExcludes:value('urlExcludes'),onlyHTML:false,status2xxOnly:checked('status2xxOnly'),perpetual:checked('perpetual'),rescanMinutes:number('rescanMinutes'),providers:sources.filter(s=>checked('src-'+s))};}
function snapshotForm(){const vals={};for(const id of configIDs){vals[id]=$(id).type==='checkbox'?checked(id):value(id)};return vals}
function loadForm(vals){if(!vals||typeof vals!=='object')return;for(const [id,val] of Object.entries(vals)){if(!configIDs.includes(id)||val===undefined||val===null)continue;const el=$(id);if(el.type==='checkbox')el.checked=Boolean(val);else el.value=String(val)};countTargets()}
function storeForm(){try{localStorage.setItem('freshsift-config-v1',JSON.stringify(snapshotForm()))}catch{}}
function syncSearchMode(){
 const mode=value('searchMode');const files=mode==='files';$('stringSettings').hidden=files;$('fileTypeSettings').hidden=!files;
 $('modeHint').textContent=mode==='source'?'Search downloaded HTML, CSS, JavaScript, binary resources and linked files (including .db). Embedded resources are fetched even at page depth 0. Maximum response 8 MiB.':mode==='visible'?'Match only rendered-style visible HTML text or plain text. Do not match scripts, styles, HTML markup, binary files or linked assets. JavaScript is not executed.':'Discover accessible files by URL extension, e.g. .jpg or .db. Uses HTTP HEAD or FTP SIZE where possible. Recent observation does not establish first search-engine indexing.';
 $('matchesLabel').textContent=files?'FILES FOUND':'TOTAL MATCHES';$('excerptHeading').textContent=files?'Found files':'Matched excerpts';
}
function syncBudgetMode(){const timer=value('budgetMode')==='timer';for(const id of ['depth','maxPathSegments','maxExtraDirs','maxPagesPerHost'])$(id).disabled=timer;$('siteMinutes').disabled=!timer;if(timer&&number('siteMinutes')===0)$('siteMinutes').value='1'}
function countTargets(){const lines=value('seeds').split(/[\n,]+/).map(s=>s.trim()).filter(s=>s&&!s.startsWith('#'));$('targetCount').textContent=fmt(lines.length)+' TARGETS'}
function toast(message,error=false){const t=$('toast');t.textContent=message;t.classList.toggle('error',error);t.hidden=false;clearTimeout(toastTimer);toastTimer=setTimeout(()=>t.hidden=true,4400)}
function message(s,error=false){$('statusMessage').textContent=s;$('statusMessage').classList.toggle('error',error);if(error)toast(s,true)}
function apiURL(path){if(pagesMode&&!connected)throw new Error('Connect an HTTPS Go crawler engine to run live scans from GitHub Pages.');return backendBase+path}
async function apiFetch(path,options={}){const headers={...(options.headers||{})};if(apiToken)headers.Authorization='Bearer '+apiToken;return fetch(apiURL(path),{...options,headers,credentials:'omit',cache:'no-store',redirect:'error'})}
async function request(path,body){const opts={method:'POST',headers:{'Content-Type':'application/json'}};if(body!==undefined)opts.body=JSON.stringify(body);let res;try{res=await apiFetch(path,opts)}catch(e){throw new Error('Cannot reach the Go crawler: '+e.message)};let data;try{data=await res.json()}catch{throw new Error('Invalid response from the crawler')};if(!res.ok)throw new Error(data.error?.message||'Request failed');return data}
function toggleConnectionPanel(show){$('connectionPanel').hidden=!show}
function setConnectionState(status){$('btnConnect').textContent=connected?'Engine connected':'Connect Go engine';$('engineConnectionMessage').textContent=status||(connected?'Connected: Go backend available.':'Browser JavaScript crawling is ready for CORS-accessible HTTP(S) URLs. For broader sites, install the optional FreshSift extension or connect Go for FTP.');$('engineConnectionMessage').classList.remove('error');if(pagesMode){$('clock').textContent=connected?'PAGES · GO ENGINE ONLINE':'PAGES · JS CRAWLER READY';}}
function normalizedEndpoint(raw){const url=new URL(raw);if((url.protocol!=='https:'&&!(url.protocol==='http:'&&['localhost','127.0.0.1','[::1]'].includes(url.hostname)))||url.username||url.password||url.search||url.hash||url.pathname!=='/' )throw new Error('Enter a bare HTTPS engine origin (localhost HTTP allowed for local testing).');return url.origin}
async function connectEngine(){try{const base=normalizedEndpoint(value('engineURL').trim());const token=value('engineToken').trim();if(!token)throw new Error('Enter the engine API token.');let r=await fetch(base+'/api/version',{headers:{Authorization:'Bearer '+token},cache:'no-store',credentials:'omit',redirect:'error'});if(!r.ok)throw new Error('Engine rejected connection: HTTP '+r.status);const data=await r.json();if(typeof data.version!=='string')throw new Error('Endpoint is not a compatible FreshSift engine.');backendBase=base;apiToken=token;connected=true;localStorage.setItem('freshsift-engine-url',base);if(typeof sessionStorage!=='undefined')sessionStorage.setItem('freshsift-engine-token',token);$('engineToken').value='';setConnectionState('Connected to FreshSift '+data.version+' at '+base+' (token held only for this browser session).');toggleConnectionPanel(false);await poll();toast('Crawler engine connected')}catch(e){$('engineConnectionMessage').textContent=e.message;$('engineConnectionMessage').classList.add('error');message(e.message,true)}}
function disconnectEngine(){backendBase='';apiToken='';connected=false;localStorage.removeItem('freshsift-engine-url');if(typeof sessionStorage!=='undefined')sessionStorage.removeItem('freshsift-engine-token');setConnectionState('Engine disconnected. To crawl websites, connect a running HTTPS Go engine.');toggleConnectionPanel(true)}
async function start(){if(isBusy)return;
 if(!connected&&browserEngine){
  if(browserEngine.running){message('Browser scan already running');return}
  demoMode=false;focusURL='';lastStatus='';storeForm();
  const config=makeConfig();
  try{if(config.perpetual)localStorage.setItem(browserPerpetualKey,'1');else localStorage.removeItem(browserPerpetualKey)}catch{}
  message(config.perpetual?'Perpetual browser discovery enabled; leave this tab open or connect an always-on Go engine.':'Starting browser-local crawler. CORS may block some websites; use the extension or Go engine for those destinations.');
  browserEngine.start(config,browserLocalFiles).then(state=>{
   try{localStorage.removeItem(browserPerpetualKey)}catch{}
   current=state;render();message('Browser scan '+state.stats.status+'. '+state.stats.errors+' errors.');
   if(state.stats.status==='completed'||state.stats.status==='cancelled')saveHistory(state);
  }).catch(e=>{try{localStorage.removeItem(browserPerpetualKey)}catch{}message('Browser scan failed: '+e.message,true)});
  return;
 }
 if(!connected){toggleConnectionPanel(true);message('Connect the Go crawler engine before starting a scan.',true);return};const config=makeConfig();if(config.searchMode!=='files'&&!config.patterns.trim()&&!config.enumerateSubdomains&&!config.discoverDNSRecords&&!config.reverseDNSPages){message('Enter search phrases or enable discovery.',true);$('patterns').focus();return};if(config.searchMode==='files'&&!config.fileTypes.trim()){message('Enter at least one file extension, such as .jpg.',true);$('fileTypes').focus();return};if(!config.seeds.trim()&&config.providers.length===0){message('Paste targets or enable a discovery source.',true);return}
 isBusy=true;try{await request('/api/start',config);demoMode=false;focusURL='';lastStatus='';message('Scan started. '+(config.enumerateSubdomains?'Enumerating seed subdomains…':'Discovering recent URLs…'));storeForm();await poll()}catch(e){message(e.message,true)}finally{isBusy=false}}
async function stop(){if(!connected&&browserEngine){try{localStorage.removeItem(browserPerpetualKey)}catch{}browserEngine.stop();message('Stopping browser scan');return}try{await request('/api/stop');message('Stopping active requests…')}catch(e){message(e.message,true)}}
async function pauseResume(){if(!connected&&browserEngine){if(browserEngine.paused){browserEngine.resume();message('Browser scan resumed')}else{browserEngine.pause();message('Browser scan paused')}return}try{let action=current.stats.status==='paused'?'resume':'pause';await request('/api/'+action);message(action==='pause'?'Pause requested.':'Resuming scan…');await poll()}catch(e){message(e.message,true)}}
async function poll(){if(demoMode||!connected)return;try{const res=await apiFetch('/api/status');if(!res.ok)throw new Error('HTTP '+res.status);const data=await res.json();current=data;render();if(data.stats.status!==lastStatus){lastStatus=data.stats.status;if(data.stats.status==='completed'){message('Scan complete. Export your matches if needed.');saveHistory(data)}if(data.stats.status==='cancelled'){message('Scan cancelled. Collected matches are available for export.');saveHistory(data)}}}catch(e){message('Status connection error: '+e.message,true)}}
function timeFmt(seconds){seconds=Math.floor(seconds||0);return String(Math.floor(seconds/60)).padStart(2,'0')+':'+String(seconds%60).padStart(2,'0')}
function renderStats(){const s=current.stats||{};const state=s.status||'idle';$('statRate').replaceChildren(document.createTextNode(fmt(Math.round(s.pagesPerMinute||0))));const rateUnit=document.createElement('span');rateUnit.className='metric-unit';rateUnit.textContent=' / min';$('statRate').append(rateUnit);
 $('statMB').replaceChildren(document.createTextNode((s.textMB||0).toFixed(2)));const mb=document.createElement('span');mb.className='metric-unit';mb.textContent=' MB';$('statMB').append(mb);
 $('statMatches').textContent=fmt(s.matches);$('statMatchedPages').textContent=fmt(s.matchedPages);$('statScanned').textContent=fmt(s.pagesScanned)+' resources scanned';$('statDiscovered').textContent=fmt(s.discovered)+' URLs queued / discovered'+(s.cycles?' · round '+fmt(s.cycles):'');$('statElapsed').textContent=timeFmt(s.elapsedSeconds);$('statActive').textContent=fmt(s.activeWorkers)+' workers';$('statErrors').textContent=fmt(s.errors)+' errors';$('scanUrl').textContent=state==='waiting'&&s.nextRunAt?'Next discovery at '+new Date(s.nextRunAt).toLocaleTimeString():s.lastUrl||'Enter targets or enable discovery to get started.';
 const names={idle:'Waiting for scan',discovering:'Discovering URLs',running:'Scanning pages',waiting:'Waiting for next discovery round',paused:'Scan paused',stopping:'Stopping scan',completed:'Scan complete',cancelled:'Scan cancelled'};$('scanStatus').textContent=names[state]||state;
 $('stateText').textContent=state.toUpperCase();$('stateChip').classList.toggle('busy',['running','discovering'].includes(state));$('stateChip').classList.toggle('failed',state==='cancelled');document.querySelector('.pulse-dot').classList.toggle('busy',state==='running');
 const active=['running','discovering','waiting','paused','stopping'].includes(state);$('startBtn').disabled=active;$('pauseBtn').disabled=!['paused','running','waiting'].includes(state);$('pauseBtn').textContent=state==='paused'?'▶':'Ⅱ';$('pauseBtn').title=state==='paused'?'Resume scan':'Pause scan';$('stopBtn').disabled=!active}
function refreshSourceSelect(id,values){const el=$(id),selected=el.value,known=Array.from(new Set(values.filter(Boolean))).sort();el.replaceChildren();const all=document.createElement('option');all.value='';all.textContent='All sources';el.append(all);for(const name of known){const opt=document.createElement('option');opt.value=name;opt.textContent=name;el.append(opt)}if(known.includes(selected))el.value=selected}
function emptyPanel(title,description){const outer=document.createElement('div');outer.className='empty';const icon=document.createElement('span');icon.className='empty-icon';icon.textContent='⌕';const bold=document.createElement('strong');bold.textContent=title;const small=document.createElement('span');small.textContent=description;outer.append(icon,bold,small);return outer}
function renderURLs(){const list=$('urls');list.replaceChildren();const filt=value('urlFilter').toLowerCase();const src=value('urlSourceFilter');let rows=(current.urls||[]).filter(x=>(x.url+' '+(x.ip||'')).toLowerCase().includes(filt)&&(!src||src===x.source));rows=rows.slice().sort((a,b)=>sortNewest?Date.parse(b.lastMatch)-Date.parse(a.lastMatch):b.count-a.count);
 $('urlCount').textContent=fmt(current.stats?.matchedPages);$('urlFooter').textContent=fmt(rows.length)+' matching URLs shown';if(rows.length===0){list.append(emptyPanel('No matching URLs','Try a broader search or clear your filters.'));return}
 for(const row of rows){const b=document.createElement('button');b.type='button';b.className='url-row'+(row.url===focusURL?' selected':'');b.title='Focus excerpts for '+row.url;const left=document.createElement('span');left.className='url-main';const icon=document.createElement('span');icon.className='url-globe';icon.textContent='↗';const data=document.createElement('span');data.className='url-data';const host=document.createElement('div');host.className='url-host';host.textContent=escapeDomain(row.url);const path=document.createElement('div');path.className='url-path';try{const u=new URL(row.url);path.textContent=u.pathname+(u.search||'')+(row.ip?' · '+row.ip:'')}catch{path.textContent=row.url};data.append(host,path);left.append(icon,data);const count=document.createElement('span');count.className='url-badge';count.textContent=fmt(row.count);b.append(left,count);b.addEventListener('click',()=>{focusURL=focusURL===row.url?'':row.url;renderURLs();renderMatches()});list.append(b)}}
function renderMatches(){const list=$('matches');list.replaceChildren();const filt=value('matchFilter').toLowerCase();const src=value('matchSourceFilter');const rows=(current.matches||[]).filter(x=>(!focusURL||x.url===focusURL)&&(!src||x.source===src)&&([x.matched,x.pattern,x.context,x.url,x.ip||''].join(' ').toLowerCase().includes(filt))).slice().reverse();$('matchCount').textContent=fmt(current.stats?.matches);$('matchFooter').textContent=fmt(rows.length)+' excerpts shown';$('clearFocus').textContent=focusURL?'Clear URL filter':'Show all';if(rows.length===0){list.append(emptyPanel('No excerpts to display','Matches will appear here with surrounding text.'));return}
 for(const m of rows){const el=document.createElement('article');el.className='excerpt';const top=document.createElement('div');top.className='excerpt-top';const phrase=document.createElement('span');phrase.className='phrase';phrase.title=m.pattern;phrase.textContent=m.pattern;const short=document.createElement('a');short.className='excerpt-short';short.href=m.url;short.rel='noopener noreferrer';short.target='_blank';short.title=m.url;short.textContent=escapeDomain(m.url);const tag=document.createElement('span');tag.className='source-tag';tag.textContent=m.source;const time=document.createElement('span');time.className='excerpt-time';time.textContent=m.foundAt?new Date(m.foundAt).toLocaleTimeString([], {hour:'2-digit',minute:'2-digit'}):'–';top.append(phrase,short,tag,time);const body=document.createElement('div');body.className='excerpt-content';if(m.kind==='file'){body.textContent='Verified file: '+m.matched+' · '+m.pattern+' · '+(m.status||'FTP')+' status';}else{body.append(document.createTextNode((m.before||'')+' '));const mark=document.createElement('mark');mark.textContent=m.matched;body.append(mark,document.createTextNode(' '+(m.after||'')));}
 const link=document.createElement('a');link.className='excerpt-url';link.href=m.url;link.rel='noopener noreferrer';link.target='_blank';link.title=m.url;link.textContent=m.url;el.append(top,body,link);list.append(el)}}
function renderLog(){const log=$('activity');log.replaceChildren();const lines=current.logs||[];if(!lines.length){const empty=document.createElement('div');empty.className='activity-empty';empty.textContent='No activity yet. Press GO to begin.';log.append(empty);return};for(const txt of lines.slice(-35)){const div=document.createElement('div');div.className='activity-line';if(txt.includes('ERROR'))div.classList.add('warn');div.textContent=txt;log.append(div)};log.scrollTop=log.scrollHeight}

function renderSubdomains(){
 const list=$('subdomainResults');list.replaceChildren();
 const rows=(current.subdomains||[]).filter(r=>(r.hostname+' '+r.source+' '+(r.addresses||[]).join(' ')).toLowerCase().includes(value('subdomainFilter').toLowerCase()));
 $('subdomainCount').textContent=fmt(current.stats?.subdomains||current.subdomains?.length||0);
 if(!rows.length){list.textContent='No subdomains discovered yet. Enable enumeration and supply at least one seed domain.';return}
 for(const item of rows){
  const div=document.createElement('div');div.className='subdomain-row';
  const host=document.createElement('span');host.className='subdomain-host';host.textContent=item.hostname;
  const meta=document.createElement('span');meta.className='subdomain-meta';
  meta.textContent=(item.verified?'DNS verified':'CT historical')+' · '+item.source+(item.addresses?.length?' · '+item.addresses.join(', '):'');
  div.append(host,meta);list.append(div);
 }
}
function countSubdomainWords(){
 try{
  const engine=window.FreshSiftSubdomains;
  const n=checked('subdomainGuess')?(engine?.parseWords(value('subdomainWords'),checked('subdomainUseBuiltIn')).length||0):0;
  $('subdomainWordCount').textContent=fmt(n)+' candidate words loaded';
 }catch(e){$('subdomainWordCount').textContent=e.message}
}
async function importSubdomainFile(file){
 if(!file)return;
 if(file.size>180000){toast('Subdomain wordlist must be 180 KB or less',true);return}
 try{
  const txt=await file.text();
  const combined=[value('subdomainWords').trim(),txt.trim()].filter(Boolean).join('\n');
  if(combined.length>200000)throw Error('Subdomain wordlist exceeds 200 KB');
  if(!window.FreshSiftSubdomains?.parseWords(combined,false).length)throw Error('No valid subdomain prefixes in file');
  $('subdomainWords').value=combined;
  $('enumerateSubdomains').checked=true;$('subdomainGuess').checked=true;
  countSubdomainWords();storeForm();toast('Imported subdomain wordlist: '+file.name);
 }catch(e){toast(e.message,true)}
}
function subdomainCSV(rows){
 return [['Hostname','Source','DNS Verified','Addresses'],...rows.map(r=>[r.hostname,r.source,Boolean(r.verified),(r.addresses||[]).join('; ')])]
  .map(a=>a.map(csvCell).join(',')).join('\r\n')+'\r\n';
}
async function loadSubdomains(){
 if(!connected&&browserEngine)return browserEngine.allSubdomains();
 if(!connected)return current.subdomains||[];
 const r=await apiFetch('/api/subdomains?format=json');
 if(!r.ok)throw Error('Subdomain export HTTP '+r.status);
 const data=await r.json();
 if(!Array.isArray(data))throw Error('Invalid subdomain API response');
 return data;
}
async function exportSubdomains(){
 try{
  const rows=await loadSubdomains();
  if(!rows.length){toast('No subdomains discovered to export',true);return}
  downloadBlob(subdomainCSV(rows),'freshsift-subdomains.csv','text/csv;charset=utf-8');
  toast('Exported '+fmt(rows.length)+' discovered subdomains');
 }catch(e){toast(e.message,true)}
}
async function copySubdomains(){
 try{
  const rows=await loadSubdomains();
  if(!rows.length){toast('No subdomains to copy',true);return}
  await navigator.clipboard.writeText(rows.map(x=>x.hostname).join('\n'));
  toast('Copied '+fmt(rows.length)+' subdomain hostnames');
 }catch(e){toast('Clipboard unavailable: '+e.message,true)}
}

function renderDNSRecords(){
 const target=$('dnsRecordResults');target.replaceChildren();
 const text=value('dnsRecordFilter').toLowerCase();
 const rows=(current.dnsRecords||[]).filter(x=>[x.domain,x.type,x.value].join(' ').toLowerCase().includes(text));
 $('dnsRecordCount').textContent=fmt(current.stats?.dnsRecords||rows.length);
 if(!rows.length){target.textContent='No DNS records found. Enable DNS record lookup and scan a seed domain.';return}
 for(const item of rows){
  const line=document.createElement('div');line.className='subdomain-row';
  const host=document.createElement('span');host.className='subdomain-host';host.textContent=item.domain+' · '+item.type;
  const value=document.createElement('span');value.className='subdomain-meta';value.textContent=item.value;
  line.append(host,value);target.append(line);
 }
}
function renderReverseDNS(){
 const target=$('reverseDNSResults');target.replaceChildren();
 const text=value('reverseDNSFilter').toLowerCase();
 const rows=(current.reverseDNS||[]).filter(x=>[x.ip,x.hostname,x.origin].join(' ').toLowerCase().includes(text));
 $('reverseDNSCount').textContent=fmt(current.stats?.reverseHosts||rows.length);
 if(!rows.length){target.textContent='No forward-confirmed PTR hostnames found.';return}
 for(const item of rows){
  const line=document.createElement('div');line.className='subdomain-row';
  const host=document.createElement('span');host.className='subdomain-host';host.textContent=item.hostname;
  const meta=document.createElement('span');meta.className='subdomain-meta';meta.textContent=item.ip+' · '+item.origin+' · '+(item.verified?'forward verified':'unverified');
  line.append(host,meta);target.append(line);
 }
}
async function loadDNSRecords(){
 if(!connected&&browserEngine)return browserEngine.allDNSRecords();
 if(!connected)return current.dnsRecords||[];
 const r=await apiFetch('/api/dns-records?format=json');
 if(!r.ok)throw Error('DNS export HTTP '+r.status);
 const rows=await r.json();if(!Array.isArray(rows))throw Error('Unexpected DNS response');
 return rows;
}
async function loadReverseDNS(){
 if(!connected&&browserEngine)return browserEngine.allReverseDNS();
 if(!connected)return current.reverseDNS||[];
 const r=await apiFetch('/api/reverse-dns?format=json');
 if(!r.ok)throw Error('PTR export HTTP '+r.status);
 const rows=await r.json();if(!Array.isArray(rows))throw Error('Unexpected PTR response');
 return rows;
}
async function exportDNSRecords(){
 try{
  const rows=await loadDNSRecords();
  if(!rows.length){toast('No DNS records to export',true);return}
  const csv=[['Domain','Type','Value'],...rows.map(x=>[x.domain,x.type,x.value])].map(x=>x.map(csvCell).join(',')).join('\r\n')+'\r\n';
  downloadBlob(csv,'freshsift-dns-records.csv','text/csv;charset=utf-8');
 }catch(e){toast(e.message,true)}
}
async function exportReverseDNS(){
 try{
  const rows=await loadReverseDNS();
  if(!rows.length){toast('No PTR hostnames to export',true);return}
  const csv=[['IP','PTR Hostname','Origin','Forward Verified'],...rows.map(x=>[x.ip,x.hostname,x.origin,Boolean(x.verified)])].map(x=>x.map(csvCell).join(',')).join('\r\n')+'\r\n';
  downloadBlob(csv,'freshsift-reverse-dns.csv','text/csv;charset=utf-8');
 }catch(e){toast(e.message,true)}
}
function render(){refreshSourceSelect('urlSourceFilter',(current.urls||[]).map(x=>x.source));refreshSourceSelect('matchSourceFilter',(current.matches||[]).map(x=>x.source));renderStats();renderURLs();renderMatches();renderLog();renderSubdomains();renderDNSRecords();renderReverseDNS()}
async function importFile(file){if(!file)return;if(file.size>2_000_000){message('File too large (2 MB maximum).',true);return};const content=await file.text();const lines=content.split(/\r?\n/);const urls=[];for(let line of lines){line=line.trim();if(!line||line.startsWith('#'))continue;if(file.name.toLowerCase().endsWith('.csv'))line=line.split(',')[0].replace(/^"|"$/g,'').trim();if(/^(url|website|domain|host|ip)$/i.test(line))continue;urls.push(line)}if(!urls.length){message('No URL lines found in that file.',true);return};$('seeds').value=[$('seeds').value.trim(),...urls].filter(Boolean).join('\n');$('fileHint').textContent=file.name+' · '+fmt(urls.length)+' added';countTargets();storeForm();toast('Imported '+fmt(urls.length)+' candidate targets')}
function updatePresets(){const el=$('presetSelect'),stored=JSON.parse(localStorage.getItem('freshsift-presets-v1')||'{}');el.replaceChildren();const opt=document.createElement('option');opt.value='';opt.textContent='Load preset';el.append(opt);for(const name of Object.keys(stored).sort()){const next=document.createElement('option');next.value=name;next.textContent=name;el.append(next)}}
function savePreset(){const name=window.prompt('Name this configuration preset:');if(!name)return;const trimmed=name.trim().slice(0,60);if(!trimmed)return;const stored=JSON.parse(localStorage.getItem('freshsift-presets-v1')||'{}');stored[trimmed]=snapshotForm();localStorage.setItem('freshsift-presets-v1',JSON.stringify(stored));updatePresets();$('presetSelect').value=trimmed;toast('Preset saved locally in this browser')}
function openSearchLinks(){const query=value('engineQuery').trim(),box=$('engineLinks');box.replaceChildren();if(!query)return;let count=0;for(const [id,name,prefix] of searchEngineOptions){if(!checked('engine-'+id))continue;const link=document.createElement('a');link.className='engine-link';link.href=prefix+encodeURIComponent(query);link.target='_blank';link.rel='noopener noreferrer';link.textContent=name+' ↗';box.append(link);count++}if(!count)box.textContent='Select one or more engines above.'}
async function lookupIP(){const ip=value('ipInput').trim();if(!ip){toast('Enter a public IP address',true);return};const pre=$('ipOutput');pre.hidden=false;pre.textContent='Looking up public IP…';try{
 if(!connected&&browserEngine){
  const bits=ip.split('.').map(Number);
  if(bits.length!==4||bits.some(n=>!Number.isInteger(n)||n<0||n>255))throw Error('Enter a valid public IPv4 address');
  const privateIP=bits[0]===0||bits[0]===10||bits[0]===127||bits[0]>=224||
   bits[0]===169&&bits[1]===254||bits[0]===172&&bits[1]>=16&&bits[1]<=31||
   bits[0]===192&&bits[1]===168;
  if(privateIP)throw Error('InternetDB requires a public IPv4 address');
  const endpoint='https://internetdb.shodan.io/'+encodeURIComponent(ip);
  let result;
  if(window.FreshSiftExtension?.ready){
   const remote=await window.FreshSiftExtension.fetch(endpoint,{method:'GET',timeoutSeconds:12});
   if(remote.status!==200)throw Error('InternetDB HTTP '+remote.status);
   result=JSON.parse(remote.body);
  }else{
   const remote=await fetch(endpoint,{method:'GET',mode:'cors',credentials:'omit',cache:'no-store',redirect:'error'});
   if(!remote.ok)throw Error('InternetDB HTTP '+remote.status);
   result=await remote.json();
  }
  pre.textContent=JSON.stringify(result,null,2);return;
 }
 const res=await apiFetch('/api/ip?ip='+encodeURIComponent(ip));const data=await res.json();if(!res.ok)throw new Error(data.error?.message||'Lookup failed');pre.textContent=JSON.stringify(data,null,2)}catch(e){pre.textContent=e.message}}
function downloadBlob(data,filename,mimetype){const blob=new Blob([data],{type:mimetype}),url=URL.createObjectURL(blob),a=document.createElement('a');a.href=url;a.download=filename;document.body.append(a);a.click();a.remove();setTimeout(()=>URL.revokeObjectURL(url),1000)}
async function downloadAll(){
 if(!connected&&browserEngine){
  const rows=browserEngine.allResults();if(!rows.length){toast('No browser scan results to export',true);return}
  const format=value('exportFormat');let data,contentType='application/json';
  if(format==='json')data=JSON.stringify(rows,null,2);
  else if(format==='jsonl'){data=rows.map(x=>JSON.stringify(x)).join('\n')+'\n';contentType='application/x-ndjson'}
  else{
   const columns=['URL','Matched String','Pattern','Before','After','Context','Source','First Observed At','Found At','IP','Kind','Area','HTTP Status'];
   const key=['url','matched','pattern','before','after','context','source','observedAt','foundAt','ip','kind','area','status'];
   data=[columns,...rows.map(r=>key.map(k=>r[k]))].map(r=>r.map(csvCell).join(',')).join('\r\n')+'\r\n';
   contentType='text/csv;charset=utf-8';
  }
  downloadBlob(data,'freshsift-browser-results.'+format,contentType);return;
 }
 if(!connected){toggleConnectionPanel(true);toast('Connect crawler to export full results',true);return}try{const format=value('exportFormat'),r=await apiFetch('/api/export?format='+encodeURIComponent(format));if(!r.ok)throw new Error('Export HTTP '+r.status);const blob=await r.blob();downloadBlob(blob,'freshsift-results.'+(format==='jsonl'?'jsonl':format),r.headers.get('content-type')||'application/octet-stream')}catch(e){toast(e.message,true)}}
function csvCell(v){let s=String(v??'');if(/^[=+\-@\t\r\n]/.test(s))s="'"+s;return '"'+s.replaceAll('"','""')+'"'}
function filteredMatches(all){const urlFilter=value('urlFilter').toLowerCase(),urlSource=value('urlSourceFilter'),excerptFilter=value('matchFilter').toLowerCase(),excerptSource=value('matchSourceFilter');return all.filter(m=>(!focusURL||m.url===focusURL)&&(!urlSource||m.source===urlSource)&&(!excerptSource||m.source===excerptSource)&&(!urlFilter||(m.url+' '+(m.ip||'')).toLowerCase().includes(urlFilter))&&(!excerptFilter||[m.matched,m.pattern,m.context,m.url,m.ip||''].join(' ').toLowerCase().includes(excerptFilter)))}
function matchesCSV(all){const hdr=['URL','Matched String','Pattern','Before','After','Context','Source','First Observed At','Found At','IP','Kind','Area','HTTP Status'];const rows=filteredMatches(all).map(m=>[m.url,m.matched,m.pattern,m.before,m.after,m.context,m.source,m.observedAt,m.foundAt,m.ip,m.kind||'text',m.area,m.status]);return [hdr,...rows].map(row=>row.map(csvCell).join(',')).join('\r\n')+'\r\n'}
async function downloadFilteredCSV(){
 if(!connected&&browserEngine){
  const rows=browserEngine.allResults(),filtered=filteredMatches(rows);
  if(!filtered.length){toast('No matching browser results',true);return}
  downloadBlob(matchesCSV(rows),'freshsift-browser-filtered.csv','text/csv;charset=utf-8');
  toast('Saved '+filtered.length+' matches');return;
 }
 if(!connected){toggleConnectionPanel(true);toast('Connect crawler to export results',true);return}try{const res=await apiFetch('/api/export?format=json');if(!res.ok)throw new Error('Export HTTP '+res.status);const results=await res.json();if(!Array.isArray(results))throw new Error('Unexpected export format');const selected=filteredMatches(results);if(!selected.length){toast('No results match the current URL / excerpt filters',true);return}downloadBlob(matchesCSV(results),'freshsift-filtered-matches.csv','text/csv;charset=utf-8');toast('Saved '+selected.length+' rows with URL, matched string, and context')}catch(e){toast(e.message,true)}}
function previewScope(){const c=makeConfig(),seeds=c.seeds.split(/[\n,]+/).map(s=>s.trim()).filter(Boolean),summary=[
 'MODE: '+({source:'Whole source + linked resources',visible:'Visible page text',files:'File URLs by extension'})[c.searchMode],
 'MANUAL SEEDS: '+seeds.length+'   DISCOVERY: '+(c.providers.join(', ')||'none'),
 'SEARCH ENGINES: '+(c.searchEngines.join(', ')||'none')+'   SEARXNG: '+(c.providers.includes('searxng')?(c.searxngURL||'URL required'):'off'),
 'SUBDOMAINS: '+(c.enumerateSubdomains?('enabled · limit '+c.subdomainLimit+' · guesses '+c.subdomainGuess+' · CT '+c.subdomainUseCT):'off'),
 'DNS RECORDS: '+c.discoverDNSRecords+'   DNS ONLY: '+c.dnsOnly+'   PAGE PTR: '+c.reverseDNSPages+' / depth '+c.reverseDNSPageDepth+'   SUBDOMAIN PTR: '+c.reverseDNSSubdomains+' / depth '+c.reverseDNSSubdomainDepth,
 'MATCHING: '+(c.searchMode==='files'?(c.fileTypes||'[none]'):(c.regex?'RE2 regex':'Exact strings')+', '+(c.caseSensitive?'case-sensitive':'case-insensitive')),
 'EXCLUDED TYPES: '+(c.excludeFileTypes||'none')+'   TLDs: '+(c.allowedTLDs||'all / IPs permitted'),
 'ROBOTS.TXT: '+(c.respectRobots?'follow':'not enforced')+'   FTP: '+(c.enableFTP?'enabled':'disabled'),
 'RECURSION: '+c.depth+'   MAX RESOURCES: '+c.maxPages+'   WORKERS: '+c.concurrency,
 'PER-SITE TIMER: '+(c.budgetMode==='timer'?c.siteMinutes+' min':'off')+'   PRIVATE IPs: '+(c.allowPrivate?'allowed':'blocked'),
 'NOTE: Feed observation time does not establish search-engine indexing date.' ];$('scopeReport').textContent=summary.join('\n');$('scopeReport').hidden=false}
function saveHistory(data){try{const entries=JSON.parse(localStorage.getItem('freshsift-scan-history')||'[]');entries.unshift({date:new Date().toISOString(),status:data.stats.status,mode:value('searchMode'),scanned:data.stats.pagesScanned||0,matches:data.stats.matches||0,urls:data.stats.matchedPages||0});localStorage.setItem('freshsift-scan-history',JSON.stringify(entries.slice(0,12)));renderHistory()}catch{}}
function renderHistory(){const el=$('scanHistory');el.replaceChildren();let history=[];try{history=JSON.parse(localStorage.getItem('freshsift-scan-history')||'[]')}catch{};if(!history.length){el.textContent='No completed scans in this browser yet.';return}for(const h of history){const div=document.createElement('div');div.className='history-entry';div.textContent=new Date(h.date).toLocaleString()+' · '+h.status+' · '+h.mode+' · '+fmt(h.scanned)+' scanned · '+fmt(h.matches)+' matches / '+fmt(h.urls)+' URLs';el.append(div)}}
function exportSettings(){downloadBlob(JSON.stringify({schema:'freshsift-config-v1',settings:snapshotForm()},null,2),'freshsift-settings.json','application/json')}
async function importSettings(file){if(!file)return;if(file.size>100000){toast('Settings file exceeds 100 KB',true);return}try{const obj=JSON.parse(await file.text());if(obj.schema!=='freshsift-config-v1'||!obj.settings||typeof obj.settings!=='object')throw Error('Not a FreshSift settings export');loadForm(obj.settings);syncSearchMode();syncBudgetMode();storeForm();toast('Settings imported')}catch(e){toast(e.message,true)}}
function wire(){if(pagesMode){$('localFiles').addEventListener('change',e=>{browserLocalFiles=[...(e.target.files||[])];$('localCount').textContent=browserLocalFiles.length+' local files selected'});}for(const id of configIDs){$(id).addEventListener('change',()=>{storeForm();if(id==='seeds')countTargets();if(id==='budgetMode')syncBudgetMode();if(id==='searchMode')syncSearchMode();if(id==='subdomainUseBuiltIn'||id==='subdomainWords'||id==='subdomainGuess')countSubdomainWords();if(id.startsWith('engine-'))openSearchLinks();
  if(id==='dnsOnly'&&checked('dnsOnly'))$('discoverDNSRecords').checked=true;
  if(id==='reverseDNSSubdomains'&&checked('reverseDNSSubdomains'))$('enumerateSubdomains').checked=true});if(['seeds','patterns','sourceQuery','searxngURL','allowDomains','excludeDomains','urlContains','urlExcludes','fileTypes','excludeFileTypes','allowedTLDs','subdomainWords'].includes(id))$(id).addEventListener('input',()=>{if(id==='seeds')countTargets();storeForm()})}
 $('allSearchEngines').addEventListener('change',()=>{for(const [id] of searchEngineOptions)$('engine-'+id).checked=$('allSearchEngines').checked;storeForm();openSearchLinks()});
 $('subdomainImportBtn').addEventListener('click',()=>$('subdomainFile').click());
 $('subdomainFile').addEventListener('change',e=>importSubdomainFile(e.target.files?.[0]));
 $('subdomainFilter').addEventListener('input',renderSubdomains);
 $('dnsRecordFilter').addEventListener('input',renderDNSRecords);
 $('reverseDNSFilter').addEventListener('input',renderReverseDNS);
 $('dnsRecordExport').addEventListener('click',exportDNSRecords);
 $('reverseDNSExport').addEventListener('click',exportReverseDNS);
 $('subdomainExport').addEventListener('click',exportSubdomains);
 $('subdomainCopy').addEventListener('click',copySubdomains);
 $('subdomainWords').addEventListener('input',countSubdomainWords);
 $('startBtn').addEventListener('click',start);$('stopBtn').addEventListener('click',stop);$('pauseBtn').addEventListener('click',pauseResume);$('chooseFile').addEventListener('click',()=> $('fileInput').click());$('fileInput').addEventListener('change',e=>importFile(e.target.files?.[0]));$('clearSeeds').addEventListener('click',()=>{$('seeds').value='';countTargets();storeForm()});
 const dz=$('dropzone');for(const evt of ['dragover','dragenter'])dz.addEventListener(evt,e=>{e.preventDefault();dz.classList.add('dragging')});for(const evt of ['dragleave','drop'])dz.addEventListener(evt,e=>{e.preventDefault();dz.classList.remove('dragging')});dz.addEventListener('drop',e=>importFile(e.dataTransfer?.files?.[0]));
 $('urlFilter').addEventListener('input',renderURLs);$('urlSourceFilter').addEventListener('change',renderURLs);$('matchFilter').addEventListener('input',renderMatches);$('matchSourceFilter').addEventListener('change',renderMatches);
 $('sortUrls').addEventListener('click',()=>{sortNewest=!sortNewest;$('sortUrls').textContent=sortNewest?'Newest ↓':'Most matches ↓';renderURLs()});$('clearFocus').addEventListener('click',()=>{focusURL='';renderURLs();renderMatches()});
 $('saveBtn').addEventListener('click',downloadAll);$('filteredCSV').addEventListener('click',downloadFilteredCSV);
 $('copyUrls').addEventListener('click',async()=>{const text=(current.urls||[]).map(x=>x.url).join('\n');if(!text){toast('There are no matching URLs to copy',true);return}try{await navigator.clipboard.writeText(text);toast('Copied '+current.urls.length+' matching URLs')}catch(e){toast('Clipboard unavailable: '+e.message,true)}});
 $('btnConnect').addEventListener('click',()=>{if(!pagesMode){toast('The local engine is already connected.');return};toggleConnectionPanel($('connectionPanel').hidden)});
 $('connectSubmit').addEventListener('click',connectEngine);$('disconnectEngine').addEventListener('click',disconnectEngine);$('connectionClose').addEventListener('click',()=>toggleConnectionPanel(false));
 $('engineURL').addEventListener('keydown',e=>{if(e.key==='Enter')connectEngine()});$('engineToken').addEventListener('keydown',e=>{if(e.key==='Enter')connectEngine()});
 $('scopePreview').addEventListener('click',previewScope);$('exportConfig').addEventListener('click',exportSettings);$('importConfigBtn').addEventListener('click',()=>$('importConfigFile').click());$('importConfigFile').addEventListener('change',e=>importSettings(e.target.files?.[0]));
 $('btnPresets').addEventListener('click',savePreset);$('presetSelect').addEventListener('change',()=>{const presets=JSON.parse(localStorage.getItem('freshsift-presets-v1')||'{}');const selected=presets[value('presetSelect')];if(selected){loadForm(selected);syncBudgetMode();syncSearchMode();storeForm();toast('Preset loaded')}});
 $('servicePreset').addEventListener('change',()=>{$('engineQuery').value=value('servicePreset');openSearchLinks()});$('engineQuery').addEventListener('input',openSearchLinks);$('ipLookup').addEventListener('click',lookupIP);
 document.addEventListener('keydown',e=>{if(e.ctrlKey&&e.key==='Enter'){e.preventDefault();start()}if(e.shiftKey&&e.key==='Escape'){e.preventDefault();stop()}})}
function init(){try{loadForm(JSON.parse(localStorage.getItem('freshsift-config-v1')||'null'))}catch{};try{updatePresets()}catch{};wire();syncBudgetMode();syncSearchMode();countSubdomainWords();$('clock').textContent=(pagesMode?'PAGES CLIENT':'LOCAL ENGINE')+' · '+new Date().toLocaleDateString(undefined,{month:'short',day:'numeric'}).toUpperCase();renderHistory();if(pagesMode){$('browserLocalGroup').hidden=false;$('browserModeNote').hidden=false;$('engineURL').value=backendBase;toggleConnectionPanel(false);setConnectionState();if(backendBase&&apiToken){connected=true;setConnectionState();}}else{$('btnConnect').textContent='Local engine';}if(demoMode){current=sampleData;$('patterns').value='version 2.4\nexample phrase';$('seeds').value='https://example.org\nhttps://example.net';countTargets();message('DEMO PREVIEW — showing fictitious example results, not a live scan.');render()}else{if(connected)poll();setInterval(poll,1000);if(pagesMode&&!connected&&checked('perpetual')){try{if(localStorage.getItem(browserPerpetualKey)==='1')start()}catch{}}}}
document.addEventListener('DOMContentLoaded',init);
