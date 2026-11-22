'use strict';
// No browser installation needed: verify Pages is the same dashboard and exercise its core client helpers.
const fs=require('node:fs');const assert=require('node:assert/strict');const vm=require('node:vm');
const web=fs.readFileSync('web/index.html','utf8');
const pages=fs.readFileSync('docs/index.html','utf8');
assert.equal(pages.replace('\n<meta name="freshsift-frontend" content="pages">',''),web,'Pages HTML diverges from Go dashboard');
for(const f of ['app.js','browser-engine.js','subdomains.js','dns-intel.js','style.css','favicon.svg'])assert.equal(fs.readFileSync('docs/'+f,'utf8'),fs.readFileSync('web/'+f,'utf8'),`Pages asset ${f} diverges from desktop`);
assert(pages.includes('<script src="./subdomains.js" defer></script>'),'Pages subdomain engine is not loaded');
assert(pages.includes('<script src="./dns-intel.js" defer></script>'),'Pages DNS intelligence module missing');
assert(pages.includes('<script src="./browser-engine.js" defer></script>'),'Pages browser engine is not loaded');
const go=fs.readFileSync('internal/freshsift/types.go','utf8');
const struct=go.match(/type Config struct \{([\s\S]*?)\n\}/);
assert(struct,'Go config struct missing');
// Legacy searchRawHTML is superseded by searchMode and not emitted by the dashboard.
const keys=[...struct[1].matchAll(/`json:"([^"]+)"`/g)].map(m=>m[1]).filter(x=>x!=='searchRawHTML').sort();
const ids=[...pages.matchAll(/\bid="([^"]+)"/g)].map(x=>x[1]);assert.equal(ids.length,new Set(ids).size,'duplicate page control IDs');
for(const id of ['searchMode','seeds','patterns','regex','fileTypes','excludeFileTypes','allowedTLDs','respectRobots','enableFTP','depth','budgetMode','siteMinutes','sourceQuery','searxngURL','engine-google','src-searxng','enumerateSubdomains','subdomainUseBuiltIn','subdomainUseCT','subdomainWords','subdomainLimit','subdomainFile','subdomainExport','subdomainResults','subdomainGuess','discoverDNSRecords','dnsOnly','reverseDNSPages','reverseDNSPageDepth','reverseDNSSubdomains','reverseDNSSubdomainDepth','dnsRecordResults','reverseDNSResults','dnsRecordExport','reverseDNSExport','src-urlscan','src-ct','filteredCSV','saveBtn','urlFilter','matchFilter','btnConnect','engineToken','engineURL','scopePreview','exportConfig','importConfigBtn','scanHistory'])assert(ids.includes(id),'missing feature from Pages: '+id);
const nodes=Object.fromEntries(ids.map(id=>[id,{id,value:'',checked:false,type:'text',hidden:false,textContent:'',classList:{add(){},remove(){}},replaceChildren(){},append(){}}]));
nodes.searchMode.value='source';nodes.respectRobots.checked=true;nodes.enableFTP.checked=true;nodes.seeds.value='https://example.org';nodes.enumerateSubdomains.checked=true;nodes.subdomainUseBuiltIn.checked=true;nodes.subdomainUseCT.checked=true;nodes.subdomainLimit.value='250';nodes.subdomainGuess.checked=true;nodes.discoverDNSRecords.checked=true;nodes.dnsOnly.checked=true;nodes.reverseDNSPages.checked=true;nodes.reverseDNSPageDepth.value='2';nodes.reverseDNSSubdomains.checked=true;nodes.reverseDNSSubdomainDepth.value='3';nodes.subdomainWords.value='dev, api';nodes.searxngURL.value='https://search.example.org';nodes['engine-google'].checked=true;nodes['engine-bing'].checked=true;
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
const cfg=vm.runInContext('makeConfig()',ctx);assert.deepEqual(Object.keys(cfg).sort(),keys,'Pages configuration diverges from Go engine');assert.equal(cfg.searchMode,'source');assert.equal(cfg.respectRobots,true);assert.equal(cfg.enableFTP,true);assert.equal(cfg.enumerateSubdomains,true);assert.equal(cfg.subdomainUseBuiltIn,true);assert.equal(cfg.subdomainUseCT,true);assert.equal(cfg.subdomainLimit,250);assert.equal(cfg.subdomainWords,'dev, api');assert.equal(cfg.searxngURL,'https://search.example.org');assert.deepEqual([...cfg.searchEngines],['google','bing']);assert.equal(cfg.subdomainGuess,true);assert.equal(cfg.discoverDNSRecords,true);assert.equal(cfg.dnsOnly,true);assert.equal(cfg.reverseDNSPages,true);assert.equal(cfg.reverseDNSPageDepth,2);assert.equal(cfg.reverseDNSSubdomains,true);assert.equal(cfg.reverseDNSSubdomainDepth,3);
const input=[{url:'https://example.org/x',matched:'=HYPERLINK("evil")',pattern:'p',before:'just',after:'after',source:'urlscan',context:'just =HYPERLINK after',foundAt:'2026-11-21',status:200},{url:'https://other.org/a',matched:'needle',pattern:'needle',source:'hn'}];
let csv=vm.runInContext('matchesCSV',ctx)(input);
assert(csv.includes('https://example.org/x'));assert(csv.includes('Matched String'));assert(csv.includes("'=HYPERLINK"),'CSV formula escaping missing');assert(csv.includes('just'));assert(csv.includes('after'));
nodes.urlFilter.value='example.org';csv=vm.runInContext('matchesCSV',ctx)(input);assert(!csv.includes('other.org'),'URL filter not applied to export');
nodes.urlFilter.value='';nodes.matchFilter.value='needle';csv=vm.runInContext('matchesCSV',ctx)(input);assert(csv.includes('other.org'));assert(!csv.includes('example.org/x'));
console.log('PASS: Pages dashboard parity, remote URL validation, source-default config, filtered contextual CSV, injection escaping');
