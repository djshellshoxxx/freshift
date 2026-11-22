'use strict';
const assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const origin='https://djshellshoxxx.github.io',events={message:[]};
const page={
 location:{origin,pathname:'/freshift/'},
 addEventListener(type,callback){(events[type]??=[]).push(callback)},
 postMessage(data,target){assert.equal(target,origin);
  for(const callback of [...(events.message||[])])callback({source:page,origin,data});
 }
};
const ctx={window:page,location:page.location,document:{},setTimeout,clearTimeout,
 chrome:{runtime:{lastError:null,sendMessage(payload,cb){
  assert.equal(payload.operation,'freshsift-fetch');
  assert.equal(payload.url,'https://example.org/no-cors');
  cb({url:payload.url,status:200,type:'text/html',body:'EXTENSION_MATCH'});
 }}}
};
vm.runInNewContext(fs.readFileSync('extension/content-script.js','utf8'),ctx,{filename:'extension/content-script.js'});
vm.runInNewContext(fs.readFileSync('docs/browser-engine.js','utf8'),ctx,{filename:'docs/browser-engine.js'});
(async()=>{
 assert.equal(page.FreshSiftExtension.ready,true,'extension readiness handshake failed');
 const out=await page.FreshSiftExtension.fetch('https://example.org/no-cors');
 assert.equal(out.body,'EXTENSION_MATCH');
 console.log('PASS: Pages <-> content-script extension fetch transport handshake');
})().catch(e=>{console.error(e);process.exitCode=1});
