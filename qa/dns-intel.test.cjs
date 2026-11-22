'use strict';
const assert=require('node:assert/strict');
const dns=require('../web/dns-intel.js');
const observed=[];
const hostRecords={
 'example.org':{A:['8.8.8.8'],MX:['10 mail.example.org.'],TXT:['"v=spf1 include:mail.example.org ~all"'],NS:['ns1.example.org.']},
 'site.example.org':{A:['8.8.8.8']},
 'ptr.example.net':{A:['8.8.8.8']},
 'edge.example.org':{A:['1.1.1.1']},
 'edge-ptr.example.net':{A:['1.1.1.1']}
};
async function mockedFetch(url){
 const u=new URL(url);
 assert.equal(u.hostname,'dns.google');
 const n=u.searchParams.get('name'),t=u.searchParams.get('type');
 observed.push(n+' '+t);
 let arr;
 if(t==='PTR'&&n==='8.8.8.8.in-addr.arpa')arr=['ptr.example.net.'];
 else if(t==='PTR'&&n==='1.1.1.1.in-addr.arpa')arr=['edge-ptr.example.net.'];
 else arr=hostRecords[n]?.[t]||[];
 const num={A:1,AAAA:28,MX:15,NS:2,CNAME:5,TXT:16,SOA:6,SRV:33,CAA:257,PTR:12}[t];
 return {url,status:200,type:'application/json',body:JSON.stringify({Status:arr.length?0:3,Answer:arr.map(data=>({type:num,data}))})};
}
(async()=>{
 const records=await dns.lookupRecords('example.org',mockedFetch);
 assert(records.some(r=>r.type==='MX'&&r.value.includes('mail.example.org')),'MX record absent');
 assert(records.some(r=>r.type==='TXT'&&r.value.includes('v=spf1')),'TXT record absent');
 assert(records.some(r=>r.type==='A'&&r.value==='8.8.8.8'),'A record absent');
 assert(!records.some(r=>r.domain==='unrelated.org'));
 const ptr=await dns.verifiedPTR('8.8.8.8',mockedFetch);
 assert.deepEqual(ptr,['ptr.example.net'],'forward-confirmed PTR missing');
 const different=await dns.differentSubdomainIPs('example.org','edge.example.org',mockedFetch);
 assert.deepEqual(different,['1.1.1.1'],'different-IP detection failed');
 const same=await dns.differentSubdomainIPs('example.org','site.example.org',mockedFetch);
 assert.deepEqual(same,[],'same-IP subdomain should be excluded');
 assert.equal(dns.isPublicIP('127.0.0.1'),false);
 assert.equal(dns.isPublicIP('8.8.8.8'),true);
 assert(observed.includes('8.8.8.8.in-addr.arpa PTR'),'missing reverse IPv4 lookup');
 console.log('PASS: DNS inventory, reverse PTR forward validation, IP comparisons and restricted hosts');
})().catch(e=>{console.error(e);process.exitCode=1});
