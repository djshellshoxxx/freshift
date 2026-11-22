/* FreshSift DNS intelligence: record search and forward-verified PTR. */
(function(root){
'use strict';
const TYPES={A:1,AAAA:28,CNAME:5,MX:15,NS:2,TXT:16,SOA:6,SRV:33,CAA:257,PTR:12};
const DNS_TYPES=['A','AAAA','CNAME','MX','NS','TXT','SOA','SRV','CAA'];
function validHost(name){
 return typeof name==='string'&&name.length>0&&name.length<=253&&
 name.split('.').every(s=>s.length>0&&s.length<=63&&/^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$/.test(s));
}
function isPublicIP(raw){
 const ip=String(raw||'').toLowerCase();
 const part=ip.split('.');
 if(part.length===4){
  if(part.some(p=>p===''||!/^\d{1,3}$/.test(p)||+p>255))return false;
  const a=+part[0],b=+part[1],c=+part[2];
  if(a===0||a===10||a===127||a>=224||a===169&&b===254||
    a===172&&b>=16&&b<=31||a===192&&(b===168||b===0&&c===0)||
    a===100&&b>=64&&b<=127||a===198&&(b===18||b===19||b===51&&c===100)||
    a===192&&b===0&&c===2||a===203&&b===0&&c===113)return false;
  return true;
 }
 if(!ip.includes(':')||!/^[0-9a-f:]+$/.test(ip))return false;
 if(ip==='::1'||ip==='::'||/^(?:fc|fd|fe8|fe9|fea|feb|ff)/.test(ip)||ip.startsWith('2001:db8:'))return false;
 try{new URL('https://['+ip+']/');return true}catch{return false}
}
function reverseName(ip){
 if(!isPublicIP(ip))return '';
 if(ip.includes('.'))return ip.split('.').reverse().join('.')+'.in-addr.arpa';
 const halves=ip.toLowerCase().split('::');
 if(halves.length>2)return '';
 const left=halves[0]?halves[0].split(':'):[];
 const right=halves[1]?halves[1].split(':'):[];
 const missing=8-left.length-right.length;
 if((halves.length===1&&missing!==0)||(halves.length===2&&missing<1))return '';
 const groups=[...left,...Array(missing).fill('0'),...right];
 if(groups.length!==8||groups.some(g=>!/^[0-9a-f]{1,4}$/.test(g)))return '';
 return groups.map(g=>g.padStart(4,'0')).join('').split('').reverse().join('.')+'.ip6.arpa';
}
async function query(name,type,fetchResource){
 const endpoint='https://dns.google/resolve?name='+encodeURIComponent(name)+'&type='+type;
 const r=await fetchResource(endpoint);
 if(r.status!==200)throw Error('DNS-over-HTTPS HTTP '+r.status);
 const data=JSON.parse(r.body);
 if(data.Status===3)return [];
 if(data.Status!==0)throw Error('DNS response '+data.Status+' for '+name+' '+type);
 return (Array.isArray(data.Answer)?data.Answer:[]).filter(x=>x.type===TYPES[type])
  .map(x=>String(x.data||'').trim()).filter(x=>x.length>0&&x.length<=2048).slice(0,200);
}
async function lookupRecords(domain,fetchResource){
 domain=String(domain||'').trim().toLowerCase().replace(/\.$/,'');
 if(!validHost(domain)||!domain.includes('.'))return [];
 const out=[];
 const settled=await Promise.allSettled(DNS_TYPES.map(type=>query(domain,type,fetchResource)));
 for(let i=0;i<settled.length;i++){
  if(settled[i].status!=='fulfilled')continue;
  for(const value of settled[i].value)out.push({domain,type:DNS_TYPES[i],value});
 }
 return out.slice(0,250).sort((a,b)=>(a.type+a.value).localeCompare(b.type+b.value));
}
async function lookupAddresses(domain,fetchResource){
 const name=String(domain||'').toLowerCase().replace(/\.$/,'');
 if(!validHost(name))return [];
 const answers=await Promise.allSettled(['A','AAAA'].map(t=>query(name,t,fetchResource)));
 const out=[];
 for(const r of answers){if(r.status==='fulfilled')for(const ip of r.value){if(isPublicIP(ip)&&!out.includes(ip))out.push(ip)}}
 return out;
}
async function verifiedPTR(ip,fetchResource){
 if(!isPublicIP(ip))return [];
 const reverse=reverseName(ip);
 if(!reverse)return [];
 let ptr;
 try{ptr=await query(reverse,'PTR',fetchResource)}catch{return []}
 const results=[],seen=new Set();
 for(const value of ptr.slice(0,16)){
  const name=value.toLowerCase().replace(/\.$/,'');
  if(!validHost(name)||!name.includes('.')||seen.has(name))continue;
  seen.add(name);
  const forward=await lookupAddresses(name,fetchResource);
  if(forward.includes(ip))results.push(name);
 }
 return results.sort();
}
async function differentSubdomainIPs(root,hostname,fetchResource){
 const [base,sub]=await Promise.all([lookupAddresses(root,fetchResource),lookupAddresses(hostname,fetchResource)]);
 if(!base.length)return [];
 return sub.filter(ip=>!base.includes(ip));
}
const api={TYPES,DNS_TYPES,query,lookupRecords,lookupAddresses,verifiedPTR,differentSubdomainIPs,reverseName,isPublicIP,validHost};
root.FreshSiftDNSIntel=api;
if(typeof module!=='undefined'&&module.exports)module.exports=api;
})(typeof window!=='undefined'?window:globalThis);
