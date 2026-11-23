'use strict';
const assert = require('node:assert/strict');
const {BrowserEngine}=require('../web/browser-engine.js');

(async()=>{
 let fetched=0;
 const engine=new BrowserEngine({fetchResource:async url=>{
  if(url!=='https://news.example.org/')throw Error('unexpected URL: '+url);
  fetched++;
  return {url,status:200,type:'text/plain',body:'tracked needle'};
 }});
 engine.cycleDelayMs=25; // deterministic tests, production interval remains minutes
 const run=engine.start({seeds:'https://news.example.org/',patterns:'needle',searchMode:'source',
  providers:[],perpetual:true,rescanMinutes:1,depth:0,maxPages:1,respectRobots:false});
 const deadline=Date.now()+3000;
 while((engine.snapshot().stats.cycles<2||fetched<2)&&Date.now()<deadline)
  await new Promise(resolve=>setTimeout(resolve,10));
 assert(engine.snapshot().stats.cycles>=2,'not rescanning after the first round');
 assert(fetched>=2,'did not revisit the seed URL');
 assert.equal(engine.snapshot().stats.matches,1,'identical match duplicated across rounds');
 assert.notEqual(engine.snapshot().stats.status,'completed','perpetual mode stopped itself');
 engine.stop();
 await Promise.race([run,new Promise((_,reject)=>setTimeout(()=>reject(Error('stop did not wake interval')),2000))]);
 assert.equal(engine.snapshot().stats.status,'cancelled');
 const oneShot=new BrowserEngine({fetchResource:async url=>({url,status:200,type:'text/plain',body:'needle'})});
 await oneShot.start({seeds:'https://news.example.org/',patterns:'needle',perpetual:false,providers:[],maxPages:1,respectRobots:false});
 assert.equal(oneShot.snapshot().stats.status,'completed','nonperpetual scan behavior regressed');
 console.log('PASS: perpetual browser scan, rescan, dedupe, cancellation and one-shot');
})().catch(e=>{console.error(e);process.exitCode=1});
