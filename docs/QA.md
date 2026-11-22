# FreshSift v0.3.0 — QA and validation record

This QA record distinguishes executed checks from work not verified in the current environment.

## Executed checks

| Check | Result | Coverage |
|---|---|---|
| `go test ./...` | Pass | Existing and new unit/integration tests |
| `go test -race ./... -count=1 -timeout=90s` | Pass | Race-enabled test suite |
| `go vet ./...` | Pass | Go static analysis |
| `gofmt -l .` | Clean | Go formatting |
| `node --check web/app.js` | Pass | JavaScript syntax |
| `node qa/ui-contract.test.cjs` | Pass | HTML IDs, mode default, checkbox and search-mode UI state/config contracts |
| `python3 qa/api-smoke.py ./dist/freshsift-linux-amd64` | Pass | Actual local Go HTTP server; synthetic websites for source/visible/file modes, filters and exports |
| `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ...` | Pass | PE32+ Windows x64 binary produced |

### Behavior verified with fixtures

- Default mode scans **HTML source, CSS, JS and raw binary `.db`** for a test phrase, including assets linked at depth 0.
- `.asx` extension exclusion occurs before fetching, case-insensitively and regardless of URL query.
- **Visible** mode ignores content that appears only in `<script>` or linked resources.
- **File** mode finds a reachable `.JPG` without a search string, using HTTP `HEAD`; falls back to range `GET` for 405 responses. FTP uses `SIZE` where available.
- Anonymous passive FTP mock server supports `MLSD` directory listings, file retrieval and file availability checks; blocked private/loopback network destinations remain blocked unless opted in.
- Robots enabled: denied path is skipped. Robots disabled: same path can be scanned.
- TLD filter rejects non-selected domains and excludes literal IP targets.
- Nested CSS `@import` and resource `url(...)`, HTML FTP links, filetype-only CSV/JSON/JSONL export, and bounded binary/regex match context.
- Local API integrates with UI configuration names and has JSON status/export contracts.

## Not verified / limits

- A managed Chromium browser policy in this container (`URLBlocklist: ["*"]`) prevented loading the loopback interface. **No browser visual, interactive Playwright, or Windows GUI runtime pass is claimed.** The generated README images are labeled illustrative design previews.
- External urlscan/CT/HN/Brave rate limits and real-world discovery freshness were not live-tested during this change.
- The Windows EXE was **cross-compiled and identified as PE32+ x64** but not executed on a Windows machine here.
- Plain anonymous FTP is supported; FTPS/SFTP/authenticated FTP and JS-executed browser navigation are not supported.
- Max HTTP/FTP body: 8 MiB; larger raw resources are not fully searched. Filetype-only verification can identify large matches without downloading their bodies.
- Newly observed timestamps do **not** establish first search engine indexing dates.

## Reproduce tests

```bash
go test -race ./... -count=1 -timeout=90s
go vet ./...
node --check web/app.js
node qa/ui-contract.test.cjs
go build -o dist/freshsift-linux-amd64 .
python3 qa/api-smoke.py ./dist/freshsift-linux-amd64
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o dist/FreshSift-windows-amd64.exe .
```

## v0.3 additions

Run `go test -race ./...`, `node qa/ui-contract.test.cjs`, and `node qa/pages-parity.test.cjs`. The Pages source uses the same application controls, with a secure Go API backend required for runtime crawling. New tests cover allowed-origin / CORS / bearer token behavior, CSV matching rows and injection mitigation, and UI parity. Browser visual testing and a live GitHub Pages deployment must be verified separately.

### v0.3 executed verification (2026-11-22 UTC)

- Go `go test ./...` and `go test -race ./...`: PASS.
- `go vet ./...`, gofmt check: PASS.
- `node --check web/app.js`, `node qa/ui-contract.test.cjs`, `node qa/pages-parity.test.cjs`: PASS.
- `python3 qa/api-smoke.py` against built Go engine: PASS (HTML, CSS, JS, `.db`, `.asx`, .jpg, robots and TLD behavior).
- `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build`: PASS (Windows PE32+ 64-bit).
- Browser screenshot attempt with headless Chromium did not complete due to runtime browser limitations. Existing screenshots remain **illustrative**, not live captures.
- Live GitHub Actions release and Pages publication: must be observed separately after pushing source and enabling Pages.
