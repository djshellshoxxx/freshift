'use strict';
const assert = require('node:assert/strict');
const {BrowserEngine} = require('../web/browser-engine.js');
const base='https://demo.example.org';
const resources=new Map([
 [base+'/', {status:200,type:'text/html',body:'<html><style>.x{color:red}</style><script src="/app.js"></script><link rel="stylesheet" href="/site.css"><a href="/more">More</a><a href="/dump.DB?download=1">DB</a><img src="/pic.jpg">visible keyword</html>'}],
 [base+'/site.css',{status:200,type:'text/css',body:'/* KEYWORD_CSS */ @import "/more.css";'}],
 [base+'/app.js',{status:200,type:'text/javascript',body:'const phrase = "KEYWORD_JS";'}],
 [base+'/more.css',{status:200,type:'text/css',body:'/* KEYWORD_MORECSS */'}],
 [base+'/more',{status:200,type:'text/html',body:'Next page KEYWORD_NEXT'}],
 [base+'/dump.DB?download=1',{status:200,type:'application/octet-stream',body:'binary KEYWORD_DB bytes'}],
 [base+'/pic.jpg',{status:200,type:'image/jpeg',body:'jpegbytes'}],
 [base+'/ignored.ASX',{status:200,type:'text/html',body:'KEYWORD_ASX'}],
 [base+'/robots.txt',{status:200,type:'text/plain',body:'User-agent: *\nDisallow: /secret\n'}],
 [base+'/secret',{status:200,type:'text/html',body:'KEYWORD_SECRET'}],
 ]);
const attempts=[];
async function mockTransport(url,opts={}){
 const name=new URL(url).href;attempts.push({url:name,method:opts.method||'GET'});
 const v=resources.get(name);if(!v)throw new Error('404 fixture '+name);
 return {url:name,status:v.status,type:v.type,body:opts.method==='HEAD'?'':v.body};
}
function cfg(extra={}){return {...{
 seeds:base+'/',searchMode:'source',patterns:'KEYWORD',depth:0,concurrency:2,maxPages:50,
 maxPagesPerHost:50,allowedTLDs:'.org',enableFTP:true,respectRobots:false,
 excludeFileTypes:'.asx',status2xxOnly:true
 },...extra}}
async function scan(config,fetchResource=mockTransport){const engine=new BrowserEngine({fetchResource});await engine.start(config);return {state:engine.snapshot(),matches:engine.allResults(),engine}}
(async()=>{
 assert.equal(typeof BrowserEngine,'function');
 attempts.length=0;
 let {state,matches}=await scan(cfg());
 assert.equal(state.stats.status,'completed',JSON.stringify(state.stats));
 for(const token of ['KEYWORD_CSS','KEYWORD_JS','KEYWORD_DB','KEYWORD_MORECSS']) assert(matches.some(m=>m.context.includes(token)),token);
 assert(!matches.some(m=>m.matched==='KEYWORD_NEXT'),'depth=0 should not navigate');
 assert(!attempts.some(a=>a.url.includes('.ASX')),'excluded extension fetched');
 assert(matches.some(m=>m.url.includes('.DB')),'binary .db match absent');
 ({matches}=await scan(cfg({searchMode:'visible',patterns:'keyword',depth:0})));
 assert(matches.some(m=>m.matched==='keyword'),'visible page text not searched');
 assert(!matches.some(m=>/KEYWORD_(CSS|JS|DB)/.test(m.matched)),'visible scanned linked sources');
 ({matches}=await scan(cfg({searchMode:'files',patterns:'',fileTypes:'.jpg,.db',depth:0})));
 assert(matches.some(m=>m.url.includes('/pic.jpg')),'jpg not reported');
 assert(matches.some(m=>m.url.includes('/dump.DB')),'db not reported');
 ({matches}=await scan(cfg({seeds:base+'/secret',patterns:'KEYWORD',respectRobots:true})));
 assert(!matches.length,'robots denied path');
 ({matches}=await scan(cfg({seeds:base+'/secret',patterns:'KEYWORD',respectRobots:false})));
 assert(matches.some(m=>m.context.includes('KEYWORD_SECRET')),'robots disabled path missing');
 ({state}=await scan(cfg({seeds:'https://unavailable.example.org',patterns:'WORD'}),async()=>{throw new TypeError('Failed to fetch (CORS)')}));
 assert(state.stats.errors>0,'CORS failures must surface rather than silently report success');
 assert(state.logs.some(x=>/CORS|Failed to fetch/i.test(x)),'CORS error must be logged');
 const localEngine=new BrowserEngine({fetchResource:async()=>{throw Error('Local files must not use network')}});
 const localBytes=new TextEncoder().encode('bytes KEYWORD_LOCAL more');
 const localFile={name:'local.db',size:localBytes.byteLength,arrayBuffer:async()=>localBytes.buffer};
 await localEngine.start(cfg({seeds:'',providers:[],searchMode:'source'}),[localFile]);
 assert(localEngine.allResults().some(x=>x.context.includes('KEYWORD_LOCAL')),'local file source scan missing');
 const localFilesMode=new BrowserEngine({fetchResource:async()=>{throw Error('Local file verification must not fetch')}});
 await localFilesMode.start(cfg({seeds:'',providers:[],searchMode:'files',patterns:'',fileTypes:'.db'}),[localFile]);
 assert(localFilesMode.allResults().some(x=>x.kind==='file'),'local file type detection missing');
 const cancelEngine=new BrowserEngine({fetchResource:mockTransport});cancelEngine.stop();assert.equal(cancelEngine.snapshot().stats.status,'idle');
 console.log('PASS: browser source assets, visible, filetypes, exclusions, robots, CORS reporting');
})().catch(e=>{console.error(e);process.exitCode=1});
