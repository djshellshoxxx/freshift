/* Public starter-page catalog shared by the Go dashboard and static GitHub Pages.
   A listed portal is a navigation seed, not a promise of bulk API/index ingestion. */
(function(root){
 'use strict';
 const specs=[
  ['fresh','Fast-moving discovery hubs','Recommended starting points with frequently changing public links.',[
   'https://news.ycombinator.com/newest','https://lobste.rs/newest','https://github.com/trending',
   'https://dev.to/latest','https://arxiv.org/list/cs/new','https://www.techmeme.com/',
   'https://www.theregister.com/','https://gist.github.com/discover'
  ]],
  ['pastes','Pastes, snippets and public Gists','Includes public listings plus hosting homepages; private/unlisted pastes cannot be discovered from a homepage.',[
   'https://pastebin.com/archive','https://gist.github.com/discover','https://rentry.co/',
   'https://paste.ee/','https://controlc.com/','https://dpaste.com/',
   'https://paste.rs/','https://snippet.host/'
  ]],
  ['archives','Web indexes and historical archives','Common Crawl, Wayback and urlscan are index portals; their full holdings require dedicated query APIs.',[
   'https://index.commoncrawl.org/','https://urlscan.io/search/',
   'https://web.archive.org/','https://archive.org/','https://archive.org/details/texts',
   'https://archive.org/details/software','https://commoncrawl.org/get-started',
   'https://www.loc.gov/collections/'
  ]],
  ['source-code','Public code forges and repository discovery','Recently published code, repository directories, tags and public project listings.',[
   'https://github.com/explore','https://github.com/topics','https://github.com/trending',
   'https://gist.github.com/discover','https://gitlab.com/explore/projects',
   'https://codeberg.org/explore/repos','https://gitea.com/explore/repos',
   'https://sourcehut.org/','https://forgejo.org/'
  ]],
  ['packages','Package registries and release directories','Package updates, release pages and current software publication indexes.',[
   'https://pypi.org/','https://pypi.org/rss/updates.xml','https://www.npmjs.com/',
   'https://crates.io/','https://rubygems.org/','https://pkg.go.dev/',
   'https://hub.docker.com/','https://repology.org/','https://www.nuget.org/packages',
   'https://packagist.org/'
  ]],
  ['security','Security advisories and vulnerability records','Official advisory databases, disclosure listings, CVE and open vulnerability feeds.',[
   'https://www.cisa.gov/news-events/cybersecurity-advisories','https://nvd.nist.gov/vuln/search',
   'https://github.com/advisories','https://osv.dev/list','https://www.cve.org/',
   'https://www.zerodayinitiative.com/advisories/published/',
   'https://packetstormsecurity.com/files/','https://www.exploit-db.com/',
   'https://www.openwall.com/lists/oss-security/'
  ]],
  ['security-news','Security reporting and research blogs','Frequently updated incident coverage, investigations and defensive research.',[
   'https://www.bleepingcomputer.com/news/security/','https://thehackernews.com/',
   'https://www.securityweek.com/','https://www.darkreading.com/',
   'https://www.theregister.com/security/','https://krebsonsecurity.com/',
   'https://www.schneier.com/blog/','https://unit42.paloaltonetworks.com/',
   'https://blog.talosintelligence.com/','https://securelist.com/'
  ]],
  ['research','Research papers and scientific preprints','New academic papers, publication indexes and public scholarly collections.',[
   'https://arxiv.org/list/cs/new','https://arxiv.org/list/math/new',
   'https://arxiv.org/list/physics/new','https://arxiv.org/list/stat/new',
   'https://www.biorxiv.org/','https://www.medrxiv.org/',
   'https://pubmed.ncbi.nlm.nih.gov/','https://paperswithcode.com/latest',
   'https://www.nature.com/nature/articles'
  ]],
  ['ai-ml','AI, ML models and dataset activity','Model releases, newly modified datasets, benchmarks and AI research.',[
   'https://huggingface.co/models?sort=modified','https://huggingface.co/datasets?sort=modified',
   'https://arxiv.org/list/cs.AI/recent','https://arxiv.org/list/cs.LG/recent',
   'https://paperswithcode.com/latest','https://openreview.net/',
   'https://www.kaggle.com/datasets','https://ollama.com/library',
   'https://github.com/topics/large-language-models'
  ]],
  ['open-data','Open data and public datasets','Government data catalogs, open knowledge bases, geographic edits and datasets.',[
   'https://catalog.data.gov/dataset/','https://open.canada.ca/data/en/dataset',
   'https://data.europa.eu/en','https://www.data.gov.uk/search',
   'https://data.un.org/','https://www.wikidata.org/wiki/Special:RecentChanges',
   'https://www.openstreetmap.org/changesets','https://ourworldindata.org/',
   'https://huggingface.co/datasets'
  ]],
  ['government','Government publications and public notices','Publicly published policy changes, official documents and regulatory filings.',[
   'https://www.federalregister.gov/documents/current','https://www.govinfo.gov/',
   'https://www.sec.gov/edgar/search/','https://www.canada.ca/en/news.html',
   'https://gazette.gc.ca/','https://www.legislation.gov.uk/new',
   'https://www.europa.eu/newsroom/','https://www.loc.gov/collections/'
  ]],
  ['news','General news and current reporting','Broad publication coverage with links into frequently updated articles.',[
   'https://apnews.com/','https://www.reuters.com/world/',
   'https://www.theguardian.com/technology','https://www.bbc.com/news/technology',
   'https://www.aljazeera.com/news/','https://www.npr.org/sections/technology/',
   'https://www.wired.com/','https://arstechnica.com/',
   'https://www.engadget.com/','https://techcrunch.com/'
  ]],
  ['community','Public technology forums and discussions','Recently posted questions, discussions and community-submitted links.',[
   'https://news.ycombinator.com/newest','https://lobste.rs/newest',
   'https://stackoverflow.com/questions?tab=Newest','https://meta.discourse.org/latest',
   'https://discuss.python.org/latest','https://discuss.huggingface.co/latest',
   'https://www.reddit.com/r/programming/new/','https://www.reddit.com/r/netsec/new/',
   'https://dev.to/latest'
  ]],
  ['blogs','Blogs, independent writing and publishing','Independent posts, creator feeds and frequently updated technical blogs.',[
   'https://wordpress.com/read','https://medium.com/tag/technology/latest',
   'https://medium.com/tag/programming/latest','https://hashnode.com/',
   'https://substack.com/','https://blog.mozilla.org/',
   'https://blog.cloudflare.com/','https://mastodon.social/explore',
   'https://www.smashingmagazine.com/articles/'
  ]],
  ['downloads','Public file libraries and software mirrors','Public archives and downloadable directory listings; larger files stay within scan limits.',[
   'https://ftp.gnu.org/gnu/','https://mirrors.edge.kernel.org/pub/',
   'https://cdimage.debian.org/','https://releases.ubuntu.com/',
   'https://archive.org/details/software','https://www.gutenberg.org/ebooks/',
   'https://www.gutenberg.org/browse/recent/','https://www.loc.gov/collections/',
   'https://download.gnome.org/'
  ]],
  ['standards','Technical standards and changelogs','Specifications, drafts, standards and implementation release notes.',[
   'https://www.rfc-editor.org/rfc/','https://datatracker.ietf.org/doc/',
   'https://www.w3.org/TR/','https://www.kernel.org/',
   'https://go.dev/doc/devel/release','https://docs.python.org/3/whatsnew/',
   'https://www.mozilla.org/en-US/firefox/releases/',
   'https://www.postgresql.org/docs/release/',
   'https://docs.docker.com/engine/release-notes/'
  ]],
  ['business','Business, filings and economic releases','Public market filings, financial news, macroeconomic and trade releases.',[
   'https://www.sec.gov/edgar/search/','https://www.federalreserve.gov/newsevents/pressreleases.htm',
   'https://www.bls.gov/news.release/','https://www.bankofcanada.ca/press/',
   'https://www.statcan.gc.ca/en/subjects-start','https://www.imf.org/en/News',
   'https://www.worldbank.org/en/news','https://www.oecd.org/en/about/news.html',
   'https://www.investing.com/news/'
  ]],
  ['nonprofit-civic','Civic technology, NGOs and public-interest research','Nonprofit publications and projects with publicly searchable changelogs.',[
   'https://www.eff.org/deeplinks','https://citizenlab.ca/',
   'https://www.accessnow.org/press-release/','https://www.mozilla.org/en-US/blog/',
   'https://www.wikimedia.org/','https://www.openstreetmap.org/',
   'https://www.internetarchive.org/','https://www.owasp.org/',
   'https://www.linuxfoundation.org/blog/'
  ]]
 ];
 const groups=Object.freeze(specs.map(([id,label,description,urls])=>Object.freeze({
  id,label,description,urls:Object.freeze(urls)
 })));
 const byId=new Map(groups.map(g=>[g.id,g]));
 const defaultIds=Object.freeze(['fresh']);
 function selectionForSettings(saved) {
  if(saved===null||saved===undefined)return [...defaultIds];
  if(!Array.isArray(saved.starterLists))return [];
  return [...new Set(saved.starterLists.filter(id=>typeof id==='string'&&byId.has(id)))];
 }
 function normalizeKey(raw){
  const input=String(raw||'').trim();
  try{
   const u=new URL(/^[a-z][a-z0-9+.-]*:\/\//i.test(input)?input:'https://'+input);
   if(['https:','http:','ftp:'].includes(u.protocol)){
    u.hash='';
    return u.href;
   }
  }catch{}
  return input.toLowerCase();
 }
 function combine(manualText, selectedIds=[], excludedByGroup={}){
  const ids=[...new Set((Array.isArray(selectedIds)?selectedIds:[]).filter(x=>byId.has(x)))];
  const urls=[],seen=new Set();
  let duplicateCount=0,manualCount=0,groupURLCount=0;
  const add=(raw,manual)=>{
   const value=String(raw||'').trim();
   if(!value||value.startsWith('#'))return;
   const key=normalizeKey(value);
   if(seen.has(key)){duplicateCount++;return}
   seen.add(key);urls.push(value);
   if(manual)manualCount++;else groupURLCount++;
  };
  for(const raw of String(manualText||'').split(/[\r\n,]+/))add(raw,true);
  for(const id of ids){
   const exclusions=new Set(Array.isArray(excludedByGroup?.[id])?excludedByGroup[id]:[]);
   for(const u of byId.get(id).urls)if(!exclusions.has(u))add(u,false);
  }
  return {urls,manualCount,groupURLCount,groupCount:ids.length,duplicateCount};
 }
 function filterGroups(query){
  const q=String(query||'').trim().toLowerCase();
  return !q?[...groups]:groups.filter(g=>[g.label,g.description,...g.urls].join(' ').toLowerCase().includes(q));
 }
 const api=Object.freeze({groups,defaultIds,combine,normalizeKey,filterGroups,selectionForSettings});
 root.FreshSiftStarters=api;
 if(typeof module!=='undefined'&&module.exports)module.exports=api;
})(typeof globalThis!=='undefined'?globalThis:this);
