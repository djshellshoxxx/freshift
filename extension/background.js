'use strict';
/* Browser-local HTTP(S) fetch proxy. Never forwards cookies or credentials.
   Host permissions are explicitly granted when the extension is installed. */
const SOURCE='https://djshellshoxxx.github.io/freshift/';
const MAX=8*1024*1024;
function privateHost(host) {
 host=host.toLowerCase().replace(/^\[|\]$/g,'');
 return host==='localhost'||host.endsWith('.localhost')||host.endsWith('.local')||host==='::1'||
  /^127\./.test(host)||/^10\./.test(host)||/^192\.168\./.test(host)||
  /^169\.254\./.test(host)||/^172\.(1[6-9]|2\d|3[01])\./.test(host)||
  /^0\./.test(host)||/^fc|^fd|^fe80:/i.test(host);
}
async function get(request) {
 const u=new URL(request.url);
 if (!['http:','https:'].includes(u.protocol) || u.username || u.password)
   throw Error('Only public HTTP(S) URLs without embedded credentials are supported');
 if (privateHost(u.hostname) && request.allowPrivate!==true)
   throw Error('Private/localhost targets require the private-IP checkbox');
 if (!['GET','HEAD'].includes(request.method||'GET'))
   throw Error('Unsupported method');
 const headers={};
 if(request.headers && request.headers.Range==='bytes=0-0')headers.Range='bytes=0-0';
 const controller=new AbortController();
 const timeout=Math.max(2000,Math.min(90000,(Number(request.timeoutSeconds)||12)*1000));
 const timer=setTimeout(()=>controller.abort(),timeout);
 try {
  // No redirect following: prevents a public URL redirecting into a private target.
  // User can provide the destination URL directly.
  const response=await fetch(u.href,{method:request.method||'GET',
    headers,credentials:'omit',redirect:'error',cache:'no-store',signal:controller.signal});
  const type=response.headers.get('content-type')||'';
  if(request.method==='HEAD')return {url:response.url,status:response.status,type,body:'',size:0};
  if(Number(response.headers.get('content-length'))>MAX)throw Error('Response larger than 8 MiB');
  const chunks=[];let size=0;
  if(response.body?.getReader){
   const reader=response.body.getReader();
   while(true){const {value,done}=await reader.read();if(done)break;
    size+=value.byteLength;if(size>MAX){await reader.cancel();throw Error('Response larger than 8 MiB')}
    chunks.push(value);
   }
  }else{
   const data=new Uint8Array(await response.arrayBuffer());
   size=data.byteLength;if(size>MAX)throw Error('Response larger than 8 MiB');chunks.push(data);
  }
  const bytes=new Uint8Array(size);let pos=0;for(const part of chunks){bytes.set(part,pos);pos+=part.byteLength}
  const encoding=/(?:html|xml|json|javascript|css|text)/i.test(type)?'utf-8':'latin1';
  return {url:response.url,status:response.status,type,body:new TextDecoder(encoding).decode(bytes),size};
 }finally{clearTimeout(timer)}
}
chrome.runtime.onMessage.addListener((request,sender,reply) => {
 const page=sender.tab?.url||sender.url||'';
 if(!page.startsWith(SOURCE)||request?.operation!=='freshsift-fetch')return false;
 get(request).then(result=>reply(result)).catch(error=>reply({error:String(error.message||error)}));
 return true;
});