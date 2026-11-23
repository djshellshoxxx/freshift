package freshsift

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func waitForPerpetual(t *testing.T, check func() bool) {
	t.Helper()
	until := time.Now().Add(4 * time.Second)
	for time.Now().Before(until) {
		if check() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for perpetual scan state")
}

func TestPerpetualConfigValidation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Seeds = "https://example.org/"
	cfg.Patterns = "needle"
	cfg.Perpetual = true
	if err := cfg.Normalize(); err != nil {
		t.Fatal(err)
	}
	if cfg.RescanMinutes != 5 {
		t.Fatalf("default rescan minutes = %d, want 5", cfg.RescanMinutes)
	}
	for _, n := range []int{-1, 1441} {
		bad := cfg
		bad.RescanMinutes = n
		if err := bad.Normalize(); err == nil {
			t.Errorf("accepted rescan minutes %d", n)
		}
	}
}

func TestPerpetualRescansAndDeduplicates(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			requests.Add(1)
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("needle"))
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	cfg := DefaultConfig()
	cfg.Seeds, cfg.Patterns = server.URL+"/", "needle"
	cfg.Perpetual, cfg.RescanMinutes = true, 1
	cfg.AllowPrivate, cfg.RespectRobots = true, false
	cfg.Providers = nil
	cfg.MaxPages, cfg.Concurrency = 1, 1
	engine := NewEngine()
	engine.cycleInterval = 35 * time.Millisecond // test-only clock override
	if err := engine.Start(cfg); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Stop() }()
	waitForPerpetual(t, func() bool { return engine.Snapshot().Stats.Cycles >= 2 && requests.Load() >= 2 })
	snap := engine.Snapshot()
	if snap.Stats.Matches != 1 {
		t.Fatalf("duplicate matches across rounds: got %d", snap.Stats.Matches)
	}
	if snap.Stats.Status == "completed" {
		t.Fatal("perpetual scanner must not complete between rounds")
	}
	if err := engine.Stop(); err != nil {
		t.Fatal(err)
	}
	waitForPerpetual(t, func() bool { return engine.Snapshot().Stats.Status == "cancelled" })
}

func TestPerpetualPreservesMultipleOccurrencesAtSameURL(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Seeds, cfg.Patterns = "https://example.org", "needle"
	cfg.Perpetual = true
	engine := NewEngine()
	engine.cfg = cfg
	res := fetchResult{URL: "https://example.org/", Status: 200}
	c := Candidate{URL: res.URL, Source: "manual"}
	hits := []Match{
		{Pattern: "needle", Matched: "needle", Before: "", After: ""},
		{Pattern: "needle", Matched: "needle", Before: "", After: ""},
	}
	engine.appendMatches(c, res, "example.org", hits, "Source", "text")
	if got := engine.Snapshot().Stats.Matches; got != 2 {
		t.Fatalf("distinct occurrences collapsed: got %d, want 2", got)
	}
	engine.appendMatches(c, res, "example.org", hits, "Source", "text")
	if got := engine.Snapshot().Stats.Matches; got != 2 {
		t.Fatalf("identical second round duplicated matches: got %d, want 2", got)
	}
}

func TestPerpetualStateSavedAndRemovedOnStop(t *testing.T) {
	state := filepath.Join(t.TempDir(), "nested", "perpetual.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("needle")) }))
	defer server.Close()
	cfg := DefaultConfig()
	cfg.Seeds, cfg.Patterns = server.URL, "needle"
	cfg.Perpetual, cfg.RescanMinutes = true, 1
	cfg.AllowPrivate, cfg.RespectRobots = true, false
	cfg.Providers = nil
	cfg.MaxPages = 1
	engine := NewEngine()
	engine.SetStateFile(state)
	engine.cycleInterval = 100 * time.Millisecond
	if err := engine.Start(cfg); err != nil {
		t.Fatal(err)
	}
	file, err := os.Stat(state)
	if err != nil {
		t.Fatal(err)
	}
	if file.Mode().Perm()&0077 != 0 {
		t.Fatalf("state file permissions: %v", file.Mode())
	}
	if err := engine.Stop(); err != nil {
		t.Fatal(err)
	}
	waitForPerpetual(t, func() bool { return engine.Snapshot().Stats.Status == "cancelled" })
	if _, err := os.Stat(state); !os.IsNotExist(err) {
		t.Fatalf("stopped schedule must not resume on restart: %v", err)
	}
	other := NewEngine()
	other.SetStateFile(state)
	resumed, err := other.ResumeSavedScan()
	if err != nil || resumed {
		t.Fatalf("unexpected restored scan: %v, %v", resumed, err)
	}
}
