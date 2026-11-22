package freshsift

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestNewDefaultsAndModes(t *testing.T) {
	c := DefaultConfig()
	if c.SearchMode != "source" || c.OnlyHTML || !c.EnableFTP || !c.RespectRobots {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	c.Providers = nil
	c.Seeds = "https://example.org"
	c.Patterns = ""
	c.SearchMode = "files"
	c.FileTypes = ".jpg, .DB"
	if err := c.Normalize(); err != nil {
		t.Fatal(err)
	}
	if c.SearchMode != "files" {
		t.Fatal("file mode changed")
	}
	for _, bad := range []string{"*", "foo/bar", "jpg*", "."} {
		test := c
		test.FileTypes = bad
		if err := test.Normalize(); err == nil {
			t.Errorf("accepted invalid filetype %q", bad)
		}
	}
	test := c
	test.SearchMode = "visible"
	test.Patterns = ""
	if err := test.Normalize(); err == nil {
		t.Fatal("visible mode accepted no patterns")
	}
}
func TestFiletypeExclusionsAndTLDTargets(t *testing.T) {
	c := DefaultConfig()
	c.ExcludeFileTypes = ".asx, DB"
	c.AllowedTLDs = ".com, co.uk"
	c.Providers = nil
	c.Seeds = "https://example.com"
	c.Patterns = "needle"
	if err := c.Normalize(); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		url string
		ok  bool
	}{
		{"https://example.com/a.db", false}, {"https://example.com/a.asx?x=1", false},
		{"https://example.com/A.JPG?v=1", true}, {"https://sub.example.co.uk/a.db", false},
		{"https://sub.example.co.uk/p", true}, {"https://example.org/a", false},
		{"https://fakecom/a", false}, {"http://1.1.1.1:8080/", false},
	}
	for _, tc := range tests {
		if got := candidateAllowed(c, tc.url); got != tc.ok {
			t.Errorf("candidateAllowed(%q)=%v want %v", tc.url, got, tc.ok)
		}
	}
	if canonical, err := canonicalURL("ftp://ftp.example.com/pub/image.jpg"); err != nil || canonical != "ftp://ftp.example.com/pub/image.jpg" {
		t.Fatalf("ftp URL normalization %s %v", canonical, err)
	}
	if _, err := canonicalURL("ftp://bob:secret@ftp.example.com/path"); err == nil {
		t.Fatal("FTP credentials allowed")
	}
}
func TestHTMLAndCSSResources(t *testing.T) {
	body := `<html><head><link rel="stylesheet" href="/main.css"><script src='/app.js'></script><link rel="icon" href="/favicon.ico"></head><body><a href="/next.html">next</a><a href="/dump.db">db</a><img src="/i.png"><source srcset="/small.jpg 1x, /large.jpg 2x"></body></html>`
	pages, assets := extractHTMLLinks("https://example.org/start.html", body)
	if !containsURL(pages, "https://example.org/next.html") {
		t.Fatalf("missing page link: %v", pages)
	}
	for _, u := range []string{"/main.css", "/app.js", "/favicon.ico", "/i.png", "/small.jpg", "/large.jpg", "/dump.db"} {
		if !containsURL(assets, "https://example.org"+u) {
			t.Errorf("missing asset %s (%v)", u, assets)
		}
	}
	css := `@import 'extra.css';body{background:url("/images/bg.png")}`
	urls := extractCSSLinks("https://example.org/main.css", css)
	if !containsURL(urls, "https://example.org/extra.css") || !containsURL(urls, "https://example.org/images/bg.png") {
		t.Fatalf("CSS URLs: %v", urls)
	}
}
func containsURL(list []string, u string) bool {
	for _, x := range list {
		if x == u {
			return true
		}
	}
	return false
}

func TestSourceDefaultScansAssetsAndDBButSkipsASX(t *testing.T) {
	var blockedRequests atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			fmt.Fprint(w, "User-agent: *\nAllow: /\n")
		case "/":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<html><body>visible only<link rel="stylesheet" href="/styles.css"><script src="/app.js"></script><a href="/data.db">db</a><a href="/blocked.asx">blocked</a></body></html>`)
		case "/styles.css":
			w.Header().Set("Content-Type", "text/css")
			fmt.Fprint(w, `body{content:'findme CSS'}`)
		case "/app.js":
			w.Header().Set("Content-Type", "text/javascript")
			fmt.Fprint(w, `const secret='findme JS'`)
		case "/data.db":
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write([]byte{0, 1, 2})
			fmt.Fprint(w, `findme DB`)
		case "/blocked.asx":
			blockedRequests.Add(1)
			fmt.Fprint(w, "findme ASX")
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := DefaultConfig()
	c.Seeds = srv.URL
	c.Providers = nil
	c.Patterns = "findme\nneedle"
	c.ExcludeFileTypes = ".asx"
	c.Depth = 0
	c.Concurrency = 2
	c.AllowPrivate = true
	c.ExcludeFileTypes = ".asx"
	e := NewEngine()
	if err := e.Start(c); err != nil {
		t.Fatal(err)
	}
	snap := waitUntilFinished(t, e)
	if blockedRequests.Load() != 0 {
		t.Fatalf("excluded .asx was fetched %d times", blockedRequests.Load())
	}
	if snap.Stats.Matches != 3 {
		t.Fatalf("want 3 resource matches, got %d: %+v / %+v", snap.Stats.Matches, snap.Stats, snap.Matches)
	}
	for _, m := range snap.Matches {
		if m.Area != "Raw source" {
			t.Errorf("wrong source area %s", m.Area)
		}
	}
}
func TestVisibleOnlySkipsSourceAndAssets(t *testing.T) {
	var jsRequests atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/app.js" {
			jsRequests.Add(1)
			fmt.Fprint(w, "needle")
			return
		}
		if r.URL.Path == "/robots.txt" {
			fmt.Fprint(w, "User-agent: *\nAllow: /\n")
			return
		}
		fmt.Fprint(w, `<html><head><script>needle</script><script src="/app.js"></script></head><body>hello world</body></html>`)
	}))
	defer srv.Close()
	c := DefaultConfig()
	c.Seeds = srv.URL
	c.Providers = nil
	c.Patterns = "needle"
	c.SearchMode = "visible"
	c.Depth = 0
	c.AllowPrivate = true
	e := NewEngine()
	if err := e.Start(c); err != nil {
		t.Fatal(err)
	}
	snap := waitUntilFinished(t, e)
	if snap.Stats.Matches != 0 || jsRequests.Load() != 0 {
		t.Fatalf("visible-only scanned resource/source: %+v js=%d", snap.Stats, jsRequests.Load())
	}
}
func TestFileModeFindsImageWithoutDownloadingBody(t *testing.T) {
	var imageGET atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			fmt.Fprint(w, "User-agent: *\nAllow: /\n")
		case "/":
			fmt.Fprint(w, `<html><a href="/images/2026.JPG?size=large">photo</a><a href="/x.asx">exclude</a><a href="/missing.jpg">gone</a></html>`)
		case "/images/2026.JPG":
			if r.Method == http.MethodGet {
				imageGET.Add(1)
			}
			w.Header().Set("Content-Type", "image/jpeg")
			w.Header().Set("Content-Length", "10000000")
			if r.Method == http.MethodGet {
				fmt.Fprint(w, "fake")
			}
		case "/missing.jpg":
			http.NotFound(w, r)
		case "/x.asx":
			fmt.Fprint(w, "ignored")
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := DefaultConfig()
	c.Seeds = srv.URL
	c.Providers = nil
	c.Patterns = ""
	c.SearchMode = "files"
	c.FileTypes = ".jpg"
	c.ExcludeFileTypes = ".asx"
	c.Depth = 0
	c.AllowPrivate = true
	e := NewEngine()
	if err := e.Start(c); err != nil {
		t.Fatal(err)
	}
	snap := waitUntilFinished(t, e)
	if snap.Stats.Matches != 1 || snap.Stats.MatchedPages != 1 || imageGET.Load() != 0 {
		t.Fatalf("file discovery %v, GET=%d matches=%+v", snap.Stats, imageGET.Load(), snap.Matches)
	}
	if !strings.Contains(snap.Matches[0].URL, ".JPG?") || snap.Matches[0].Area != "File URL" {
		t.Fatalf("bad file record: %+v", snap.Matches[0])
	}
}
func TestRobotsCanBeDisabled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			fmt.Fprint(w, "User-agent: *\nDisallow: /\n")
			return
		}
		fmt.Fprint(w, "needle")
	}))
	defer srv.Close()
	c := DefaultConfig()
	c.Seeds = srv.URL
	c.Providers = nil
	c.Patterns = "needle"
	c.AllowPrivate = true
	c.RespectRobots = false
	e := NewEngine()
	if err := e.Start(c); err != nil {
		t.Fatal(err)
	}
	snap := waitUntilFinished(t, e)
	if snap.Stats.Matches != 1 {
		t.Fatalf("robots-disabled mode didn't fetch page: %+v", snap.Stats)
	}
}

func TestRawBinaryContextAndRegexMatchAreBounded(t *testing.T) {
	c := DefaultConfig()
	c.Patterns = "needle"
	ms, _ := compileMatchers(c)
	binary := strings.Repeat("\x00", 2<<20) + "needle" + strings.Repeat("\x01", 2<<20)
	hits := findMatches(binary, ms, 200)
	if len(hits) != 1 || len(hits[0].Context) > 1000 || len(hits[0].Before) > 500 || len(hits[0].After) > 500 {
		t.Fatalf("binary match context not bounded: %d hits, before=%d, after=%d, context=%d", len(hits), len(hits[0].Before), len(hits[0].After), len(hits[0].Context))
	}
	c.Patterns = "n.*e"
	c.Regex = true
	ms, _ = compileMatchers(c)
	hits = findMatches("n"+strings.Repeat("z", 1<<20)+"e", ms, 5)
	if len(hits) != 1 || len(hits[0].Matched) > 512 {
		t.Fatalf("regex matched unbounded content: %d", len(hits[0].Matched))
	}
}

func TestFileModeHEADFallbackToRangeGET(t *testing.T) {
	var gotRange atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			fmt.Fprint(w, "User-agent: *\nAllow: /\n")
			return
		}
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("Range") == "bytes=0-0" {
			gotRange.Store(true)
		}
		w.WriteHeader(http.StatusPartialContent)
		w.Write([]byte("x"))
	}))
	defer srv.Close()
	c := DefaultConfig()
	c.SearchMode = "files"
	c.FileTypes = ".jpg"
	c.Providers = nil
	c.Seeds = srv.URL + "/example.jpg"
	c.AllowPrivate = true
	e := NewEngine()
	if err := e.Start(c); err != nil {
		t.Fatal(err)
	}
	result := waitUntilFinished(t, e)
	if result.Stats.FilesFound != 1 || !gotRange.Load() {
		t.Fatalf("405 HEAD fallback failed: %+v logs=%v", result.Stats, result.Logs)
	}
}
func TestEmbeddedFTPAndNestedCSSAtZeroPageDepth(t *testing.T) {
	seed, _ := startFakeFTP(t)
	var base string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			fmt.Fprint(w, "User-agent: *\nAllow: /\n")
		case "/":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<link rel="stylesheet" href="/base.css"><a href="`+seed+`">FTP root</a>`)
		case "/base.css":
			w.Header().Set("Content-Type", "text/css")
			fmt.Fprint(w, `@import "nested.css"; body{background:url("/test.jpg")}`)
		case "/nested.css":
			w.Header().Set("Content-Type", "text/css")
			fmt.Fprint(w, `body{font-family:'findme nested CSS'}`)
		case "/test.jpg":
			w.Header().Set("Content-Type", "image/jpeg")
			fmt.Fprint(w, "findme jpeg bytes")
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	base = srv.URL
	c := DefaultConfig()
	c.Seeds = base
	c.Providers = nil
	c.Patterns = "findme\nneedle"
	c.ExcludeFileTypes = ".asx"
	c.Depth = 0
	c.AllowPrivate = true
	c.Concurrency = 2
	e := NewEngine()
	if err := e.Start(c); err != nil {
		t.Fatal(err)
	}
	result := waitUntilFinished(t, e)
	if result.Stats.Matches != 3 {
		t.Fatalf("expected nested CSS, image, and FTP .db; got %+v matches %+v log %+v", result.Stats, result.Matches, result.Logs)
	}
}

func TestFileModeIgnoresInactiveRegexField(t *testing.T) {
	c := DefaultConfig()
	c.Seeds = "https://example.com"
	c.Providers = nil
	c.SearchMode = "files"
	c.FileTypes = ".jpg"
	c.Regex = true
	c.Patterns = "[broken regex"
	if err := c.Normalize(); err != nil {
		t.Fatalf("hidden regex field blocked file search: %v", err)
	}
	if _, err := compileMatchers(c); err != nil {
		t.Fatalf("compiled irrelevant regex in file mode: %v", err)
	}
}
