package freshsift

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

type robotRule struct {
	pattern string
	allow   bool
}
type robotsRules struct{ rules []robotRule }

func parseRobots(input string) robotsRules {
	// Common allow/disallow syntax for * and our user-agent. Unsupported directives
	// are ignored; advanced robots patterns (*, $) are supported in matchRobots.
	s := bufio.NewScanner(strings.NewReader(input))
	s.Buffer(make([]byte, 4096), 128*1024)
	var rules robotsRules
	groupAgents := []string{}
	active := false
	seenRule := false
	for s.Scan() {
		line := strings.TrimSpace(strings.SplitN(s.Text(), "#", 2)[0])
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		if key == "user-agent" {
			if seenRule {
				groupAgents = nil
				seenRule = false
			}
			groupAgents = append(groupAgents, strings.ToLower(value))
			active = false
			for _, a := range groupAgents {
				if a == "*" || a == "freshsift" {
					active = true
				}
			}
		} else if key == "allow" || key == "disallow" {
			seenRule = true
			if active && value != "" {
				rules.rules = append(rules.rules, robotRule{value, key == "allow"})
			}
		}
	}
	return rules
}

func matchRobots(pattern, path string) bool {
	if pattern == "" {
		return false
	}
	// Wildcard matching with final $ end-anchor, preserving path case.
	anchored := strings.HasSuffix(pattern, "$")
	pattern = strings.TrimSuffix(pattern, "$")
	sections := strings.Split(pattern, "*")
	if !strings.HasPrefix(path, sections[0]) {
		return false
	}
	pos := len(sections[0])
	for _, part := range sections[1:] {
		if part == "" {
			continue
		}
		next := strings.Index(path[pos:], part)
		if next < 0 {
			return false
		}
		pos += next + len(part)
	}
	return !anchored || pos == len(path)
}
func (r robotsRules) allowed(path string) bool {
	matched := false
	ruleLength := -1
	allow := true
	for _, rule := range r.rules {
		if matchRobots(rule.pattern, path) {
			// Longest match wins; Allow wins an equal-length tie.
			l := len(rule.pattern)
			if l > ruleLength || l == ruleLength && rule.allow {
				matched = true
				ruleLength = l
				allow = rule.allow
			}
		}
	}
	return !matched || allow
}

type robotsCache struct{ items sync.Map }
type robotEntry struct {
	once  sync.Once
	rules robotsRules
}

func (c *robotsCache) allowed(ctx context.Context, client *http.Client, raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	key := u.Scheme + "://" + u.Host
	anyEntry, _ := c.items.LoadOrStore(key, &robotEntry{})
	entry := anyEntry.(*robotEntry)
	entry.once.Do(func() {
		req, e := http.NewRequestWithContext(ctx, "GET", key+"/robots.txt", nil)
		if e != nil {
			return
		}
		req.Header.Set("User-Agent", "FreshSift/0.1")
		res, e := client.Do(req)
		if e != nil {
			return
		}
		defer res.Body.Close()
		if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
			entry.rules = robotsRules{rules: []robotRule{{"/", false}}}
			return
		}
		if res.StatusCode != 200 {
			return
		}
		data, e := io.ReadAll(io.LimitReader(res.Body, 128*1024))
		if e != nil {
			return
		}
		entry.rules = parseRobots(string(data))
	})
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}
	return entry.rules.allowed(path)
}
