'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const catalog = require('../web/starter-lists.js');

assert(catalog.groups.length >= 12, 'need extensive selectable categories');
assert(catalog.groups.reduce((n,g)=>n+g.urls.length,0) >= 100, 'need at least 100 starter URLs across categories');
assert.deepEqual(catalog.defaultIds, ['fresh'], 'only the fast-moving starter group should be selected by default');
const ids = new Set();
for (const group of catalog.groups) {
  assert(/^[a-z][a-z0-9-]+$/.test(group.id), 'safe group ID required');
  assert(!ids.has(group.id), 'duplicate group ID');
  ids.add(group.id);
  assert(group.label && group.description, 'group metadata required');
  assert(group.urls.length >= 5, 'groups must have useful source coverage');
  assert.equal(new Set(group.urls).size, group.urls.length, 'duplicate URLs within group');
  for (const raw of group.urls) {
    const u = new URL(raw);
    assert.equal(u.protocol, 'https:', 'starter URLs must use HTTPS');
    assert(u.hostname.includes('.'), 'real domain required');
    assert(!u.username && !u.password, 'catalog URLs must not contain credentials');
  }
}
assert(ids.has('pastes') && ids.has('archives') && ids.has('security') && ids.has('research'), 'earlier-conversation sources must be represented');
assert(catalog.filterGroups('Gist').some(g=>g.id==='pastes'), 'search must match sources in groups');
const selected = catalog.combine('https://news.ycombinator.com/newest\nexample.org\nftp://ftp.gnu.org/gnu/\nhttps://example.org/', ['fresh','pastes','unknown']);
assert(selected.urls.includes('https://news.ycombinator.com/newest'));
assert(selected.urls.includes('example.org'));
assert(selected.urls.includes('ftp://ftp.gnu.org/gnu/'));
assert.equal(selected.urls.filter(u=>u==='https://news.ycombinator.com/newest').length,1, 'manual+group duplicates must be eliminated');
assert(selected.duplicateCount>=1,'report duplicate source URLs');
assert.equal(selected.groupCount,2,'unknown group should be ignored');
assert(selected.urls.length>15,'multiple groups must be combined');
assert.deepEqual(catalog.selectionForSettings(null),['fresh'],'fresh installs use curated defaults');
assert.deepEqual(catalog.selectionForSettings({seeds:'https://example.org'}),[], 'legacy saved settings must not silently inject new seeds');
assert.deepEqual(catalog.selectionForSettings({starterLists:['pastes','unknown','pastes']}),['pastes'],'saved selection sanitized and deduplicated');

const html=fs.readFileSync('web/index.html','utf8');
const pages=fs.readFileSync('docs/index.html','utf8');
const app=fs.readFileSync('web/app.js','utf8');
assert(html.includes('id="starterCatalog"'),'starter categories control missing');
assert(html.includes('id="starterSearch"'),'searchable starter catalog missing');
assert(html.includes('id="starterPreview"'),'effective URL preview missing');
assert(html.includes('id="starterReset"'),'recommended default reset missing');
assert(html.includes('id="starterExport"'),'starter export missing');
assert(html.includes('src="./starter-lists.js"'),'catalog module must load before app');
assert(pages.includes('src="./starter-lists.js"'),'Pages catalog module missing');
assert(app.includes('starterLists:'),'starter selection must be included in persisted settings');
assert(app.includes('FreshSiftStarters'),'UI must consume shared catalog');
assert(app.includes('combine('),'scan configuration must use deduplicated effective seed URLs');
console.log('PASS: curated catalog breadth, deduplication, multi-selection, default/legacy behavior, UI controls and parity');
