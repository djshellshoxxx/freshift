# FreshSift v0.3.0 — feature matrix

This is the implemented feature set in the source tree, not a guarantee of external feed availability.

| Feature | Status | Relevant code |
|---|---|---|
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
