# FreshSift Local Fetch Bridge (Chromium/Edge)

This optional Manifest V3 extension makes HTTP(S) requests from the browser's extension service worker, so the FreshSift GitHub Pages dashboard can read websites that do not permit cross-origin JavaScript fetches. It runs on your machine; no hosted proxy or Go server is required for this HTTP(S) mode.

## Install from source

1. Download/clone this repository, including the extension folder.
2. In Chrome, open chrome://extensions (Edge: edge://extensions).
3. Enable **Developer mode**, choose **Load unpacked**, and select the extension folder (not the repository root).
4. Review and approve the requested website permissions. The extension can request HTTP(S) resources on all hostnames because that is what broad crawling requires.
5. Open https://djshellshoxxx.github.io/freshift/ and reload the page. Searches will use the extension when its bridge is detected.

Only FreshSift Pages at the path /freshift/ gets the extension bridge. It does not capture cookies, execute scripts from scanned pages, or request resources with stored browser login credentials. Responses and matched content are handled in the user's browser.

## What the bridge does and does not do

- Browser-local HTTP(S) fetches with user-approved host permissions; no external proxy.
- GET, HEAD, and a one-byte Range fallback; up to 8 MiB response bodies.
- Rejects credential-bearing URLs, FTP/SFTP, and literal private/localhost hosts unless the FreshSift **Allow private IPs** setting is selected.
- Does not follow redirects (to avoid redirecting public requests into unexpected destinations). Supply the final URL when a site redirects.
- Does **not** provide FTP/TCP sockets, JavaScript execution of scanned pages, RE2-compatible regular expressions, real first-indexed timestamps, or immunity from websites that reject automated requests.
- DNS-rebinding risks cannot be eliminated by simple browser-level hostname checks. Do not grant broad extension permissions on an untrusted machine or run it against untrusted local network destinations.

The Go edition remains available for anonymous FTP and its additional networking/discovery behavior. Browser mode also works without the extension on servers that permit CORS.

## Validate

From the repo root:

    node --check extension/background.js
    node --check extension/content-script.js
    node qa/extension-contract.test.cjs
    node qa/browser-engine.test.cjs

The provided tests exercise transport restrictions and a mock client-side crawler, not the actual Chrome extension install/runtime or a live external scan.
