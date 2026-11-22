package freshsift

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func waitUntilFinished(t *testing.T, e *Engine) Snapshot {
	t.Helper()
	deadline := time.After(7 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("crawler never finished")
		default:
		}
		s := e.Snapshot()
		if s.Stats.Status == "completed" || s.Stats.Status == "cancelled" {
			return s
		}
		time.Sleep(25 * time.Millisecond)
	}
}
func TestCrawlTwoPagesWithContextAndStats(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			fmt.Fprint(w, "User-agent: *\nAllow: /\n")
		case "/":
			fmt.Fprint(w, `<html><body>before needle after <a href="/child">next</a></body></html>`)
		case "/child":
			fmt.Fprint(w, `<html><body>now a second needle here</body></html>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := DefaultConfig()
	c.Providers = nil
	c.Seeds = srv.URL
	c.Patterns = "needle"
	c.Depth = 1
	c.Concurrency = 3
	c.AllowPrivate = true
	e := NewEngine()
	if err := e.Start(c); err != nil {
		t.Fatal(err)
	}
	result := waitUntilFinished(t, e)
	if result.Stats.PagesScanned != 2 || result.Stats.Matches != 2 || result.Stats.MatchedPages != 2 {
		t.Fatalf("incorrect crawling statistics: %+v; logs %+v", result.Stats, result.Logs)
	}
	if result.Stats.TextBytes == 0 || result.Stats.BytesScanned == 0 {
		t.Fatalf("byte stats missing: %+v", result.Stats)
	}
	if len(e.AllResults()) != 2 || !strings.Contains(result.Matches[0].Context, "needle") {
		t.Fatalf("missing matched context: %+v", result.Matches)
	}
}
func TestCrawlHonorsRobots(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			fmt.Fprint(w, "User-agent: *\nDisallow: /private\n")
		case "/":
			fmt.Fprint(w, `<html>hello <a href="/private">private</a></html>`)
		case "/private":
			fmt.Fprint(w, `<html>TOP_SECRET_MATCH</html>`)
		}
	}))
	defer srv.Close()
	c := DefaultConfig()
	c.Providers = nil
	c.Seeds = srv.URL
	c.Patterns = "TOP_SECRET_MATCH"
	c.Depth = 2
	c.Concurrency = 2
	c.AllowPrivate = true
	e := NewEngine()
	if err := e.Start(c); err != nil {
		t.Fatal(err)
	}
	result := waitUntilFinished(t, e)
	if result.Stats.PagesScanned != 1 || result.Stats.Matches != 0 || result.Stats.Skipped < 1 {
		t.Fatalf("robots exclusion not enforced: %+v", result.Stats)
	}
}
func TestCrawlDoesNotFollowExternalByDefault(t *testing.T) {
	ext := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "needle other") }))
	defer ext.Close()
	root := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			fmt.Fprint(w, "User-agent: *\nAllow: /\n")
			return
		}
		fmt.Fprint(w, `<html><body><a href="`+ext.URL+`/">external</a> no match</body></html>`)
	}))
	defer root.Close()
	c := DefaultConfig()
	c.Providers = nil
	c.Seeds = root.URL
	c.Patterns = "needle"
	c.Depth = 2
	c.AllowPrivate = true
	e := NewEngine()
	if err := e.Start(c); err != nil {
		t.Fatal(err)
	}
	result := waitUntilFinished(t, e)
	if result.Stats.Matches != 0 || result.Stats.PagesScanned != 1 {
		t.Fatalf("external link was followed: %+v", result.Stats)
	}
}
func TestTimerModeOverridesLinkDepthAndRejectsZeroTime(t *testing.T) {
	c := DefaultConfig()
	c.Providers = nil
	c.Seeds = "https://example.org"
	c.Patterns = "needle"
	c.BudgetMode = "timer"
	c.SiteMinutes = 0
	if err := c.Normalize(); err == nil {
		t.Fatal("timer mode accepted zero-minute limit")
	}
	c.SiteMinutes = 1
	if err := c.Normalize(); err != nil {
		t.Fatal(err)
	}
	if c.Depth != 8 || c.MaxPathSegments != 50 || c.MaxPagesPerHost != 10000 {
		t.Fatalf("timer limits not active: %+v", c)
	}
}
