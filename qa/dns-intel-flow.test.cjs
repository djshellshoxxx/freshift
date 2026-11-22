'use strict';
const assert=require('node:assert/strict');
const {BrowserEngine}=require('../web/browser-engine.js');
const requests=[];
const urls={
 'https://example.org/':'<html>MAGIC <a href="/origin-child">origin child</a></html>',
 'https://ptr.example.net/':'<html>MAGIC <a href="/level1">first</a></html>',
 'https://ptr.example.net/level1':'<html>MAGIC <a href="/level2">second</a></html>',
 'https://ptr.example.net/level2':'<html>MAGIC</html>',
 'https://edge.example.org/':'<html>MAGIC</html>',
 'https://edgeptr.example.net/':'<html>MAGIC <a href="/level1">first</a></html>',
 'https://edgeptr.example.net/level1':'<html>MAGIC <a href="/level2">second</a></html>',
 'https://edgeptr.example.net/level2':'<html>MAGIC <a href="/level3">third</a></html>',
 'https://edgeptr.example.net/level3':'<html>MAGIC</html>'
};
const data={
 'example.org':{A:['8.8.8.8'],MX:['10 mail.example.org.'],TXT:['"v=spf1 include:mail.example.org"']},
 'ptr.example.net':{A:['8.8.8.8']},
 'edge.example.org':{A:['1.1.1.1']},
 'edgeptr.example.net':{A:['1.1.1.1']}
};
async function fetchResource(raw){
 const u=new URL(raw);requests.push(u.href);
 if(u.hostname==='dns.google'){
  const host=u.searchParams.get('name'),type=u.searchParams.get('type');
  const answers=type==='PTR'?
   ({'8.8.8.8.in-addr.arpa':['ptr.example.net.'],'1.1.1.1.in-addr.arpa':['edgeptr.example.net.']}[host]||[]):
   (data[host]?.[type]||[]);
  const num={A:1,AAAA:28,CNAME:5,MX:15,NS:2,TXT:16,SOA:6,SRV:33,CAA:257,PTR:12}[type];
  return {url:raw,status:200,type:'application/json',body:JSON.stringify({Status:answers.length?0:3,
   Answer:answers.map(x=>({type:num,data:x}))})};
 }
 if(!Object.hasOwn(urls,u.href))throw Error('Unmocked HTTP target: '+u.href);
 return {url:u.href,status:200,type:'text/html',body:urls[u.href]};
}
const base={seeds:'https://example.org',patterns:'MAGIC',searchMode:'source',depth:0,maxPages:40,
 maxPagesPerHost:20,concurrency:2,perHostConcurrency:2,respectRobots:false,providers:[],
 status2xxOnly:true,subdomainUseCT:false};
(async()=>{
 const only=new BrowserEngine({fetchResource});
 requests.length=0;
 await only.start({...base,patterns:'mail',discoverDNSRecords:true,dnsOnly:true});
 assert.equal(only.snapshot().stats.pagesScanned,0,'DNS-only scanned an HTTP page');
 assert(only.allDNSRecords().some(x=>x.type==='MX'&&x.value.includes('mail.example.org')),'MX inventory missing');
 assert(only.allResults().some(x=>x.source==='dns-record'&&x.matched.toLowerCase()==='mail'),'DNS values not content-searched');
 assert(!requests.some(url=>new URL(url).hostname==='example.org'),'DNS-only performed a web request');

 const pages=new BrowserEngine({fetchResource});
 requests.length=0;
 await pages.start({...base,reverseDNSPages:true,reverseDNSPageDepth:1});
 assert(pages.allReverseDNS().some(x=>x.hostname==='ptr.example.net'&&x.origin==='page'&&x.verified),'page PTR missing');
 assert(requests.includes('https://ptr.example.net/level1'),'reverse page depth 1 not honored');
 assert(!requests.includes('https://ptr.example.net/level2'),'reverse page crawl exceeded independent depth');
 assert(!requests.includes('https://example.org/origin-child'),'regular crawl depth 0 exceeded');

 const sub=new BrowserEngine({fetchResource});
 requests.length=0;
 await sub.start({...base,enumerateSubdomains:true,subdomainGuess:true,subdomainUseBuiltIn:false,
  subdomainWords:'edge',subdomainLimit:10,reverseDNSSubdomains:true,reverseDNSSubdomainDepth:2});
 assert(sub.allSubdomains().some(x=>x.hostname==='edge.example.org'&&x.verified),'subdomain guessing failed');
 assert(sub.allReverseDNS().some(x=>x.hostname==='edgeptr.example.net'&&x.origin==='subdomain'),'changed-IP subdomain PTR missing');
 assert(requests.includes('https://edgeptr.example.net/level2'),'reverse subdomain depth 2 not honored');
 assert(!requests.includes('https://edgeptr.example.net/level3'),'reverse subdomain crawl exceeded its depth');
 console.log('PASS: DNS-only, DNS content match, page PTR depth, different-IP subdomain PTR depth');
})().catch(e=>{console.error(e);process.exitCode=1});
