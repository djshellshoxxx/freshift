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
  await page.locator('#respectRobots').uncheck();
  await page.locator('#startBtn').click();
  await page.waitForFunction(()=>Number(document.getElementById('matchCount').textContent)>0);
  assert((await page.locator('#matches').innerText()).includes('HTTP_BROWSER_MATCH'));
  console.log('PASS: Chromium browser-only local binary file, same-origin HTTP source and JSON export');
 }finally{await browser.close()}
})().catch(e=>{console.error(e);process.exitCode=1});
