package freshsift

import (
	"html"
	"net/url"
	"regexp"
	"strings"
)

var (
	stripTags   = regexp.MustCompile(`(?s)<[^>]*>`)
	whitespace  = regexp.MustCompile(`\s+`)
	linkPattern = regexp.MustCompile(`(?i)\bhref\s*=\s*["']([^"']+)["']`)
	htmlTags    = regexp.MustCompile(`(?is)<([a-z][a-z0-9:-]*)\b([^>]*)>`)
	htmlAttrs   = regexp.MustCompile(`(?is)([a-z][a-z0-9:-]*)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)
	cssURL      = regexp.MustCompile(`(?is)url\(\s*['"]?([^'"\)]+)['"]?\s*\)`)
	cssImport   = regexp.MustCompile(`(?is)@import\s+['"]([^'"]+)['"]`)
	jsImport    = regexp.MustCompile(`(?m)\b(?:from\s*|import\s*\(?\s*)['"]([^'"\n]+)['"]`)
)

func visibleText(input string) string {
	// RE2 intentionally has no backreferences; strip each element separately.
	for _, tag := range []string{"script", "style", "noscript", "svg", "template"} {
		re := regexp.MustCompile(`(?is)<` + tag + `\b[^>]*>.*?</` + tag + `\s*>`)
		input = re.ReplaceAllString(input, " ")
	}
	input = stripTags.ReplaceAllString(input, " ")
	input = html.UnescapeString(input)
	return strings.TrimSpace(whitespace.ReplaceAllString(input, " "))
}

func pageLinks(base string, body string) []string {
	u, err := url.Parse(base)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, m := range linkPattern.FindAllStringSubmatch(body, 1000) {
		target := html.UnescapeString(strings.TrimSpace(m[1]))
		v, e := url.Parse(target)
		if e != nil || v.Scheme != "" && v.Scheme != "https" && v.Scheme != "http" {
			continue
		}
		joined := u.ResolveReference(v)
		clean, e := canonicalURL(joined.String())
		if e != nil || seen[clean] {
			continue
		}
		seen[clean] = true
		out = append(out, clean)
	}
	return out
}

// extractHTMLLinks separates navigable pages from embedded/static resources.
// Static resources do not consume the user-selected page recursion depth.
func extractHTMLLinks(base, body string) ([]string, []string) {
	pages, assets := []string{}, []string{}
	pageSeen, assetSeen := map[string]bool{}, map[string]bool{}
	add := func(raw string, resource bool) {
		resolved := resolveLink(base, raw)
		if resolved == "" {
			return
		}
		if resource {
			if !assetSeen[resolved] {
				assets = append(assets, resolved)
				assetSeen[resolved] = true
			}
		} else if !pageSeen[resolved] {
			pages = append(pages, resolved)
			pageSeen[resolved] = true
		}
	}
	for _, tag := range htmlTags.FindAllStringSubmatch(body, 3000) {
		name := strings.ToLower(tag[1])
		attrs := map[string]string{}
		for _, attr := range htmlAttrs.FindAllStringSubmatch(tag[2], -1) {
			value := attr[2]
			if value == "" {
				value = attr[3]
			}
			if value == "" {
				value = attr[4]
			}
			attrs[strings.ToLower(attr[1])] = html.UnescapeString(strings.TrimSpace(value))
		}
		switch name {
		case "a", "area":
			v := attrs["href"]
			add(v, resourceExtension(v) || strings.HasPrefix(strings.ToLower(v), "ftp://"))
		case "link":
			v := attrs["href"]
			rel := strings.ToLower(attrs["rel"])
			// Most link elements describe assets; canonical/alternate links denote pages.
			resource := !(strings.Contains(rel, "canonical") || strings.Contains(rel, "alternate") || strings.Contains(rel, "next") || strings.Contains(rel, "prev"))
			add(v, resource || resourceExtension(v))
		case "script", "img", "iframe", "source", "audio", "video", "embed", "track", "input":
			add(attrs["src"], true)
			if name == "video" {
				add(attrs["poster"], true)
			}
			if name == "input" && strings.ToLower(attrs["type"]) != "image" { /* other input src rare; still fetch if present */
			}
		case "object":
			add(attrs["data"], true)
		}
		if srcset := attrs["srcset"]; srcset != "" {
			for _, candidate := range strings.Split(srcset, ",") {
				fields := strings.Fields(strings.TrimSpace(candidate))
				if len(fields) > 0 {
					add(fields[0], true)
				}
			}
		}
		if attrs["style"] != "" {
			for _, link := range extractCSSLinks(base, attrs["style"]) {
				add(link, true)
			}
		}
	}
	for _, link := range extractCSSLinks(base, body) {
		// CSS URLs inside inline style blocks and style attributes.
		add(link, true)
	}
	return pages, assets
}
func resourceExtension(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	path := strings.ToLower(u.Path)
	i := strings.LastIndexByte(path, '.')
	if i < 0 || i < strings.LastIndexByte(path, '/') {
		return false
	}
	for _, ext := range []string{".html", ".htm", ".shtml", ".php", ".asp", ".aspx", ".jsp", ".cfm", ".cgi", ".do"} {
		if strings.HasSuffix(path, ext) {
			return false
		}
	}
	return true
}
func resolveLink(base, raw string) string {
	raw = strings.TrimSpace(html.UnescapeString(raw))
	if raw == "" || strings.HasPrefix(raw, "#") {
		return ""
	}
	parent, err := url.Parse(base)
	if err != nil {
		return ""
	}
	rel, err := url.Parse(raw)
	if err != nil || (rel.Scheme != "" && rel.Scheme != "https" && rel.Scheme != "http" && rel.Scheme != "ftp") {
		return ""
	}
	joined := parent.ResolveReference(rel)
	canonical, err := canonicalURL(joined.String())
	if err != nil {
		return ""
	}
	return canonical
}
func extractCSSLinks(base, body string) []string {
	out, seen := []string{}, map[string]bool{}
	add := func(x string) {
		if v := resolveLink(base, x); v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	for _, m := range cssURL.FindAllStringSubmatch(body, 2000) {
		add(m[1])
	}
	for _, m := range cssImport.FindAllStringSubmatch(body, 500) {
		add(m[1])
	}
	return out
}
func extractJSLinks(base, body string) []string {
	out, seen := []string{}, map[string]bool{}
	for _, m := range jsImport.FindAllStringSubmatch(body, 500) {
		// Bare import specifiers (e.g. react) are not resolvable URLs.
		if !strings.HasPrefix(m[1], ".") && !strings.HasPrefix(m[1], "/") && !strings.Contains(m[1], "://") {
			continue
		}
		if v := resolveLink(base, m[1]); v != "" && !seen[v] {
			out = append(out, v)
			seen[v] = true
		}
	}
	return out
}

type matcher struct {
	pattern  string
	compiled *regexp.Regexp
}

func compileMatchers(c Config) ([]matcher, error) {
	if c.SearchMode == "files" {
		return nil, nil
	}
	var result []matcher
	for _, p := range strings.Split(c.Patterns, "\n") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		expression := p
		if !c.Regex {
			expression = regexp.QuoteMeta(p)
		}
		if !c.CaseSensitive {
			expression = "(?i)" + expression
		}
		compiled, err := regexp.Compile(expression)
		if err != nil {
			return nil, err
		}
		result = append(result, matcher{pattern: p, compiled: compiled})
	}
	return result, nil
}
func displaySlice(input string, limit int) string {
	if len(input) > limit {
		input = input[:limit] + "…"
	}
	input = strings.ToValidUTF8(input, "�")
	return strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return ' '
		}
		return r
	}, input)
}
func contextParts(content string, start, end int) (string, string, string) {
	if start < 0 || end > len(content) || start > end {
		return "", "", ""
	}
	left := start - 256
	if left < 0 {
		left = 0
	}
	right := end + 256
	if right > len(content) {
		right = len(content)
	}
	before := strings.Fields(displaySlice(content[left:start], 512))
	after := strings.Fields(displaySlice(content[end:right], 512))
	if len(before) > 8 {
		before = before[len(before)-8:]
	}
	if len(after) > 8 {
		after = after[:8]
	}
	b, a := strings.Join(before, " "), strings.Join(after, " ")
	matched := displaySlice(content[start:end], 256)
	return b, a, strings.TrimSpace(b + " " + matched + " " + a)
}
func findMatches(content string, ms []matcher, max int) []Match {
	out := []Match{}
	for _, m := range ms {
		indexes := m.compiled.FindAllStringIndex(content, max)
		for _, loc := range indexes {
			if len(out) >= max {
				return out
			}
			if loc[0] == loc[1] {
				continue
			}
			matched := displaySlice(content[loc[0]:loc[1]], 256)
			b, a, ctx := contextParts(content, loc[0], loc[1])
			out = append(out, Match{Pattern: m.pattern, Matched: matched, Before: b, After: a, Context: ctx})
		}
	}
	return out
}
