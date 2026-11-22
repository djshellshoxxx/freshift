'use strict';
// Dependency-free UI contract regression checks. Run with: node qa/ui-contract.test.cjs
const fs = require('node:fs');
const vm = require('node:vm');
const assert = require('node:assert/strict');
const html=fs.readFileSync('web/index.html','utf8');
const js=fs.readFileSync('web/app.js','utf8');
const ids=[...html.matchAll(/\bid="([^"]+)"/g)].map(x=>x[1]);
assert.equal(new Set(ids).size,ids.length,'duplicate HTML IDs');
const nodes={};
for(const id of ids){nodes[id]={id,type:id.startsWith('src-')||id.startsWith('engine-')?'checkbox':'text',value:'',checked:false,hidden:false,textContent:'',disabled:false};}
for(const id of ['regex','caseSensitive','respectRobots','enableFTP','allowPrivate','followExternal','status2xxOnly','enumerateSubdomains','subdomainUseBuiltIn','subdomainUseCT','subdomainGuess','discoverDNSRecords','dnsOnly','reverseDNSPages','reverseDNSSubdomains']){nodes[id].type='checkbox'}
for(const id of ['respectRobots','enableFTP','status2xxOnly'])nodes[id].checked=true;
nodes['engine-google'].checked=true;nodes['engine-bing'].checked=true;nodes.searxngURL.value='https://search.example.org';nodes.subdomainGuess.checked=true;nodes.subdomainUseBuiltIn.checked=true;nodes.reverseDNSPageDepth.value='1';nodes.reverseDNSSubdomainDepth.value='1';nodes.subdomainUseCT.checked=true;nodes.subdomainLimit.value='250';nodes.searchMode.value='source';nodes.recentHours.value='24';nodes.depth.value='0';nodes.maxPathSegments.value='5';nodes.maxPages.value='1000';nodes.concurrency.value='24';nodes.fileTypes.value='.jpg';nodes.seeds.value='http://127.0.0.1:9000/';
const ctx=vm.createContext({
 console,URL,URLSearchParams,Intl,Date,Number,String,Object,Array,Set,Map,Error,Promise,
 document:{getElementById:(id)=>{if(!nodes[id])throw new Error('Unknown DOM ID '+id);return nodes[id]},addEventListener(){}},
 location:{search:''},window:{},localStorage:{getItem:()=>null,setItem(){}},
 setTimeout:()=>1,clearTimeout(){},setInterval(){},fetch:()=>{throw new Error('network requests not allowed')}
});
vm.runInContext(js,ctx,{filename:'web/app.js'});
for(const id of vm.runInContext('configIDs',ctx))assert(ids.includes(id),'missing config element: '+id);
let cfg=vm.runInContext('makeConfig()',ctx);
assert.equal(cfg.searchMode,'source');assert.equal(cfg.searxngURL,'https://search.example.org');assert.deepEqual([...cfg.searchEngines],['google','bing']);assert.equal(cfg.onlyHTML,false);assert.equal(cfg.subdomainUseBuiltIn,true);assert.equal(cfg.subdomainGuess,true);assert.equal(cfg.reverseDNSPageDepth,1);assert.equal(cfg.reverseDNSSubdomainDepth,1);assert.equal(cfg.subdomainUseCT,true);assert.equal(cfg.subdomainLimit,250);assert.equal(cfg.respectRobots,true);assert.equal(cfg.enableFTP,true);
vm.runInContext('syncSearchMode()',ctx);
assert.equal(nodes.stringSettings.hidden,false);assert.equal(nodes.fileTypeSettings.hidden,true);
nodes.searchMode.value='files';vm.runInContext('syncSearchMode()',ctx);
cfg=vm.runInContext('makeConfig()',ctx);
assert.equal(cfg.searchMode,'files');assert.equal(cfg.fileTypes,'.jpg');
assert.equal(nodes.stringSettings.hidden,true);assert.equal(nodes.fileTypeSettings.hidden,false);
assert.equal(nodes.matchesLabel.textContent,'FILES FOUND');
nodes.searchMode.value='visible';vm.runInContext('syncSearchMode()',ctx);
assert.equal(nodes.stringSettings.hidden,false);assert.equal(nodes.fileTypeSettings.hidden,true);
assert.equal(nodes.matchesLabel.textContent,'TOTAL MATCHES');
assert.equal(typeof cfg.excludeFileTypes,'string');assert.equal(typeof cfg.allowedTLDs,'string');
console.log('PASS: UI contract (unique IDs, all controls, default source, visible/files toggles, new settings)');
