package freshsift

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type sourceLog func(string)

func discoveryRequest(ctx context.Context, client *http.Client, raw string, headers map[string]string, out any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", raw, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "FreshSift/0.1 (+open-source discovery client)")
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("HTTP %d%s", res.StatusCode, func() string {
			if res.StatusCode == 429 {
				return " (quota reached)"
			}
			return ""
		}())
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}
func discoverURLScan(ctx context.Context, client *http.Client, c Config, log sourceLog) ([]Candidate, error) {
	apiKey := os.Getenv("URLSCAN_API_KEY")
	limit := c.DiscoveryLimit
	result := []Candidate{}
	cursor := ""
	for len(result) < limit {
		size := limit - len(result)
		if size > 100 {
			size = 100
		}
		q := "date:>now-" + strconv.Itoa(c.RecentHours) + "h"
		params := url.Values{"q": {q}, "size": {strconv.Itoa(size)}}
		if cursor != "" {
			params.Set("search_after", cursor)
		}
		endpoint := "https://urlscan.io/api/v1/search/?" + params.Encode()
		var response struct {
			Results []struct {
				Page struct {
					URL string `json:"url"`
					IP  string `json:"ip"`
				} `json:"page"`
				Task struct {
					URL string `json:"url"`
				} `json:"task"`
				Date time.Time `json:"date"`
				Sort []any     `json:"sort"`
			} `json:"results"`
			HasMore bool `json:"has_more"`
		}
		head := map[string]string{}
		if apiKey != "" {
			head["api-key"] = apiKey
		}
		if err := discoveryRequest(ctx, client, endpoint, head, &response); err != nil {
			return result, fmt.Errorf("urlscan: %w", err)
		}
		if len(response.Results) == 0 {
			break
		}
		for _, r := range response.Results {
			raw := r.Page.URL
			if raw == "" {
				raw = r.Task.URL
			}
			if clean, e := canonicalURL(raw); e == nil {
				result = append(result, Candidate{URL: clean, Source: "urlscan", ObservedAt: r.Date, IP: r.Page.IP})
			}
		}
		last := response.Results[len(response.Results)-1]
		if !response.HasMore || len(last.Sort) == 0 {
			break
		}
		parts := []string{}
		for _, v := range last.Sort {
			parts = append(parts, fmt.Sprint(v))
		}
		cursor = strings.Join(parts, ",")
		if cursor == "" {
			break
		}
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(350 * time.Millisecond):
		}
	}
	log(fmt.Sprintf("urlscan: %d recently scanned URLs (not necessarily newly indexed)", len(result)))
	return result, nil
}
func discoverHN(ctx context.Context, client *http.Client, c Config, log sourceLog) ([]Candidate, error) {
	lower := time.Now().Add(-time.Duration(c.RecentHours) * time.Hour).Unix()
	result := []Candidate{}
	pages := (c.DiscoveryLimit + 49) / 50
	if pages > 8 {
		pages = 8
	}
	for page := 0; page < pages; page++ {
		params := url.Values{"tags": {"story"}, "hitsPerPage": {"50"}, "page": {strconv.Itoa(page)}, "numericFilters": {"created_at_i>" + strconv.FormatInt(lower, 10)}}
		if strings.TrimSpace(c.SourceQuery) != "" {
			params.Set("query", c.SourceQuery)
		}
		endpoint := "https://hn.algolia.com/api/v1/search_by_date?" + params.Encode()
		var response struct {
			Hits []struct {
				URL       string    `json:"url"`
				CreatedAt time.Time `json:"created_at"`
			} `json:"hits"`
		}
		if err := discoveryRequest(ctx, client, endpoint, nil, &response); err != nil {
			return result, fmt.Errorf("Hacker News: %w", err)
		}
		for _, h := range response.Hits {
			if clean, e := canonicalURL(h.URL); e == nil {
				result = append(result, Candidate{URL: clean, Source: "hn", ObservedAt: h.CreatedAt})
			}
			if len(result) >= c.DiscoveryLimit {
				break
			}
		}
		if len(response.Hits) < 50 || len(result) >= c.DiscoveryLimit {
			break
		}
	}
	log(fmt.Sprintf("Hacker News: %d recently submitted URLs", len(result)))
	return result, nil
}
func discoverBrave(ctx context.Context, client *http.Client, c Config, log sourceLog) ([]Candidate, error) {
	key := os.Getenv("BRAVE_SEARCH_API_KEY")
	if key == "" {
		return nil, errors.New("BRAVE_SEARCH_API_KEY not set (Brave provider skipped)")
	}
	query := strings.TrimSpace(c.SourceQuery)
	if query == "" {
		if c.Regex {
			return nil, errors.New("Brave needs a plain-text discovery query when regex is enabled")
		}
		query = strings.TrimSpace(strings.Split(c.Patterns, "\n")[0])
	}
	if query == "" {
		return nil, errors.New("Brave search query is empty")
	}
	result := []Candidate{}
	for offset := 0; offset < 2 && len(result) < c.DiscoveryLimit; offset++ {
		params := url.Values{"q": {query}, "count": {"20"}, "offset": {strconv.Itoa(offset)}, "freshness": {"pd"}}
		endpoint := "https://api.search.brave.com/res/v1/web/search?" + params.Encode()
		var response struct {
			Web struct {
				Results []struct {
					URL string `json:"url"`
				} `json:"results"`
			} `json:"web"`
		}
		if err := discoveryRequest(ctx, client, endpoint, map[string]string{"X-Subscription-Token": key}, &response); err != nil {
			return result, fmt.Errorf("Brave: %w", err)
		}
		for _, r := range response.Web.Results {
			if clean, e := canonicalURL(r.URL); e == nil {
				result = append(result, Candidate{URL: clean, Source: "brave"})
			}
		}
		if len(response.Web.Results) < 20 {
			break
		}
	}
	log(fmt.Sprintf("Brave: %d URLs returned using past-day freshness filter", len(result)))
	return result, nil
}
func discoverCT(ctx context.Context, client *http.Client, c Config, seeds []Candidate, log sourceLog) ([]Candidate, error) {
	// This discovers recently LOGGED TLS certificate names under supplied domains.
	// It does not enumerate the entire internet and it does not prove site creation.
	visited := map[string]bool{}
	out := []Candidate{}
	for _, seed := range seeds {
		u, _ := url.Parse(seed.URL)
		host := u.Hostname()
		if visited[host] || host == "" || strings.Contains(host, ":") {
			continue
		}
		visited[host] = true
		if len(visited) > 5 {
			break
		}
		q := "%." + host
		endpoint := "https://crt.sh/?q=" + url.QueryEscape(q) + "&output=json"
		var entries []struct {
			Name      string `json:"name_value"`
			Timestamp string `json:"entry_timestamp"`
		}
		if err := discoveryRequest(ctx, client, endpoint, nil, &entries); err != nil {
			log("CT " + host + ": " + err.Error())
			continue
		}
		for _, entry := range entries {
			seen, e := time.Parse("2006-01-02T15:04:05.999999999", entry.Timestamp)
			if e != nil {
				seen, e = time.Parse("2006-01-02T15:04:05", entry.Timestamp)
			}
			if e != nil || time.Since(seen) > time.Duration(c.RecentHours)*time.Hour {
				continue
			}
			for _, name := range strings.Split(entry.Name, "\n") {
				name = strings.TrimSpace(strings.TrimPrefix(name, "*."))
				if name == "" || (!strings.EqualFold(name, host) && !strings.HasSuffix(strings.ToLower(name), "."+strings.ToLower(host))) {
					continue
				}
				clean, e := canonicalURL("https://" + name)
				if e == nil {
					out = append(out, Candidate{URL: clean, Source: "ct", ObservedAt: seen})
				}
				if len(out) >= c.DiscoveryLimit {
					break
				}
			}
			if len(out) >= c.DiscoveryLimit {
				break
			}
		}
		if len(out) >= c.DiscoveryLimit {
			break
		}
	}
	log(fmt.Sprintf("CT: %d recently logged certificate hostnames under supplied seeds", len(out)))
	return out, nil
}

type siteMap struct {
	URLs []struct {
		Loc     string `xml:"loc"`
		LastMod string `xml:"lastmod"`
	} `xml:"url"`
	Index []struct {
		Loc     string `xml:"loc"`
		LastMod string `xml:"lastmod"`
	} `xml:"sitemap"`
}

func discoverSitemaps(ctx context.Context, client *http.Client, c Config, seeds []Candidate, log sourceLog) ([]Candidate, error) {
	seen := map[string]bool{}
	out := []Candidate{}
	for _, seed := range seeds {
		if len(out) >= c.DiscoveryLimit {
			break
		}
		u, e := url.Parse(seed.URL)
		if e != nil {
			continue
		}
		base := u.Scheme + "://" + u.Host
		if seen[base] {
			continue
		}
		seen[base] = true
		queue := []string{base + "/sitemap.xml"}
		scanned := 0
		for len(queue) > 0 && scanned < 4 && len(out) < c.DiscoveryLimit {
			current := queue[0]
			queue = queue[1:]
			scanned++
			res, e := fetchPage(ctx, client, current)
			if e != nil || res.Status != 200 {
				continue
			}
			var parsed siteMap
			if e = xml.Unmarshal([]byte(res.Body), &parsed); e != nil {
				continue
			}
			for _, row := range parsed.URLs {
				if row.LastMod != "" {
					last, er := parseSiteTime(row.LastMod)
					if er == nil && time.Since(last) > time.Duration(c.RecentHours)*time.Hour {
						continue
					}
				}
				clean, er := canonicalURL(row.Loc)
				if er == nil {
					out = append(out, Candidate{URL: clean, Source: "sitemap"})
				}
				if len(out) >= c.DiscoveryLimit {
					break
				}
			}
			for _, row := range parsed.Index {
				if len(queue) >= 6 {
					break
				}
				next, er := url.Parse(row.Loc)
				if er == nil && next.Host == u.Host && next.Scheme == u.Scheme {
					queue = append(queue, next.String())
				}
			}
		}
	}
	log(fmt.Sprintf("Sitemaps: %d URLs (lastmod filtered when available)", len(out)))
	return out, nil
}
func parseSiteTime(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02", "2006-01-02T15:04:05"} {
		if t, e := time.Parse(layout, strings.TrimSpace(s)); e == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("unknown sitemap lastmod time format")
}

// discoverRSS reads RSS/Atom feeds under supplied seed sites. Feed publish dates
// are publication observations, not search-engine indexing times.
func discoverRSS(ctx context.Context, client *http.Client, c Config, seeds []Candidate, log sourceLog) ([]Candidate, error) {
	type rssItem struct {
		Link    string `xml:"link"`
		PubDate string `xml:"pubDate"`
	}
	type atomItem struct {
		Links []struct {
			Href string `xml:"href,attr"`
			Rel  string `xml:"rel,attr"`
		} `xml:"link"`
		Updated   string `xml:"updated"`
		Published string `xml:"published"`
	}
	type feed struct {
		Items   []rssItem  `xml:"channel>item"`
		Entries []atomItem `xml:"entry"`
	}
	seen := map[string]bool{}
	out := []Candidate{}
	cutoff := time.Now().Add(-time.Duration(c.RecentHours) * time.Hour)
	for _, seed := range seeds {
		u, e := url.Parse(seed.URL)
		if e != nil {
			continue
		}
		base := u.Scheme + "://" + u.Host
		if seen[base] {
			continue
		}
		seen[base] = true
		if len(seen) > 6 {
			break
		}
		candidates := []string{base + "/feed.xml", base + "/rss.xml", base + "/feed", base + "/atom.xml"}
		if strings.HasSuffix(u.Path, ".xml") || strings.Contains(u.Path, "/feed") {
			candidates = append([]string{seed.URL}, candidates...)
		}
		for _, endpoint := range candidates {
			res, err := fetchPage(ctx, client, endpoint)
			if err != nil || res.Status != 200 {
				continue
			}
			var parsed feed
			if err := xml.Unmarshal([]byte(res.Body), &parsed); err != nil {
				continue
			}
			for _, item := range parsed.Items {
				observed, _ := parseFeedTime(item.PubDate)
				if !observed.IsZero() && observed.Before(cutoff) {
					continue
				}
				if clean, e := canonicalURL(item.Link); e == nil {
					out = append(out, Candidate{URL: clean, Source: "rss", ObservedAt: observed})
				}
			}
			for _, item := range parsed.Entries {
				observed, _ := parseFeedTime(item.Published)
				if observed.IsZero() {
					observed, _ = parseFeedTime(item.Updated)
				}
				if !observed.IsZero() && observed.Before(cutoff) {
					continue
				}
				for _, link := range item.Links {
					if link.Rel != "" && link.Rel != "alternate" {
						continue
					}
					if clean, e := canonicalURL(link.Href); e == nil {
						out = append(out, Candidate{URL: clean, Source: "rss", ObservedAt: observed})
					}
					break
				}
			}
			if len(out) >= c.DiscoveryLimit {
				break
			}
			if len(parsed.Items) > 0 || len(parsed.Entries) > 0 {
				break
			}
		}
		if len(out) >= c.DiscoveryLimit {
			break
		}
	}
	log(fmt.Sprintf("RSS/Atom: %d recent feed-entry URLs (date filtered when available)", len(out)))
	return out, nil
}
func parseFeedTime(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC1123Z, time.RFC1123, time.RFC3339, "Mon, 02 Jan 2006 15:04:05 -0700", "2006-01-02T15:04:05Z"} {
		if result, e := time.Parse(layout, strings.TrimSpace(s)); e == nil {
			return result, nil
		}
	}
	return time.Time{}, errors.New("unknown feed date format")
}
