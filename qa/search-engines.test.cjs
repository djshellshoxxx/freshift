'use strict';
const assert=require('node:assert/strict');
const {BrowserEngine}=require('../web/browser-engine.js');
(async()=>{
 const requests=[],service='https://search.example.net';
 const fetchResource=async url=>{
  const u=new URL(url);requests.push(u);
  if(u.origin===service){
   assert.equal(u.pathname,'/search');
   assert.equal(u.searchParams.get('engines'),'google,bing');
   assert.equal(u.searchParams.get('q'),'paste');
   assert.equal(u.searchParams.get('format'),'json');
   return {url,status:200,type:'application/json',body:JSON.stringify({results:u.searchParams.get('pageno')==='1'?
    [{url:'https://target.example.org/post',engines:['google']},{url:'https://target.example.org/post',engines:['bing']},{url:'javascript:bad',engines:['bing']}]:[]})};
  }
  if(url==='https://target.example.org/post')return {url,status:200,type:'text/html',body:'<p>SEARCH_MATCH</p>'};
  throw Error('unexpected request '+url);
 };
 const crawler=new BrowserEngine({fetchResource});
 await crawler.start({seeds:'',providers:['searxng'],searxngURL:service,searchEngines:['google','bing'],sourceQuery:'paste',
  searchMode:'source',patterns:'SEARCH_MATCH',discoveryLimit:50,allowedTLDs:'.org',respectRobots:false,depth:0,concurrency:1});
 const matches=crawler.allResults();
 assert.equal(crawler.snapshot().stats.status,'completed');
 assert.equal(matches.length,1);
 assert.equal(matches[0].source,'searxng/google');
 assert.equal(matches[0].url,'https://target.example.org/post');
 assert.equal(requests.filter(u=>u.origin===service).length,2);
 const unsafe=new BrowserEngine({fetchResource:async()=>{throw Error('unsafe endpoint must not be requested')}});
 await unsafe.start({seeds:'',providers:['searxng'],searxngURL:'http://127.0.0.1:8888',searchEngines:['google'],
  sourceQuery:'paste',searchMode:'source',patterns:'SEARCH_MATCH',respectRobots:false});
 assert(unsafe.snapshot().stats.errors>0,'bad endpoint must be reported');
 assert(unsafe.snapshot().logs.some(line=>line.includes('public HTTPS')),'invalid SearXNG endpoint not rejected');
 console.log('PASS: browser SearXNG selection, deduplication, provenance, endpoint validation');
})().catch(e=>{console.error(e);process.exitCode=1});
