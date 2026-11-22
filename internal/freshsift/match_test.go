package freshsift

import (
	"strings"
	"testing"
)

func TestParseSeedsAcceptsHostsAndIPsAndDeduplicates(t *testing.T) {
	x, err := parseSeeds("example.com\nhttps://example.com/\nhttp://8.8.8.8:8080/path\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(x) != 2 {
		t.Fatalf("got %d URLs: %+v", len(x), x)
	}
	if x[1].URL != "http://8.8.8.8:8080/path" {
		t.Fatalf("IP incorrectly normalized: %q", x[1].URL)
	}
}
func TestVisibleTextDropsScriptAndStripsTags(t *testing.T) {
	s := visibleText(`<html><style>SECRET</style><h1>Search <em>needle</em></h1><script>OTHER_SECRET</script><p>hello &amp; goodbye</p></html>`)
	if s != "Search needle hello & goodbye" {
		t.Fatalf("visible text: %q", s)
	}
}
func TestMatchContextAndRegex(t *testing.T) {
	c := DefaultConfig()
	c.Patterns = "hello[0-9]+"
	c.Regex = true
	ms, err := compileMatchers(c)
	if err != nil {
		t.Fatal(err)
	}
	result := findMatches("one two three four five six seven eight hello123 nine ten eleven", ms, 10)
	if len(result) != 1 || result[0].Matched != "hello123" {
		t.Fatalf("match: %+v", result)
	}
	if !strings.Contains(result[0].Before, "two three") || !strings.Contains(result[0].After, "nine ten") {
		t.Errorf("context: %+v", result[0])
	}
}
func TestRejectMalformedRegex(t *testing.T) {
	c := DefaultConfig()
	c.Patterns = "[broken"
	c.Regex = true
	if err := c.Normalize(); err == nil {
		t.Fatal("invalid regex accepted")
	}
}
func TestPathsAndDomainFilter(t *testing.T) {
	if pathSegments("https://a.com/a/b?q=x") != 2 {
		t.Fatal("wrong depth")
	}
	if !hostMatches("sub.example.com", "example.com") || hostMatches("notexample.com", "example.com") {
		t.Fatal("domain matching unsafe")
	}
}
func TestLinkExtractionRelativeAndExternal(t *testing.T) {
	links := pageLinks("https://example.org/blog/", `<a href="../about">a</a><a href="https://other.org/page#frag">x</a><a href="javascript:alert(1)">no</a>`)
	if len(links) != 2 || links[0] != "https://example.org/about" || links[1] != "https://other.org/page" {
		t.Fatalf("links: %#v", links)
	}
}
