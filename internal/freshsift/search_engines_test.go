package freshsift

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearXNGSelectedEnginesAndDeduplication(t *testing.T) {
	requests := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/search" || r.URL.Query().Get("q") != "paste" ||
			r.URL.Query().Get("engines") != "google,bing" || r.URL.Query().Get("format") != "json" ||
			r.URL.Query().Get("time_range") != "day" {
			t.Errorf("unexpected request %s", r.URL.String())
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("pageno") == "1" {
			fmt.Fprint(w, `{"results":[{"url":"https://example.org/new","engines":["google"]},{"url":"https://example.org/new","engines":["bing"]},{"url":"javascript:alert(1)"},{"url":"https://example.org/next","engines":["bing"]}]}`)
		} else {
			fmt.Fprint(w, `{"results":[{"url":"https://example.org/next","engines":["bing"]}]}`)
		}
	}))
	defer server.Close()
	c := DefaultConfig()
	c.SourceQuery = "paste"
	c.SearxngURL = server.URL
	c.SearchEngines = []string{"google", "bing"}
	c.DiscoveryLimit = 5
	found, err := discoverSearXNG(context.Background(), server.Client(), c, func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("pagination should stop when no new URLs: %d", requests)
	}
	if len(found) != 2 || found[0].URL != "https://example.org/new" ||
		found[0].Source != "searxng/google" || found[1].URL != "https://example.org/next" ||
		found[1].Source != "searxng/bing" {
		t.Fatalf("bad results: %+v", found)
	}
}

func TestSearXNGConfigValidation(t *testing.T) {
	c := DefaultConfig()
	c.Providers = []string{"searxng"}
	c.Patterns = "needle"
	c.SearchEngines = []string{"google"}
	for _, endpoint := range []string{"", "http://example.org", "https://user:pass@example.org", "https://example.org/?token=1"} {
		bad := c
		bad.SearxngURL = endpoint
		if err := bad.Normalize(); err == nil {
			t.Errorf("accepted invalid SearXNG: %s", endpoint)
		}
	}
	c.SearxngURL = "https://search.example.org"
	bad := c
	bad.SearchEngines = []string{"invalid"}
	if err := bad.Normalize(); err == nil || !strings.Contains(err.Error(), "search engine") {
		t.Errorf("unknown search engine not rejected: %v", err)
	}
	if err := c.Normalize(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
}
