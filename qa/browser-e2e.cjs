'use strict';
const assert=require('node:assert/strict');
const fs=require('node:fs');
const {chromium}=require('playwright');
const base=process.env.TEST_BASE_URL||'http://127.0.0.1:9157';
async function prepare(page){
 await page.goto(base+'/docs/index.html',{waitUntil:'domcontentloaded'});
 await page.locator('#browserModeNote').waitFor({state:'visible'});
 for(const id of ['urlscan','hn','brave','rss','sitemap','ct']){
  const selector=page.locator('#src-'+id);if(await selector.isChecked())await selector.uncheck();
 }
 await page.locator('#searchMode').selectOption('source');
 await page.locator('#patterns').fill('LOCAL_BROWSER_MATCH');
 await page.locator('#seeds').fill('');
 await page.locator('#respectRobots').evaluate(el=>{el.checked=false;el.dispatchEvent(new Event('change',{bubbles:true}))});
}

/* Live-site regression: exercise the deployed crawler, not the localhost fixture. */
async function hostedCrawlTests(browser){
 const raw=process.env.HOSTED_BASE_URL||'https://djshellshoxxx.github.io/freshift/';
 const hosted=raw.endsWith('/')?raw:raw+'/';
 const page=await browser.newPage({acceptDownloads:true});
 const jsErrors=[];
 page.on('pageerror',e=>jsErrors.push(e.message));
 const checks=[];
 function passed(name){checks.push(name);console.log('PASS HOSTED: '+name)}
 async function configure(mode,pattern,seed){
  await page.goto(hosted,{waitUntil:'domcontentloaded',timeout:45000});
  await page.locator('#browserModeNote').waitFor({state:'visible',timeout:20000});
  await page.evaluate(()=>{
   for(const id of ['src-urlscan','src-hn','src-brave','src-rss','src-sitemap','src-ct','src-searxng','respectRobots','enumerateSubdomains','discoverDNSRecords','reverseDNSPages','reverseDNSSubdomains','dnsOnly']){
    const el=document.getElementById(id);
    if(el){el.checked=false;el.dispatchEvent(new Event('change',{bubbles:true}))}
   }
  });
  await page.locator('#searchMode').selectOption('source');
  await page.locator('#patterns').fill(pattern);
  await page.locator('#seeds').fill(seed);
  await page.locator('#searchMode').selectOption(mode);
 }
 async function runAndWait(){
  await page.locator('#startBtn').click();
  await page.waitForFunction(()=>document.getElementById('stateText').textContent.trim()==='COMPLETED',null,{timeout:30000});
  return page.locator('#matches').innerText();
 }
 try{
  await configure('source','freshsift-frontend',hosted+'index.html');
  assert((await page.title()).includes('FreshSift'),'hosted page title missing');
  assert(await page.evaluate(()=>!!window.FreshSiftBrowserEngine),'hosted browser engine unavailable');
  const assets=await page.evaluate(async()=>Promise.all(['app.js','browser-engine.js','subdomains.js','dns-intel.js','style.css','favicon.svg'].map(async f=>{
   const r=await fetch('./'+f,{cache:'no-store'});return {file:f,status:r.status};
  })));
  assert(assets.every(x=>x.status===200),'missing hosted assets: '+JSON.stringify(assets));
  passed('Pages loads, browser engine initializes, all six assets respond HTTP 200');
  let matches=await runAndWait();
  assert(matches.includes('freshsift-frontend'),'source crawl did not find HTML meta marker');
  passed('same-origin HTML source crawling');

  await configure('source','const configIDs',hosted+'index.html');
  matches=await runAndWait();
  assert(matches.includes('app.js'),'source crawler did not scan linked app.js');
  passed('linked JavaScript source searched at depth zero');

  await configure('visible','freshsift-frontend',hosted+'index.html');
  matches=await runAndWait();
  assert.equal(Number(await page.locator('#matchCount').innerText()),0,'visible mode exposed HTML meta/source content');
  await configure('visible','CIRCUIT DRIFT LABS',hosted+'index.html');
  matches=await runAndWait();
  assert(matches.includes('CIRCUIT DRIFT LABS'),'visible-text crawl missed rendered text');
  passed('visible-only excludes markup and matches page text');

  await configure('source','HOSTED_LOCAL_BINARY_20261123','');
  await page.locator('#localFiles').setInputFiles({name:'hosted-crawler.db',mimeType:'application/octet-stream',buffer:Buffer.from('xHOSTED_LOCAL_BINARY_20261123z')});
  matches=await runAndWait();
  assert(matches.includes('HOSTED_LOCAL_BINARY_20261123'),'binary local-file scan did not match');
  await page.locator('#exportFormat').selectOption('json');
  const [download]=await Promise.all([page.waitForEvent('download'),page.locator('#saveBtn').click()]);
  const rows=JSON.parse(fs.readFileSync(await download.path(),'utf8'));
  assert(rows.some(x=>x.matched==='HOSTED_LOCAL_BINARY_20261123'&&x.url.includes('local-file://')),'local scan result missing from JSON export');
  passed('local binary file search and JSON export');

  await configure('files','',hosted+'favicon.svg');
  await page.locator('#fileTypes').fill('.svg');
  matches=await runAndWait();
  assert(matches.includes('favicon.svg'),'filetype mode failed to verify hosted SVG');
  passed('filetype-only discovery of hosted SVG');

  assert.deepEqual(jsErrors,[],'uncaught browser errors: '+jsErrors.join('; '));
  passed('no uncaught JavaScript page errors');
  console.log('PASS: '+checks.length+' live hosted GitHub Pages checks at '+hosted);
 }finally{await page.close()}
}

(async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  const page=await browser.newPage({acceptDownloads:true});
  await prepare(page);
  await page.locator('#localFiles').setInputFiles({name:'local.db',
   mimeType:'application/octet-stream',buffer:Buffer.from('xLOCAL_BROWSER_MATCHz')});
  await page.locator('#startBtn').click();
  await page.waitForFunction(()=>Number(document.getElementById('matchCount').textContent)>0);
  assert((await page.locator('#matches').innerText()).includes('LOCAL_BROWSER_MATCH'));
  await page.locator('#exportFormat').selectOption('json');
  const [download]=await Promise.all([page.waitForEvent('download'),page.locator('#saveBtn').click()]);
  const raw=fs.readFileSync(await download.path(),'utf8');
  assert(JSON.parse(raw).some(x=>x.url.includes('local-file://input/local.db')),'missing local export');
  await page.reload({waitUntil:'domcontentloaded'});
  for(const id of ['urlscan','hn','brave','rss','sitemap','ct']){
   const selector=page.locator('#src-'+id);if(await selector.isChecked())await selector.uncheck();
  }
  await page.locator('#searchMode').selectOption('source');
  await page.locator('#patterns').fill('HTTP_BROWSER_MATCH');
  await page.locator('#seeds').fill(base+'/qa/fixtures/browser-source.html');
  await page.locator('#allowPrivate').evaluate(el=>{el.checked=true;el.dispatchEvent(new Event('change',{bubbles:true}))});
  await page.locator('#respectRobots').evaluate(el=>{el.checked=false;el.dispatchEvent(new Event('change',{bubbles:true}))});
  await page.locator('#startBtn').click();
  await page.waitForFunction(()=>Number(document.getElementById('matchCount').textContent)>0);
  assert((await page.locator('#matches').innerText()).includes('HTTP_BROWSER_MATCH'));
  console.log('PASS: Chromium browser-only local binary file, same-origin HTTP source and JSON export');
  await hostedCrawlTests(browser);
 }finally{await browser.close()}
})().catch(e=>{console.error(e);process.exitCode=1});
