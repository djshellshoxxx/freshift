package freshsift

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSubdomainWordlistNormalizesAndRejectsUnsafeLabels(t *testing.T) {
	words, err := parseSubdomainWords(" API, www\nfoo.bar\n# comment\ninvalid/word\n*.wild\n-dev\napi", false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"api", "www", "foo.bar"}
	if !reflect.DeepEqual(words, want) {
		t.Fatalf("words=%v want %v", words, want)
	}
	builtin, err := parseSubdomainWords("my-portal", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(builtin) < 60 {
		t.Fatalf("built-in list too small: %d", len(builtin))
	}
	if builtin[0] != "www" {
		t.Fatalf("unexpected built-in order: %v", builtin[:1])
	}
	if len(strings.Join(builtin, ",")) == 0 {
		t.Fatal("missing builtins")
	}
	if _, err := parseSubdomainWords(strings.Repeat("a", 200001), false); err == nil {
		t.Fatal("oversized wordlist accepted")
	}
}

func TestCTSubdomainsScopeAndWildcard(t *testing.T) {
	raw := []byte(`[{"name_value":"*.alpha.github.io\nbeta.github.io\nalpha.evilgithub.io\ngithub.io\nALPHA.github.io\ninvalid_host.github.io"},{"name_value":"foo.bar.github.io"}]`)
	got := parseCTSubdomains(raw, "github.io", 50)
	want := []string{"alpha.github.io", "beta.github.io", "foo.bar.github.io"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CT=%v want=%v", got, want)
	}
	if got := parseCTSubdomains([]byte("not-json"), "github.io", 10); len(got) != 0 {
		t.Fatal("malformed JSON accepted")
	}
}

func TestDNSSubdomainResolutionSuppressedForWildcardAndRetainsValid(t *testing.T) {
	var lookups atomic.Int32
	lookup := func(_ context.Context, name string) ([]string, error) {
		lookups.Add(1)
		if name == "api.github.io" {
			return []string{"203.0.113.42"}, nil
		}
		if name == "mail.github.io" {
			return nil, errors.New("no DNS record")
		}
		return []string{"192.0.2.9"}, nil // wildcard
	}
	found := resolveWordlist(context.Background(), "github.io", []string{"api", "www", "mail"}, 25, lookup)
	if len(found) != 1 || found[0].Hostname != "api.github.io" || !found[0].Verified || found[0].Source != "dns" {
		t.Fatalf("unexpected subdomains %+v", found)
	}
	if lookups.Load() < 4 {
		t.Fatal("wildcard detection did not query control labels")
	}
}

func TestSubdomainConfigValidation(t *testing.T) {
	c := DefaultConfig()
	c.Seeds = "https://github.io"
	c.Patterns = "needle"
	c.Providers = nil
	c.EnumerateSubdomains = true
	c.SubdomainUseBuiltIn = true
	c.SubdomainLimit = 2000
	if err := c.Normalize(); err != nil {
		t.Fatal(err)
	}
	if !c.EnumerateSubdomains || c.SubdomainLimit != 2000 {
		t.Fatal("subdomains missing from config")
	}
	c.SubdomainLimit = 2001
	if err := c.Normalize(); err == nil {
		t.Fatal("excessive subdomain limit accepted")
	}
	c.SubdomainLimit = 50
	c.Seeds = ""
	if err := c.Normalize(); err == nil {
		t.Fatal("subdomains accepted without a scoped seed")
	}
}
