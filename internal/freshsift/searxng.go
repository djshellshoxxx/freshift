package freshsift

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// discoverSearXNG retrieves search results through a configured SearXNG instance.
// Time-range filters are approximate; results are not proof of first indexing.
func discoverSearXNG(ctx context.Context, client *http.Client, c Config, log sourceLog) ([]Candidate, error) {
	query := strings.TrimSpace(c.SourceQuery)
	if query == "" && !c.Regex {
		query = strings.TrimSpace(strings.Split(c.Patterns, "\n")[0])
	}
	if query == "" {
		return nil, errors.New("SearXNG requires a plain-text discovery query")
	}
	if c.SearxngURL == "" || len(c.SearchEngines) == 0 {
		return nil, errors.New("SearXNG URL and search engines are required")
	}
	timeRange := "year"
	if c.RecentHours <= 24 {
		timeRange = "day"
	} else if c.RecentHours <= 720 {
		timeRange = "month"
	}
	out := []Candidate{}
	seen := map[string]bool{}
	for page := 1; page <= 5 && len(out) < c.DiscoveryLimit; page++ {
		params := url.Values{
			"q": {query},
			"format": {"json"},
			"pageno": {strconv.Itoa(page)},
			"engines": {strings.Join(c.SearchEngines, ",")},
			"time_range": {timeRange},
		}
		endpoint := strings.TrimRight(c.SearxngURL, "/") + "/search?" + params.Encode()
		var response struct {
			Results []struct {
				URL     string   `json:"url"`
				Engines []string `json:"engines"`
			} `json:"results"`
		}
		if err := discoveryRequest(ctx, client, endpoint, nil, &response); err != nil {
			return out, fmt.Errorf("SearXNG page %d: %w", page, err)
		}
		newCount := 0
		for _, result := range response.Results {
			clean, err := canonicalURL(result.URL)
			if err != nil || seen[clean] {
				continue
			}
			seen[clean] = true
			source := "searxng"
			for _, engine := range result.Engines {
				for _, selected := range c.SearchEngines {
					if engine == selected {
						source += "/" + engine
						break
					}
				}
				if source != "searxng" {
					break
				}
			}
			out = append(out, Candidate{URL: clean, Source: source})
			newCount++
			if len(out) >= c.DiscoveryLimit {
				break
			}
		}
		if len(response.Results) == 0 || newCount == 0 {
			break
		}
	}
	log(fmt.Sprintf("SearXNG: %d URLs from selected engines (freshness best-effort)", len(out)))
	return out, nil
}
