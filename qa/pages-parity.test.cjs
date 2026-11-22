'use strict';
// No browser installation needed: verify Pages is the same dashboard and exercise its core client helpers.
const fs=require('node:fs');const assert=require('node:assert/strict');const vm=require('node:vm');
const web=fs.readFileSync('web/index.html','utf8');
const pages=fs.readFileSync('docs/index.html','utf8');
assert.equal(pages.replace('\n<meta name="freshsift-frontend" content="pages">',''),web,'Pages HTML diverges from Go dashboard');
for(const f of ['app.js','style.css','favicon.svg'])assert.equal(fs.readFileSync('docs/'+f,'utf8'),fs.readFileSync('web/'+f,'utf8'),`Pages asset ${f} diverges from desktop`);
const ids=[...pages.matchAll(/\bid="([^"]+)"/g)].map(x=>x[1]);assert.equal(ids.length,new Set(ids).size,'duplicate page control IDs');
for(const id of ['searchMode','seeds','patterns','regex','fileTypes','excludeFileTypes','allowedTLDs','respectRobots','enableFTP','depth','budgetMode','siteMinutes','sourceQuery','src-urlscan','src-ct','filteredCSV','saveBtn','urlFilter','matchFilter','btnConnect','engineToken','engineURL','scopePreview','exportConfig','importConfigBtn','scanHistory'])assert(ids.includes(id),'missing feature from Pages: '+id);
const nodes=Object.fromEntries(ids.map(id=>[id,{id,value:'',checked:false,type:'text',hidden:false,textContent:'',classList:{add(){},remove(){}},replaceChildren(){},append(){}}]));
nodes.searchMode.value='source';nodes.respectRobots.checked=true;nodes.enableFTP.checked=true;nodes.seeds.value='https://example.org';
const session=new Map,local=new Map;
const ctx=vm.createContext({
 console,URL,URLSearchParams,Intl,Date,Number,String,Object,Array,Set,Map,Promise,Error,
 document:{querySelector:()=>({content:'pages'}),getElementById:id=>{assert(nodes[id],id);return nodes[id]},addEventListener(){}},
 location:{search:'',hostname:'djshellshoxxx.github.io'},window:{},
 localStorage:{getItem:k=>local.get(k)||null,setItem:(k,v)=>local.set(k,v),removeItem:k=>local.delete(k)},
 sessionStorage:{getItem:k=>session.get(k)||null,setItem:(k,v)=>session.set(k,v),removeItem:k=>session.delete(k)},
 setTimeout:()=>1,clearTimeout(){},setInterval(){},fetch:()=>{throw Error('network not mocked')}
});
vm.runInContext(fs.readFileSync('docs/app.js','utf8'),ctx,{filename:'docs/app.js'});
assert.equal(vm.runInContext('pagesMode',ctx),true);
assert.throws(()=>vm.runInContext("apiURL('/api/status')",ctx),/Connect an HTTPS/);
assert.equal(vm.runInContext("normalizedEndpoint('https://crawler.example.net/')",ctx),'https://crawler.example.net');
assert.throws(()=>vm.runInContext("normalizedEndpoint('http://insecure.example.net/')",ctx),/bare HTTPS/);
assert.throws(()=>vm.runInContext("normalizedEndpoint('https://crawler.example.net/path')",ctx),/bare HTTPS/);
assert.throws(()=>vm.runInContext("normalizedEndpoint('https://a:b@crawler.example.net')",ctx),/bare HTTPS/);
const cfg=vm.runInContext('makeConfig()',ctx);assert.equal(cfg.searchMode,'source');assert.equal(cfg.respectRobots,true);assert.equal(cfg.enableFTP,true);
const input=[{url:'https://example.org/x',matched:'=HYPERLINK("evil")',pattern:'p',before:'just',after:'after',source:'urlscan',context:'just =HYPERLINK after',foundAt:'2026-11-21',status:200},{url:'https://other.org/a',matched:'needle',pattern:'needle',source:'hn'}];
let csv=vm.runInContext('matchesCSV',ctx)(input);
assert(csv.includes('https://example.org/x'));assert(csv.includes('Matched String'));assert(csv.includes("'=HYPERLINK"),'CSV formula escaping missing');assert(csv.includes('just'));assert(csv.includes('after'));
nodes.urlFilter.value='example.org';csv=vm.runInContext('matchesCSV',ctx)(input);assert(!csv.includes('other.org'),'URL filter not applied to export');
nodes.urlFilter.value='';nodes.matchFilter.value='needle';csv=vm.runInContext('matchesCSV',ctx)(input);assert(csv.includes('other.org'));assert(!csv.includes('example.org/x'));
console.log('PASS: Pages dashboard parity, remote URL validation, source-default config, filtered contextual CSV, injection escaping');
