/* Subdomain discovery for GitHub Pages: scoped CT observations and DNS-over-HTTPS wordlist checks. */
(function(root){
'use strict';
const BUILTIN=["www","mail","webmail","smtp","imap","pop","pop3","mx","mx1","mx2","ns","ns1","ns2","ns3","dns","dns1","dns2","ftp","sftp","ssh","vpn","remote","gateway","gw","router","firewall","proxy","reverse-proxy","api","api1","api2","api-dev","api-staging","api-test","v1","v2","graphql","rest","auth","login","sso","oauth","accounts","account","id","identity","admin","administrator","portal","dashboard","panel","control","console","management","manage","app","apps","application","web","mobile","m","beta","alpha","dev","development","test","testing","qa","uat","staging","stage","preprod","preview","prod","production","internal","intranet","private","secure","public","edge","cdn","static","assets","media","images","img","files","file","download","downloads","upload","uploads","storage","s3","bucket","backup","backups","archive","archives","db","database","mysql","postgres","postgresql","mongo","mongodb","redis","cache","memcached","elastic","elasticsearch","search","solr","index","logs","logging","log","metrics","monitor","monitoring","status","health","healthcheck","grafana","prometheus","kibana","jenkins","ci","cd","git","github","gitlab","gitea","registry","docker","k8s","kubernetes","cluster","node","worker","build","deploy","releases","release","docs","documentation","wiki","help","support","desk","ticket","tickets","issues","blog","news","forum","community","shop","store","cart","checkout","pay","payment","payments","billing","invoice","crm","erp","hr","staff","employee","employees","partners","partner","customer","customers","client","clients","office","home","old","legacy","new","www1","www2","www3","web1","web2","server","server1","server2","host","host1","host2","origin","origin1","origin2","lb","loadbalancer","www-dev","dev-api","staging-api","test-api","test1","test2","demo","sandbox","lab","labs","research","testbed","mail1","mail2","autodiscover","autoconfig","exchange","owa","outlook","sip","lync","teams","meeting","meet","video","voice","voip","pbx","chat","irc","irc1","irc2","matrix","ircweb","xmpp","jira","confluence","bitbucket","redmine","nextcloud","cloud","drive","data","analytics","report","reports","stats","statistics","events","track","tracking","maps","map","geo","location","calendar","schedule","updates","update","patch","patches","mirror","mirrors","repo","repos","repository","packages","pkg","npm","pypi","maven","artifacts","artifactory","nexus","wordpress","wp","cms","cpanel","whm","plesk","directadmin","phpmyadmin","pma","adminer","api-gateway","hooks","webhook","webhooks","callback","callbacks","notification","notifications","push","socket","ws","wss","stream","streaming","live","realtime","relay","relay1","relay2","connect","connectivity","ping","speedtest","ntp","time","directory","ldap","ad","dc","dc1","dc2","rdp","terminal","bastion","jump","jumpbox","citrix","vdi","desktop","workstation","printer","print","scan","scanner","cam","camera","cameras","iot","device","devices","sensor","sensors","power","ups","nas","san","backup1","backup2"];

const validLabel=x=>x.length>0&&x.length<=63&&/^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$/.test(x);
const validName=x=>x.length<=253&&x.split('.').every(validLabel);
function parseWords(raw,builtin=true){
 if(String(raw||'').length>200000)throw Error('Subdomain wordlist exceeds 200 KB');
 const seen=new Set(),out=[];
 const add=source=>{
  for(const line of String(source||'').split(/\r?\n/)){
   if(!line.trim()||line.trim().startsWith('#'))continue;
   for(const part of line.split(/[,;\s]+/)){
    const word=part.trim().toLowerCase().replace(/^[.\"']+|[.\"']+$/g,'');
    if(!word||seen.has(word)||!validName(word))continue;
    seen.add(word);out.push(word);
    if(out.length>5000)throw Error('Subdomain wordlist exceeds 5,000 distinct labels');
   }
  }
 };
 if(builtin)add(BUILTIN.join('\n'));
 add(raw);
 return out;
}
function validChild(name,domain){
 return name.length>domain.length+1&&name.endsWith('.'+domain)&&validName(name);
}
function parseCT(raw,domain,limit=250){
 let rows;try{rows=typeof raw==='string'?JSON.parse(raw):raw}catch{return []}
 if(!Array.isArray(rows)||limit<1)return [];
 const seen=new Set(),out=[];
 for(const row of rows){
  for(const nameValue of String(row.name_value||'').split(/\r?\n/)){
   const name=nameValue.trim().toLowerCase().replace(/^\*\./,'').replace(/\.$/,'');
   if(!validChild(name,domain)||seen.has(name))continue;
   seen.add(name);out.push(name);
   if(out.length>=limit)return out;
  }
 }
 return out;
}
const sameAddresses=(a,b)=>a.length>0&&b.length>0&&a.length===b.length&&
 a.slice().sort().every((x,i)=>x===b.slice().sort()[i]);
function rootOf(raw){
 try{
  const u=new URL(/^[a-z]+:\/\//i.test(raw)?raw:'https://'+raw);
  const name=u.hostname.toLowerCase().replace(/\.$/,'');
  if(!validName(name)||!name.includes('.')||/^\d+(?:\.\d+){3}$/.test(name))return '';
  return name;
 }catch{return ''}
}
async function dnsRecords(name,fetchResource){
 async function query(type){
  const endpoint='https://dns.google/resolve?name='+encodeURIComponent(name)+'&type='+type;
  const r=await fetchResource(endpoint);
  if(r.status!==200)throw Error('DNS-over-HTTPS HTTP '+r.status);
  const data=JSON.parse(r.body);
  if(data.Status===3)return {absent:true,addresses:[]};
  if(data.Status!==0)return {absent:false,addresses:[]};
  const recordType=type==='A'?1:28;
  const ips=(Array.isArray(data.Answer)?data.Answer:[]).filter(x=>x.type===recordType)
   .map(x=>String(x.data||'')).filter(x=>type==='A'?/^\d+(?:\.\d+){3}$/.test(x):/^[0-9a-f:]+$/i.test(x));
  return {absent:false,addresses:[...new Set(ips)]};
 }
 const a=await query('A');
 if(a.absent||a.addresses.length)return a.addresses;
 return (await query('AAAA')).addresses;
}
async function discover({config,fetchResource,report=()=>{},log=()=>{},signal}){
 if(!config.enumerateSubdomains)return [];
 const limit=Math.max(1,Math.min(2000,Number(config.subdomainLimit)||250));
 const words=parseWords(config.subdomainWords,!!config.subdomainUseBuiltIn);
 if(!words.length&&!config.subdomainUseCT)throw Error('Enable built-in subdomain words, import a wordlist, or enable CT');
 const roots=[...new Set(String(config.seeds||'').split(/[\n\r,]+/).map(x=>x.trim())
  .filter(x=>x&&!x.startsWith('#')).map(rootOf).filter(Boolean))].slice(0,5);
 if(!roots.length)throw Error('Subdomain enumeration requires a seed domain');
 const found=[],seen=new Set();
 const add=(hostname,source,addresses=[])=>{
  if(seen.has(hostname)||found.length>=limit)return;
  seen.add(hostname);
  const row={hostname,source,verified:source==='dns',addresses};
  found.push(row);report(row);
 };
 for(const domain of roots){
  if(signal?.aborted||found.length>=limit)break;
  let countDNS=0,failures=0;
  if(words.length){
   let available=true,wildcard=[];
   try{
    const nonce='freshsift-'+Date.now().toString(36)+'-'+Math.floor(Math.random()*100000).toString(36);
    const first=await dnsRecords(nonce+'-a.'+domain,fetchResource);
    const second=await dnsRecords(nonce+'-b.'+domain,fetchResource);
    if(sameAddresses(first,second))wildcard=first;
   }catch(e){
    available=false;log('Subdomain DNS-over-HTTPS unavailable for '+domain+': '+e.message);
   }
   if(available){
    let index=0;
    const worker=async()=>{
     while(!signal?.aborted&&index<words.length&&found.length<limit){
      const word=words[index++],hostname=word+'.'+domain;
      if(!validChild(hostname,domain))continue;
      try{
       const addresses=await dnsRecords(hostname,fetchResource);
       if(!addresses.length||sameAddresses(addresses,wildcard))continue;
       const size=found.length;
       add(hostname,'dns',addresses);
       if(found.length!==size)countDNS++;
      }catch(e){failures++}
     }
    };
    await Promise.all(Array.from({length:Math.min(8,words.length)},()=>worker()));
   }
  }
  log('Subdomain DNS '+domain+': '+countDNS+' verified names'+(failures?' ('+failures+' failed lookups)':''));
  if(config.subdomainUseCT&&!signal?.aborted&&found.length<limit){
   try{
    const endpoint='https://crt.sh/?q='+encodeURIComponent('%.'+domain)+'&output=json';
    const r=await fetchResource(endpoint);
    if(r.status!==200)throw Error('CT HTTP '+r.status);
    const before=found.length;
    for(const name of parseCT(r.body,domain,limit)){
     if(found.length>=limit)break;
     add(name,'ct');
    }
    log('Subdomain CT '+domain+': '+(found.length-before)+' historical names (not DNS-verified)');
   }catch(e){log('Subdomain CT '+domain+': '+e.message+'; DNS results preserved')}
  }
 }
 return found.slice().sort((a,b)=>a.hostname.localeCompare(b.hostname));
}
const api={BUILTIN,parseWords,parseCT,sameAddresses,rootOf,dnsRecords,discover};
root.FreshSiftSubdomains=api;
if(typeof module!=='undefined'&&module.exports)module.exports=api;
})(typeof window!=='undefined'?window:globalThis);
