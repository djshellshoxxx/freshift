# FreshSift v0.5.0 — feature matrix

This is the implemented feature set in the source tree, not a guarantee of external feed availability.

| Feature | Status | Relevant code |
|---|---|---|
| Optional DNS record lookup and DNS-only search | Implemented; no webpage fetch in DNS-only mode | `dns_intel.go`, `web/dns-intel.js` |
| Brute-force subdomain guessing vs historical CT-only discovery | Separate checkboxes, configurable wordlist and result cap | `subdomains.go`, `web/subdomains.js` |
| Forward-confirmed PTR on crawled page host IPs | Implemented; cannot enumerate all shared-IP virtual hosts | `dns_intel.go`, `web/dns-intel.js` |
| Forward-confirmed PTR for subdomains on IPs not shared with seed hostname | Implemented, scoped and capped | `dns_intel.go`, `web/browser-engine.js` |
| Independent link-depth limits for page PTR / subdomain PTR | 0–8 for each, subject to global page budgets | `engine.go`, `web/browser-engine.js` |
| Separate DNS and PTR result panels with filtered CSV | Implemented | `web/app.js`, `main.go` |
| Pasted URL/IP and FTP seeds, TXT/CSV import, file drop | Implemented | `types.go`, `web/app.js` |
| Raw HTML, CSS, JS, images and binary `.db` byte searching by default | Implemented within 8 MiB/body | `engine.go`, `match.go`, `fetch.go` |
| Separate visible-text-only option | Implemented (no JS execution) | `engine.go`, `web/index.html` |
| Filetype-only discovery with no string needed | Implemented | `types.go`, `engine.go` |
| HTTP HEAD / range fallback and FTP SIZE verification | Implemented | `fetch.go`, `ftp.go` |
| Source/asset extraction from HTML; CSS imports/URLs; static JS imports | Implemented (best-effort static discovery) | `match.go` |
| Anonymous FTP seed and linked FTP directory/file traversal | Implemented (plain, passive, read-only) | `ftp.go` |
| Filetype exclusions e.g. `.asx`, case-insensitive, query-independent | Implemented | `types.go`, `engine.go` |
| Include only selected TLDs e.g. `.com, .co.uk` | Implemented | `types.go`, `engine.go` |
| robots.txt on/off option | Implemented (HTTP(S) only) | `robots.go`, `web/index.html` |
| Allow/deny domains and URL substring filtering | Implemented | `engine.go` |
| Exact strings, RE2 regex, optional case sensitivity | Implemented | `match.go` |
| Recursion, path/directory limits, per-site timer | Implemented | `types.go`, `engine.go` |
| Concurrent workers, per-host pacing, pause/resume/stop | Implemented | `engine.go` |
| Live matched URL and excerpt / verified file panes | Implemented | `web/app.js` |
| Resource/second statistics, matched counts, errors, progress | Implemented | `engine.go`, `web/app.js` |
| CSV / JSON / JSONL exports, spreadsheet-safe CSV | Implemented | `main.go` |
| URLScan, HN, RSS, sitemap, CT, optional Brave providers | Implemented; sources may limit queries | `discovery.go` |
| Google, Bing, Brave, DDG, Mojeek, Yandex advanced links | Implemented as links, not ingestion | `web/app.js` |
| InternetDB public IP enrichment | Implemented for one public IP at a time | `main.go` |
| Local presets, copy URLs, desktop/mobile CSS | Implemented | `web/app.js`, `web/style.css` |
| Public/private IP destination validation and DNS pinning | Implemented | `fetch.go`, `ftp.go` |
| Go race tests, mock FTP/HTTP tests, API smoke test, JS contract test | Implemented, see QA | `*_test.go`, `qa/` |
| Real-time proof of **first indexed** dates | **Not available** | Observation times are source-specific |
| Browser rendering / executing dynamic JavaScript | **Not implemented** | Static URL extraction only |
| FTPS, SFTP, authenticated FTP | **Not implemented** | Only anonymous `ftp://` |
| Persistence across app restarts | **Not implemented** | Export before closing |
| Automated Google/Bing search result ingestion | **Not implemented** | Advanced search URLs only |

| Pages uses exact same UI / controls as desktop | Implemented via `docs/` asset mirror; connected Go engine required | `docs/app.js`, `docs/index.html` |
| Remote token authentication + exact HTTPS origin CORS | Implemented | `main.go`, `main_test.go` |
| Filtered CSV with matched strings, URLs, before/after context | Implemented (full export on demand) | `web/app.js` |
| Scope preview / configuration import/export / scan-summary history | Implemented locally in browser | `web/app.js` |
| Circuit Drift Labs dark / teal / orange branding | Implemented in both UI variants | `web/style.css` |
| Pure GitHub Pages browser independently crawling arbitrary URLs and FTP | **Not possible** (needs live Go engine) | HTML-only hosting limitation |
See [README](../README.md) and [QA](QA.md).

## Client-side availability matrix

The legacy matrix above describes the **Go engine**. This table describes current experimental JavaScript behavior introduced on the client-side implementation branch. Full operational parity without the Go engine is **not** established.

| Feature | Pages alone | Pages + locally installed extension | Go engine |
|---|---|---|---|
| Exact string search in local files | Yes, up to 8 MiB/file | Yes | Via URL fetch, not local upload |
| Source HTML/CSS/JS and accessible binary URLs | CORS accessible | HTTP(S) with host permissions | HTTP(S), FTP |
| Visible-style text | Static HTML approximation | Static HTML approximation | Static HTML approximation |
| Extension-only file URL discovery | Candidates verifiable if CORS permits | Candidates verifiable by HTTP HEAD/GET | HTTP HEAD/GET, FTP SIZE |
| Linked resources at depth zero | Static HTML/CSS/JS | Static HTML/CSS/JS | Static HTML/CSS/JS |
| Regex | Restricted JS RegExp, not RE2 | Restricted JS RegExp, not RE2 | Go RE2 |
| Robots checking | CORS accessible; fail closed | Via extension, fail closed | Via Go |
| Recursive crawl and filtering | Bounded, best effort | Bounded, best effort | Full configured budgets |
| FTP directory traversal and reads | No | No | Anonymous plain FTP |
| External discovery feeds | HN, urlscan, sitemap, RSS best effort | HN, urlscan, sitemap, RSS best effort | HN, urlscan, sitemap, RSS, CT, optional Brave |
| JavaScript-generated links | No | No | No |
| Session status, live results, pause/resume, CSV/JSON/JSONL | Yes | Yes | Yes |
| Automatic proof of first indexing time | No | No | No |

**Not yet equivalent:** Browser mode does not yet enforce every Go per-site pacing/time budget and provider contract, and the extension cannot substitute for FTP. The Pages/Go frontend controls remain matched; that does not mean scan execution semantics are identical.

Browser-specific tests are in qa/browser-engine.test.cjs and qa/extension-contract.test.cjs. These are mock HTTP fixture tests, not end-to-end tests against arbitrary public sites. The deployment requires an enabled GitHub Pages site in repository settings.

### Continuous discovery mode

Both interfaces expose `perpetual` and `rescanMinutes` and cycle through enabled discovery feeds and a bounded URL revisit queue until stopped. The Go process continues when the dashboard is closed and can restore its opt-in persisted configuration at process startup; standalone Pages only runs during an open browser tab (or controls a connected always-on Go process). Match deduplication and the existing per-cycle scan budgets are applied. Automatic discovery of every indexed page cannot be guaranteed by public discovery sources.

### Curated starting websites

Both Go and Pages dashboards use the same `starter-lists.js` catalog: 18 optional category checkboxes (including pastes and public Gists, archived web indexes, code, security, public data, research, news and more), with recommended fast-moving sources selected for new clients. Effective seeds merge selected groups with manual input; duplicate URLs are removed and users can filter, preview, copy or export the result. Choices persist in local settings and presets. Old saved configurations are preserved without injecting defaults; Go's API accepts the fully resolved `seeds` string.
