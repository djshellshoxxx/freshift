# FreshSift

**FreshSift** searches web content, page source, linked assets and selected file extensions. This repository is being restored from the original **v0.2.0** source archive.

## GitHub Pages browser edition

The [static browser edition](https://djshellshoxxx.github.io/freshift/) supports:

- Text or regular-expression searching inside files selected from your computer.
- HTTP(S) URL source searching **only when the destination allows browser CORS requests**.
- Visible-style HTML text filtering and extension-based URL matching.
- Domain, TLD and excluded-extension filters.
- JSON export. Uploaded files stay in your browser.

The browser edition is intentionally not represented as the full crawler. GitHub Pages cannot run a Go backend. Browser fetches cannot generally read arbitrary websites or anonymous FTP endpoints. The browser edition does not offer the local Go crawler's discovery providers, full recursive traversal, FTP or robust HTTP HEAD file verification.

### Deployment

The Pages workflow is in `.github/workflows/pages.yml`. Enable **Settings → Pages → Build and deployment → Source: GitHub Actions**. If Pages is not enabled in repository settings, the workflow may fail until this is configured.

## Original application

The original FreshSift v0.2.0 archive was recovered from ChatGPT Library during repository restoration. It contains the Go crawler, original UI, QA tests, release workflows and documentation. **The complete Go source has not yet been uploaded into this repository.** Do not interpret the Pages deployment as a release of the original Go crawler.

Key original capabilities: source/visible/filetype modes, HTTP(S) and anonymous passive FTP, optional observation discovery providers, domain/TLD/exclusion filters, robots handling, export, and local API. Time of discovery does **not** prove first search-engine indexing.

The original archive passed local `go test ./...`, `go vet ./...`, and `node qa/ui-contract.test.cjs` when rechecked during recovery.

License of the original source: MIT.
