'use strict';
const assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const manifest=JSON.parse(fs.readFileSync('extension/manifest.json','utf8'));
assert.equal(manifest.manifest_version,3);
assert(manifest.content_scripts.some(x=>x.matches.includes('https://djshellshoxxx.github.io/freshift/*')));
assert(manifest.host_permissions.includes('https://*/*'));
let listener=null;
const requests=[];
const ctx={chrome:{runtime:{onMessage:{addListener(fn){listener=fn}}}},URL,
 TextDecoder,AbortController,setTimeout,clearTimeout,
 async fetch(url,opt){requests.push({url,opt});return new Response('test content',{headers:{'content-type':'text/plain'},status:200})},
 Response
};
vm.runInNewContext(fs.readFileSync('extension/background.js','utf8'),ctx,{filename:'extension/background.js'});
assert.equal(typeof listener,'function');
const origin='https://djshellshoxxx.github.io/freshift/';
async function call(request,from=origin){
 return new Promise(resolve=>{const accepted=listener({operation:'freshsift-fetch',method:'GET',...request},{tab:{url:from}},resolve);if(!accepted)resolve({error:'REJECTED'})});
}
(async()=>{
 const ok=await call({url:'https://example.org/file.db'});
 assert.equal(ok.body,'test content');assert.equal(ok.status,200);
 const denied=await call({url:'http://127.0.0.1/private'});assert(/Private\/localhost/.test(denied.error));
 const ipv6=await call({url:'https://[::1]/admin'});assert(/Private\/localhost/.test(ipv6.error));
 const ftp=await call({url:'ftp://example.org/pub'});assert(/Only public HTTP/.test(ftp.error));
 const hidden=await call({url:'https://example.org' },'https://other.github.io/freshift/');
 assert.equal(hidden.error,'REJECTED');
 assert.equal(requests.length,1,'blocked destinations must not be fetched');
 console.log('PASS: extension manifest restrictions, fetch bridge, denied private/FTP origins');
})().catch(e=>{console.error(e);process.exitCode=1});
