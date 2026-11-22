package freshsift

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRSSDiscoveryOnlyRecentAndUndatedEntries(t *testing.T) {
	now := time.Now().UTC()
	recent := now.Add(-15 * time.Minute).Format(time.RFC1123Z)
	old := now.Add(-72 * time.Hour).Format(time.RFC1123Z)
	var origin string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/feed.xml" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		fmt.Fprintf(w, `<rss version="2.0"><channel><title>Test feed</title><item><link>%s/new</link><pubDate>%s</pubDate></item><item><link>%s/old</link><pubDate>%s</pubDate></item><item><link>%s/undated</link></item></channel></rss>`, origin, recent, origin, old, origin)
	}))
	defer srv.Close()
	origin = srv.URL
	c := DefaultConfig()
	c.Providers = []string{"rss"}
	c.RecentHours = 24
	found, err := discoverRSS(context.Background(), newHTTPClient(4*time.Second, true), c, []Candidate{{URL: srv.URL}}, func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 2 || found[0].URL != origin+"/new" || found[1].URL != origin+"/undated" {
		t.Fatalf("recent/undated entries should survive and old entry should be excluded: %+v", found)
	}
	if found[0].ObservedAt.IsZero() || !found[1].ObservedAt.IsZero() {
		t.Fatalf("observed time provenance incorrect: %+v", found)
	}
}

func TestSitemapDiscoveryFiltersOldLastmod(t *testing.T) {
	recent := time.Now().UTC().Add(-20 * time.Minute).Format(time.RFC3339)
	old := time.Now().UTC().Add(-96 * time.Hour).Format(time.RFC3339)
	var origin string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sitemap.xml" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprintf(w, `<urlset><url><loc>%s/new</loc><lastmod>%s</lastmod></url><url><loc>%s/old</loc><lastmod>%s</lastmod></url></urlset>`, origin, recent, origin, old)
	}))
	defer srv.Close()
	origin = srv.URL
	c := DefaultConfig()
	c.RecentHours = 24
	c.DiscoveryLimit = 10
	found, err := discoverSitemaps(context.Background(), newHTTPClient(4*time.Second, true), c, []Candidate{{URL: srv.URL}}, func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].URL != origin+"/new" {
		t.Fatalf("unexpected sitemap discovery: %+v", found)
	}
}
