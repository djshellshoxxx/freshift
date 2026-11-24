'use strict';
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
const catalog=require('../web/starter-lists.js');
const html=fs.readFileSync('web/index.html','utf8');
const ids=[...html.matchAll(/\bid="([^"]+)"/g)].map(x=>x[1]);
const nodes=Object.fromEntries(ids.map(id=>[id,{type:'text',value:'',checked:false,textContent:'',hidden:false}]));
for(const g of catalog.groups)nodes['starter-'+g.id]={type:'checkbox',value:'on',checked:g.id==='fresh'};
nodes.seeds.value='https://github.com/trending\nexample.org';
nodes.patterns.value='needle';
nodes.searchMode.value='source';
nodes.maxPages.value='200';
const local=new Map();
const ctx=vm.createContext({
 URL,URLSearchParams,Intl,Date,Number,String,Array,Map,Set,Promise,Error,console,
 window:{FreshSiftStarters:catalog},
 document:{querySelector:()=>({content:'pages'}),getElementById:id=>nodes[id]||null,addEventListener(){}},
 location:{search:'',hostname:'djshellshoxxx.github.io'},
 localStorage:{getItem:key=>local.get(key)||null,setItem:(key,val)=>local.set(key,val),removeItem:key=>local.delete(key)},
 sessionStorage:{getItem:()=>null},
 setTimeout:()=>1,clearTimeout(){},setInterval(){},
 fetch:()=>{throw Error('network not permitted in UI tests')}
});
vm.runInContext(fs.readFileSync('web/app.js','utf8'),ctx,{filename:'web/app.js'});
let cfg=vm.runInContext('makeConfig()',ctx);
assert(cfg.seeds.includes('https://news.ycombinator.com/newest'), 'default selected starter group missing from scan config');
assert.equal(cfg.seeds.split('\n').filter(x=>x==='https://github.com/trending').length,1,'manual and default URL duplicated');
assert(cfg.seeds.includes('example.org'),'manual hostname discarded');
let saved=vm.runInContext('snapshotForm()',ctx);
assert.deepEqual([...saved.starterLists],['fresh'],'starter selection missing from stored settings');
vm.runInContext('countTargets()',ctx);
assert(nodes.targetCount.textContent.includes('TARGETS'),'effective target counter not updated');
assert(nodes.starterPreview.textContent.includes('example.org'),'preview must show the actual scan URLs');
vm.runInContext('loadForm({seeds:"https://legacy.example.org",patterns:"needle"})',ctx);
cfg=vm.runInContext('makeConfig()',ctx);
assert.equal(cfg.seeds,'https://legacy.example.org','legacy config unexpectedly gained default sites');
assert.equal(nodes['starter-fresh'].checked,false,'legacy saved settings must clear fresh defaults');
vm.runInContext('loadForm({seeds:"https://manual.example.org",patterns:"needle",starterLists:["archives","pastes"]})',ctx);
cfg=vm.runInContext('makeConfig()',ctx);
assert(cfg.seeds.includes('https://index.commoncrawl.org/'),'restored archive group missing from scan');
assert(cfg.seeds.includes('https://pastebin.com/archive'),'restored paste category missing from scan');
assert(cfg.seeds.includes('https://manual.example.org'),'manual seed missing after restore');
assert.equal(nodes['starter-pastes'].checked,true);
assert.equal(nodes['starter-archives'].checked,true);
assert.equal(nodes['starter-fresh'].checked,false);
saved=vm.runInContext('snapshotForm()',ctx);
assert.deepEqual([...saved.starterLists],['pastes','archives'],'selection must be serialized in stable catalog order');
assert(!Object.hasOwn(cfg,'starterLists'),'Go API must receive only existing fields, not UI-only category names');
vm.runInContext('starterSiteInputs.set("pastes",[{url:"https://pastebin.com/archive",input:{checked:false,disabled:false}}])',ctx);
cfg=vm.runInContext('makeConfig()',ctx);
assert(!cfg.seeds.includes('https://pastebin.com/archive'),'unchecked URL included in scan despite group enabled');
assert(cfg.seeds.includes('https://gist.github.com/discover'),'unchecked URL blocked whole category');
saved=vm.runInContext('snapshotForm()',ctx);
assert.deepEqual([...saved.starterExclusions.pastes],['https://pastebin.com/archive'],'individual URL choice not saved');
vm.runInContext('loadForm({seeds:"https://manual.example.org",starterLists:["pastes"],starterExclusions:{pastes:["https://pastebin.com/archive"]}})',ctx);
assert.equal(vm.runInContext('makeConfig()',ctx).seeds.includes('https://pastebin.com/archive'),false,'restored exclusion not applied');
console.log('PASS: actual dashboard VM multi-selection, deduplicated seeds, settings snapshots, legacy compatibility and preview');
