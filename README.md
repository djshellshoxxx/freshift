# FreshSift v0.3.0

**Local-first, single-executable Go crawler for finding content and file URLs in recently observed web resources.** It has a browser interface, but crawling and searches happen in Go on your own computer.

FreshSift supports exact strings, RE2 regex, binary/source-byte searching, and **filetype-only searches** such as `.jpg` or `.db` with no string required. It accepts pasted HTTP(S)/FTP URLs, hostnames, public IPs, and imported TXT/CSV lists. Optional discovery feeds supply recently observed URLs; the dashboard shows results, context, source provenance and live crawl statistics.

![FreshSift v0.2 source-search interface preview](docs/preview-source.jpg)

*Illustrative v0.2 interface preview with fictional URLs and matches; not a screenshot of a real internet scan.*

![FreshSift v0.2 file-discovery interface preview](docs/preview-files.jpg)

*Illustrative filetype-search preview. The actual application uses `web/index.html`, `web/style.css` and `web/app.js`; test your screen size in the local browser.*


## GitHub Pages client-side execution (experimental)

The Pages dashboard now runs a JavaScript crawler **in the browser** when no Go engine is connected. No Go binary or external backend is needed for:

- Local file source/visible-style searches and file-extension detection (up to 8 MiB per file).
- Crawling CORS-accessible HTTP(S) pages, static linked HTML/CSS/JS assets and file URLs.
- Source / visible-style / file-extension modes; exact matches and a restricted JavaScript-regex implementation; exclusion, TLD, domain, URL and depth filters.
- Browser-side pause/resume/stop, live results and CSV/JSON/JSONL exports, plus local presets and import/export of settings.
- Best-effort public urlscan/HN/sitemap/RSS discovery if the remote API allows browser requests. Failed discovery is reported, not silently ignored.

**Optional extension:** Install the [Chromium/Edge fetch bridge](extension/README.md) locally to make HTTP(S) requests through an explicitly permissioned Manifest V3 extension service worker, avoiding ordinary page CORS restrictions. This uses **no central crawl proxy** and does not send stored site cookies or login credentials. The extension does not follow HTTP redirects; supply final URLs. It also cannot enable FTP or execute scanned webpages' JavaScript.

**Limits:** Browser regex is not Go RE2; its patterns are restricted to reduce backtracking. HTML visible mode is an approximation and not JS-rendered DOM. Robots.txt failures skip sites when robots checking is enabled. Authentication-protected sites, request throttling, private DNS/rebinding, cross-origin network policies, external service rate limits and first-indexed timestamp verification are not solved. Static GitHub Pages alone still cannot make arbitrary cross-origin or FTP requests.

To run a scan without Go, open Pages, paste target URLs or select local files, choose a mode and press GO. For sites that reject direct browser reads, use the extension. To use anonymous FTP or the full Go RE2/network engine instead, choose **Connect Go engine**.

See [feature matrix](docs/FEATURE_MATRIX.md), [browser tests](qa/browser-engine.test.cjs), and [extension documentation](extension/README.md).

## GitHub Pages — feature-identical dashboard

[Open the FreshSift Pages interface](https://djshellshoxxx.github.io/freshift/) · [Repository](https://github.com/djshellshoxxx/freshift)

**GitHub Pages and the Go desktop app use the exact same HTML, CSS and JavaScript dashboard** (`web/` is mirrored into `docs/`). Both show the same live result panes and **every search option**: exact strings, Go RE2 regex, entire HTML/JS/CSS/source/binary searching, visible text, filetype-only discovery, .asx exclusion, FTP and IP addresses, robots.txt control, TLD/domain filters, concurrency, depth and time budgets, provider age windows, advanced search links, CSV/JSON/JSONL export, pause/resume/stop and IP enrichment. Both also expose new v0.3 tools for scope preview, settings import/export, local scan history, and filtered CSV containing the URL, matching text, source, surrounding context and timestamps.

GitHub Pages cannot host Go. Its JavaScript client can now scan local files and CORS-accessible HTTP(S) targets independently, and the optional local browser extension can fetch broader HTTP(S) destinations. Connect a token-protected Go engine only when its additional behavior (notably FTP and Go RE2) is needed. This architecture does not create a public open proxy. The Windows desktop app launches a local Go engine automatically.

**Connecting Pages to your own Go backend (Unraid / Windows / Linux):**

1. Build or download FreshSift v0.3.0. Choose a long random bearer token; set `FRESHSIFT_API_TOKEN` and `FRESHSIFT_ALLOWED_ORIGIN=https://djshellshoxxx.github.io` on the crawler host. Never place the token in the Pages repository or source code.
2. Start the Go server bound to an address reachable by your TLS reverse proxy (see example). This requires both environment variables. Do not expose the bare HTTP port directly to the internet. A VPN or other access control layer is recommended in addition to the API token.
3. Publish the crawler behind a **valid HTTPS domain** (for example `https://crawler.your-domain.example`). The reverse proxy must forward `/api/` requests without stripping `Authorization`. Do not publish `/` if you only want the Pages UI accessible.
4. Open GitHub Pages, select **Connect engine**, enter the HTTPS origin and the token, then connect. Only the engine origin is saved in browser local storage; the token is stored in browser session storage (not in GitHub). Clear it with **Disconnect**.

Example Linux server launch (in a private environment behind an HTTPS reverse proxy):

```bash
export FRESHSIFT_API_TOKEN="$(openssl rand -hex 32)"
export FRESHSIFT_ALLOWED_ORIGIN="https://djshellshoxxx.github.io"
./freshsift-linux-amd64 -listen 0.0.0.0 -port 8787 -no-browser
```

Copy the generated token to a secure password manager first; changing it disconnects Pages sessions. Limit inbound port 8787 to your reverse-proxy host at the network layer. **For a reverse proxy terminating TLS, the Pages-to-engine URL must use `https://`**; browser restrictions may reject an HTTP loopback endpoint from the public Pages HTTPS origin.

GitHub Pages workflow: [`.github/workflows/pages.yml`](.github/workflows/pages.yml). **One-time activation is required by a repository administrator:** open [Settings → Pages](https://github.com/djshellshoxxx/freshift/settings/pages), and under **Build and deployment → Source**, select **GitHub Actions**. GitHub's default `GITHUB_TOKEN` can deploy to an enabled Pages site but **cannot create the Pages site**; the workflow intentionally does not attempt `enablement: true` (which fails with `Resource not accessible by integration`).

After selecting **GitHub Actions**, open the [Publish GitHub Pages workflow](https://github.com/djshellshoxxx/freshift/actions/workflows/pages.yml) and select **Run workflow** on `main`, or push a change to `docs/`, `web/`, `qa/` or the workflow file. Confirm both the `build` and `deploy` jobs succeed before treating https://djshellshoxxx.github.io/freshift/ as live. The workflow tests that the Pages dashboard matches the Go dashboard before uploading the static `docs/` site. If deployment is blocked, the workflow now prints an actionable Pages configuration error rather than attempting privileged site creation.

### CSV for matching strings and URLs

**Export all** with CSV chosen saves every recorded match from the Go engine, including URL, IP, source, matched string, regex/exact pattern, context before/after, timestamp and HTTP status. **Filtered CSV** exports all matching records that pass the current URL, source, selected-URL and excerpt filters (not merely the first 500 visible rows). CSV cells are escaped against spreadsheet formula injection. File discovery saves the verified file URL and extension using the same columns.

## Start on Windows

1. Download `FreshSift-windows-amd64.exe` from [Releases](../../releases) **once a release has been published**, or build from source.
2. Launch the EXE; the interface opens at **http://127.0.0.1:8787**. It binds to loopback only.
3. Paste seed URLs/IPs, or enable discovery feeds. For FTP, specify `ftp://hostname/pub/` (anonymous login only).
4. Select a **Search mode**, enter strings/regex *or* wanted file extensions, and adjust filters if needed.
5. Click **GO**. Watch the result panes and statistics, then export CSV, JSON or JSONL.

A different port: `FreshSift-windows-amd64.exe -port 8999`. Run without opening a browser: `-no-browser`.

## Search modes (v0.3)

| Mode | How it works | What is matched |
|---|---|---|
| **Entire source + linked resources (default)** | Fetches HTML source plus referenced CSS, JavaScript, static media and file URLs; follows CSS `url(...)` / `@import` and static JavaScript imports. Reads HTTP(S) and FTP file bodies up to **8 MiB per resource**. | Exact strings or RE2 regex **in downloaded source bytes**, including markup, scripts, styles, plain text, binary `.db` files, and other fetched resources. Binary bytes are scanned, not decoded into a database schema or image text. |
| **Visible webpage text only** | Fetches and extracts visible-style HTML text or text/plain from navigable pages; skips embedded resources. Does not execute JavaScript. | Strings/regex in visible/plain text; not CSS, JS, HTML tags, or binary files. |
| **Discover filetypes** | Enter extensions such as `.jpg, .jpeg, .db` with **no string required**. Finds candidates from manual URLs, recent-source URLs, HTML links, HTML/CSS references and anonymous FTP listings. HTTP `HEAD` and FTP `SIZE` verify availability; range GET / FTP transfer-start fallbacks handle servers without the metadata methods. | One verified file URL result per target extension. Download of the entire matching binary file is not required. |

**Default resource behavior:** linked assets are fetched at the current page depth even when **Follow links = 0**. Link depth controls navigation to additional pages and FTP subdirectories. Resource discovery is static; dynamic links created by executing JavaScript are not followed. Embedded assets may be hosted on external domains even when **Follow external page/navigation links** is off; allow/deny domains and TLD filters still apply.

### Filters and controls

- **Exclude filetypes** — e.g., `.asx, .iso, .mp4`; case-insensitive, matched against the URL path before fetching (query strings ignored). Any other type, **including `.db`**, remains eligible.
- **Only these TLDs** — e.g., `.com, .org, .co.uk`. If set, IPv4/IPv6 literal targets are excluded; leave blank to scan IPs.
- **Only these domains** / **Exclude these domains**, plus URL includes/excludes.
- **Respect robots.txt** — **on by default** for HTTP(S). Uncheck to skip robots policy checks. FTP does not use HTTP robots.txt.
- **Enable anonymous FTP** — **on by default** in the UI. Only anonymous, passive-mode **plain FTP** is implemented. The client is read-only; authenticated FTP, FTPS/SFTP, uploads and active-mode FTP are not implemented. The passive data connection uses the validated server IP, not the IP supplied by a server in PASV responses.
- **Follow external page/navigation links** — off by default; separate from linked resource loading.
- **Crawl budgets** — link depth 0–8, absolute/extra URL directory depth, global and per-host resource caps, concurrency/host pacing, time-per-site budget, network timeout.
- **LAN/private IPs** — disallowed by default, including FTP and redirected HTTP targets; checkbox required for local/private testing.

A URL with multiple extensions can match a compound extension such as `.tar.gz` when specified; exported records retain the selected extension, source, status and observed timestamp when known. Search results are kept in memory until exported.

## Discovery feeds and age fields

| Feed | What an age window indicates |
|---|---|
| urlscan.io | Public scan observation time (subject to quota) |
| Hacker News / Algolia | Time the URL was submitted |
| RSS / Atom | Publication/update timestamp if the feed supplies it |
| Sitemaps | `<lastmod>` if supplied; may be missing or inaccurate |
| crt.sh Certificate Transparency | Certificate log timestamp for names **under seeded domains** |
| Brave Web Search API | `freshness=pd` search results; requires `BRAVE_SEARCH_API_KEY` |

**“Recently observed” does not mean “first indexed.”** No feed here proves that Google or another search engine first indexed a particular `.jpg`, `.db` or webpage within the last 24 hours. For a linked image, the observation timestamp generally belongs to the source page, **not the image**, so the image result does not inherit that timestamp. Manually supplied links and FTP listing entries do not have a verified index age. Sources may rate-limit searches or provide no results.

URLScan can optionally use `URLSCAN_API_KEY`; it and Brave credentials are read from Go process environment variables, never embedded in frontend JavaScript. The browser UI itself uses no remote trackers, scripts or fonts. The advanced search section links to Google, Bing, Brave, DuckDuckGo, Mojeek and Yandex; those links **do not import results**. InternetDB can enrich one specific public IP, not discover all recent IPs.

## Results and exports

- **Found URLs pane** — select a URL to filter the matching details.
- **Matched excerpts / Found files pane** — for source/visible modes: matched phrase and bounded context before and after; for file mode: discovered URL and matched extension.
- **Live statistics** — resources per minute, scanned content MB, matched strings or found files, distinct matched URLs, errors and progress.
- **Pause / resume / stop**, local configuration presets, quick URL copy and source/result text filters.
- **Exports** — CSV, JSON, JSON Lines. CSV escapes formula-leading fields; file results include `Kind=file` and `Area=File URL`.

Bounds: 8 MiB per downloaded body or FTP directory listing, 200 string occurrences per resource, up to 50,000 stored results per run and 500 live entries per pane. The results remain in memory; export before starting a new run or closing the program. File discovery validates availability rather than retrieving large binary bodies. The crawler is not a full browser, does not run page JavaScript, does not search password-protected FTP, and does not guarantee exhaustive file discovery.

## Architecture

```text
Manual HTTP(S) / ftp:// seeds       Recent observation sources
                  \                 /
                   URL normalization
                   filters / dedupe
                           |
              Concurrency-limited Go workers
             /             |             \
  HTTP(S) pages    CSS / JS / files     anonymous FTP
             \             |             /
           source bytes / visible text / file URL verification
                           |
                     match records
                           |
              local dashboard + export files
```

## Build and QA

Go 1.23+; no third-party Go modules required. Node 22+ runs optional UI contract tests; Python 3 runs optional local HTTP API smoke tests.

```bash
go test -race ./... -count=1 -timeout=90s
go vet ./...
node --check web/app.js
node qa/ui-contract.test.cjs
go build -o dist/freshsift-linux-amd64 .
python3 qa/api-smoke.py ./dist/freshsift-linux-amd64
```

Cross-compile Windows x64 from Linux/macOS:

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o dist/FreshSift-windows-amd64.exe .
```

PowerShell on Windows: `go build -trimpath -ldflags="-s -w" -o FreshSift-windows-amd64.exe .`

Tests use local HTTP and mock anonymous FTP servers, including robots toggles, `.asx` exclusion, TLD scoping, HTTP `HEAD` fallback, FTP directory listings, byte searching in `.db`, binary context bounding, HTML/CSS/JS asset traversal, and filetype-only exports. See **[QA results and limitations](docs/QA.md)**. Browser visual end-to-end testing was blocked by a managed Chromium URL policy in the build environment; screenshots above are explicitly illustrative.

The GitHub workflows validate format, Go vet, race tests, UI control contracts, unified Pages parity, local HTTP API smoke tests, and Windows compilation. A `v*` tag triggers a Windows x64 **prerelease**; a new repository must first exist, with GitHub Actions enabled and publish permissions for Releases. The repository is already created at `djshellshoxxx/freshift`. To publish from a local checkout:

```bash
git init && git add . && git commit -m "FreshSift v0.2.0 source and file discovery"
git branch -M main
gh auth login
git remote add origin https://github.com/djshellshoxxx/freshift.git
git push -u origin main
git tag v0.2.0
git push origin v0.2.0
```

If the repository already exists, do **not** re-create it; add its remote, push the changes and tag through your normal workflow. Check GitHub Actions logs and release assets rather than assuming publication succeeded.

## API (local loopback)

`POST /api/start`, `/api/pause`, `/api/resume`, `/api/stop`; `GET /api/status`, `/api/version`, `/api/export?format=csv|json|jsonl`, `/api/ip?ip=1.1.1.1`.

Example filetype-only scan:

```json
{
  "seeds": "https://example.org\nftp://ftp.example.org/pub/",
  "searchMode": "files",
  "fileTypes": ".jpg, .db",
  "excludeFileTypes": ".asx",
  "allowedTLDs": ".org",
  "respectRobots": true,
  "enableFTP": true,
  "depth": 1,
  "maxPathSegments": 5,
  "maxPages": 200,
  "providers": ["urlscan"]
}
```

## License

MIT; see [LICENSE](LICENSE).
