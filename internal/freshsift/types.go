package freshsift

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Config is the POST /api/start request. All limits are applied server-side.
type Config struct {
	Seeds               string   `json:"seeds"`
	Patterns            string   `json:"patterns"`
	Regex               bool     `json:"regex"`
	CaseSensitive       bool     `json:"caseSensitive"`
	SearchRawHTML       bool     `json:"searchRawHTML"` // legacy; searchMode supersedes it
	SearchMode          string   `json:"searchMode"`    // source (default), visible, files
	FileTypes           string   `json:"fileTypes"`     // required in files mode, e.g. .jpg, .db
	ExcludeFileTypes    string   `json:"excludeFileTypes"`
	AllowedTLDs         string   `json:"allowedTLDs"`
	EnableFTP           bool     `json:"enableFTP"`
	RecentHours         int      `json:"recentHours"`
	Depth               int      `json:"depth"`
	MaxPathSegments     int      `json:"maxPathSegments"`
	MaxExtraDirs        int      `json:"maxExtraDirs"`
	SiteMinutes         int      `json:"siteMinutes"`
	BudgetMode          string   `json:"budgetMode"`
	MaxPages            int      `json:"maxPages"`
	MaxPagesPerHost     int      `json:"maxPagesPerHost"`
	Concurrency         int      `json:"concurrency"`
	PerHostConcurrency  int      `json:"perHostConcurrency"`
	TimeoutSeconds      int      `json:"timeoutSeconds"`
	DiscoveryLimit      int      `json:"discoveryLimit"`
	EnumerateSubdomains bool     `json:"enumerateSubdomains"`
	SubdomainUseBuiltIn bool     `json:"subdomainUseBuiltIn"`
	SubdomainUseCT      bool     `json:"subdomainUseCT"`
	SubdomainGuess      *bool    `json:"subdomainGuess"`
	DiscoverDNSRecords  bool     `json:"discoverDNSRecords"`
	DNSOnly             bool     `json:"dnsOnly"`
	ReverseDNSPages     bool     `json:"reverseDNSPages"`
	ReverseDNSPageDepth int      `json:"reverseDNSPageDepth"`
	ReverseDNSSubdomains bool    `json:"reverseDNSSubdomains"`
	ReverseDNSSubdomainDepth int `json:"reverseDNSSubdomainDepth"`
	SubdomainWords      string   `json:"subdomainWords"`
	SubdomainLimit      int      `json:"subdomainLimit"`
	SourceQuery         string   `json:"sourceQuery"`
	Providers           []string `json:"providers"`
	FollowExternal      bool     `json:"followExternal"`
	RespectRobots       bool     `json:"respectRobots"`
	AllowPrivate        bool     `json:"allowPrivate"`
	AllowDomains        string   `json:"allowDomains"`
	ExcludeDomains      string   `json:"excludeDomains"`
	URLContains         string   `json:"urlContains"`
	URLExcludes         string   `json:"urlExcludes"`
	OnlyHTML            bool     `json:"onlyHTML"`
	Status2xxOnly       bool     `json:"status2xxOnly"`
}

func DefaultConfig() Config {
	return Config{
		RecentHours: 24, Depth: 1, MaxPathSegments: 5, SiteMinutes: 0,
		MaxPages: 1000, MaxPagesPerHost: 80, Concurrency: 24,
		PerHostConcurrency: 2, TimeoutSeconds: 12, DiscoveryLimit: 250, SubdomainLimit: 250, ReverseDNSPageDepth: 1, ReverseDNSSubdomainDepth: 1,
		RespectRobots: true, OnlyHTML: false, Status2xxOnly: true,
		SearchMode: "source", EnableFTP: true,
		Providers: []string{"urlscan", "hn"},
	}
}

func (c *Config) Normalize() error {
	if c.SearchMode == "" {
		c.SearchMode = "source"
	}
	if c.SearchMode != "source" && c.SearchMode != "visible" && c.SearchMode != "files" {
		return errors.New("search mode must be source, visible or files")
	}
	if c.SearchMode != "files" && strings.TrimSpace(c.Patterns) == "" && !c.EnumerateSubdomains && !c.DiscoverDNSRecords && !c.ReverseDNSPages {
		return errors.New("enter at least one search phrase")
	}
	if c.SearchMode == "files" && strings.TrimSpace(c.FileTypes) == "" && !c.DNSOnly {
		return errors.New("file discovery requires at least one extension, such as .jpg")
	}
	for _, spec := range []struct{ name, value string }{
		{"file types", c.FileTypes}, {"excluded file types", c.ExcludeFileTypes},
	} {
		if _, err := extensionList(spec.value); err != nil {
			return fmt.Errorf("%s: %w", spec.name, err)
		}
	}
	if _, err := tldList(c.AllowedTLDs); err != nil {
		return err
	}
	if c.RecentHours == 0 {
		c.RecentHours = 24
	}
	if c.RecentHours < 1 || c.RecentHours > 24*365 {
		return errors.New("age must be between 1 and 8760 hours")
	}
	if c.Depth < 0 || c.Depth > 8 {
		return errors.New("link depth must be between 0 and 8")
	}
	if c.MaxExtraDirs < 0 || c.MaxExtraDirs > 50 {
		return errors.New("extra directory depth must be 0 to 50 (0 means unlimited)")
	}
	if c.MaxPathSegments < 0 || c.MaxPathSegments > 50 {
		return errors.New("maximum URL path segments must be between 0 and 50")
	}
	if c.BudgetMode != "" && c.BudgetMode != "pages" && c.BudgetMode != "timer" {
		return errors.New("budget mode must be pages or timer")
	}
	if c.BudgetMode == "pages" {
		c.SiteMinutes = 0
	}
	if c.BudgetMode == "timer" {
		if c.SiteMinutes == 0 {
			return errors.New("timer mode requires minutes per site greater than zero")
		}
		c.Depth = 8
		c.MaxPathSegments = 50
		c.MaxExtraDirs = 0
		c.MaxPagesPerHost = 10000
	}
	if c.SiteMinutes < 0 || c.SiteMinutes > 1440 {
		return errors.New("per-site time limit must be 0 to 1440 minutes")
	}
	if c.MaxPages == 0 {
		c.MaxPages = 1000
	}
	if c.MaxPages < 1 || c.MaxPages > 100000 {
		return errors.New("maximum pages must be between 1 and 100000")
	}
	if c.MaxPagesPerHost == 0 {
		c.MaxPagesPerHost = 80
	}
	if c.MaxPagesPerHost < 1 || c.MaxPagesPerHost > 10000 {
		return errors.New("per-host page limit must be 1 to 10000")
	}
	if c.Concurrency == 0 {
		c.Concurrency = 24
	}
	if c.Concurrency < 1 || c.Concurrency > 200 {
		return errors.New("concurrency must be between 1 and 200")
	}
	if c.PerHostConcurrency == 0 {
		c.PerHostConcurrency = 2
	}
	if c.PerHostConcurrency < 1 || c.PerHostConcurrency > 10 {
		return errors.New("per-host concurrency must be between 1 and 10")
	}
	if c.TimeoutSeconds == 0 {
		c.TimeoutSeconds = 12
	}
	if c.TimeoutSeconds < 2 || c.TimeoutSeconds > 90 {
		return errors.New("request timeout must be 2 to 90 seconds")
	}
	if c.DiscoveryLimit == 0 {
		c.DiscoveryLimit = 250
	}
	if c.DiscoveryLimit < 1 || c.DiscoveryLimit > 2000 {
		return errors.New("discovery limit must be between 1 and 2000")
	}
	if c.SubdomainLimit == 0 {
		c.SubdomainLimit = 250
	}
	if c.SubdomainLimit < 1 || c.SubdomainLimit > 2000 {
		return errors.New("subdomain discovery limit must be between 1 and 2000")
	}
	if c.ReverseDNSPageDepth < 0 || c.ReverseDNSPageDepth > 8 || c.ReverseDNSSubdomainDepth < 0 || c.ReverseDNSSubdomainDepth > 8 {
		return errors.New("reverse DNS crawl depths must be between 0 and 8")
	}
	if c.ReverseDNSSubdomains && !c.EnumerateSubdomains {
		return errors.New("reverse DNS of subdomains requires subdomain enumeration")
	}
	if c.DNSOnly && !c.DiscoverDNSRecords {return errors.New("DNS-only mode requires DNS record lookup")}
	if c.DiscoverDNSRecords && strings.TrimSpace(c.Seeds) == "" {
		return errors.New("DNS record search requires at least one seed domain")
	}
	if c.SubdomainGuess==nil {
		enabled:=true // Older API clients without this new setting keep wordlist discovery.
		c.SubdomainGuess=&enabled
	}
	if c.EnumerateSubdomains {
		if strings.TrimSpace(c.Seeds) == "" {
			return errors.New("subdomain discovery requires at least one seed domain")
		}
		words := []string{}
		if *c.SubdomainGuess {
			var err error
			words, err = parseSubdomainWords(c.SubdomainWords, c.SubdomainUseBuiltIn)
			if err != nil {return err}
		}
		if len(words) == 0 && !c.SubdomainUseCT {
			return errors.New("enable the built-in subdomain list, enter words, or enable CT lookup")
		}
	}
	if len(c.Patterns) > 10000 {
		return errors.New("search patterns exceed 10000 characters")
	}
	if len(c.Seeds) > 200000 {
		return errors.New("seed list exceeds 200 KB")
	}
	if len(strings.Fields(c.Seeds)) > 10000 {
		return errors.New("too many seed URLs")
	}
	if len(c.SourceQuery) > 256 {
		return errors.New("discovery query must be 256 characters or fewer")
	}
	allowed := map[string]bool{"urlscan": true, "hn": true, "brave": true, "sitemap": true, "ct": true, "rss": true}
	for _, p := range c.Providers {
		if !allowed[p] {
			return errors.New("unknown discovery provider: " + p)
		}
	}
	if c.URLContains != "" && len(c.URLContains) > 256 {
		return errors.New("URL filter too long")
	}
	if c.URLExcludes != "" && len(c.URLExcludes) > 256 {
		return errors.New("URL exclusion too long")
	}
	if c.SearchMode != "files" {
		for _, p := range strings.Split(c.Patterns, "\n") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if c.Regex {
				expr := p
				if !c.CaseSensitive {
					expr = "(?i)" + expr
				}
				if _, err := regexp.Compile(expr); err != nil {
					return errors.New("invalid regular expression: " + err.Error())
				}
			}
		}
	}
	if strings.TrimSpace(c.Seeds) == "" && len(c.Providers) == 0 {
		return errors.New("provide seed URLs or choose at least one discovery source")
	}
	if len(c.Providers) == 1 && (c.Providers[0] == "sitemap" || c.Providers[0] == "ct") && strings.TrimSpace(c.Seeds) == "" {
		return errors.New("sitemap and CT discovery require at least one seed hostname")
	}
	return nil
}

type Candidate struct {
	URL              string    `json:"url"`
	Source           string    `json:"source"`
	ObservedAt       time.Time `json:"observedAt,omitempty"`
	IP               string    `json:"ip,omitempty"`
	BasePathSegments int       `json:"-"`
	Resource         bool      `json:"-"` // Embedded asset; does not use page-recursion depth.
	ReverseOrigin    string    `json:"-"`
	ReverseHost      string    `json:"-"`
	ReverseDepth     int       `json:"-"`
	Depth            int       `json:"depth"`
}

type Match struct {
	ID         int64     `json:"id"`
	URL        string    `json:"url"`
	Host       string    `json:"host"`
	IP         string    `json:"ip,omitempty"`
	Source     string    `json:"source"`
	Pattern    string    `json:"pattern"`
	Matched    string    `json:"matched"`
	Before     string    `json:"before"`
	After      string    `json:"after"`
	Context    string    `json:"context"`
	Area       string    `json:"area"`
	Kind       string    `json:"kind,omitempty"` // "file" or "text"
	FoundAt    time.Time `json:"foundAt"`
	ObservedAt time.Time `json:"observedAt,omitempty"`
	Status     int       `json:"status"`
}

type FoundURL struct {
	URL       string    `json:"url"`
	IP        string    `json:"ip,omitempty"`
	Source    string    `json:"source"`
	Count     int       `json:"count"`
	LastMatch time.Time `json:"lastMatch"`
}

type Stats struct {
	Status         string    `json:"status"`
	StartedAt      time.Time `json:"startedAt,omitempty"`
	FinishedAt     time.Time `json:"finishedAt,omitempty"`
	ElapsedSeconds float64   `json:"elapsedSeconds"`
	PagesScanned   int       `json:"pagesScanned"`
	PagesPerMinute float64   `json:"pagesPerMinute"`
	BytesScanned   int64     `json:"bytesScanned"`
	TextBytes      int64     `json:"textBytes"`
	TextMB         float64   `json:"textMB"`
	MBScanned      float64   `json:"mbScanned"`
	Matches        int       `json:"matches"`
	MatchedPages   int       `json:"matchedPages"`
	FilesFound     int       `json:"filesFound"`
	Discovered     int       `json:"discovered"`
	Subdomains     int       `json:"subdomains"`
	DNSRecords     int       `json:"dnsRecords"`
	ReverseHosts   int       `json:"reverseHosts"`
	Queued         int       `json:"queued"`
	Skipped        int       `json:"skipped"`
	TruncatedPages int       `json:"truncatedPages"`
	Errors         int       `json:"errors"`
	ActiveWorkers  int       `json:"activeWorkers"`
	LastURL        string    `json:"lastUrl"`
	LastError      string    `json:"lastError"`
}

type Snapshot struct {
	Stats      Stats       `json:"stats"`
	Matches    []Match     `json:"matches"`
	URLs       []FoundURL  `json:"urls"`
	Subdomains []Subdomain `json:"subdomains"`
	DNSRecords []DNSRecord `json:"dnsRecords"`
	ReverseDNS []ReverseHost `json:"reverseDNS"`
	Logs       []string    `json:"logs"`
}

func canonicalURL(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("empty URL")
	}
	if strings.HasPrefix(s, "#") {
		return "", errors.New("comment")
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "ftp" {
		return "", errors.New("only HTTP, HTTPS and FTP supported")
	}
	if u.Hostname() == "" || u.User != nil {
		return "", errors.New("missing hostname or URL contains credentials")
	}
	if strings.ContainsAny(u.Hostname(), " \t\r\n") {
		return "", errors.New("invalid hostname")
	}
	u.Fragment = ""
	u.Host = strings.ToLower(u.Host)
	if (u.Scheme == "https" && u.Port() == "443") || (u.Scheme == "http" && u.Port() == "80") || (u.Scheme == "ftp" && u.Port() == "21") {
		u.Host = u.Hostname()
	}
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String(), nil
}

func parseSeeds(s string) ([]Candidate, error) {
	seen := map[string]bool{}
	out := []Candidate{}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Also permit comma-separated URLs for pasted exports.
		for _, piece := range strings.Split(line, ",") {
			piece = strings.TrimSpace(piece)
			if piece == "" {
				continue
			}
			norm, err := canonicalURL(piece)
			if err != nil {
				return nil, errors.New("invalid seed " + piece + ": " + err.Error())
			}
			if !seen[norm] {
				seen[norm] = true
				out = append(out, Candidate{URL: norm, Source: "manual"})
			}
		}
	}
	return out, nil
}

func hostMatches(host, filter string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, x := range strings.FieldsFunc(strings.ToLower(filter), func(r rune) bool { return r == ',' || r == '\n' || r == ' ' }) {
		x = strings.TrimSpace(strings.TrimPrefix(x, "*."))
		if x != "" && (host == x || strings.HasSuffix(host, "."+x)) {
			return true
		}
	}
	return false
}
func pathSegments(raw string) int {
	u, err := url.Parse(raw)
	if err != nil {
		return 999
	}
	n := 0
	for _, x := range strings.Split(u.EscapedPath(), "/") {
		if x != "" {
			n++
		}
	}
	return n
}

// extensionList parses comma, whitespace or semicolon delimited file extensions.
// Compound suffixes such as .tar.gz are supported; URL query strings are ignored.
var validExtension = regexp.MustCompile(`^\.[a-z0-9][a-z0-9.+_-]{0,24}$`)

func extensionList(input string) ([]string, error) {
	if len(input) > 2048 {
		return nil, errors.New("extension list exceeds 2048 characters")
	}
	out := []string{}
	seen := map[string]bool{}
	for _, word := range strings.FieldsFunc(strings.ToLower(input), func(r rune) bool { return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\t' }) {
		word = strings.TrimSpace(word)
		if !strings.HasPrefix(word, ".") {
			word = "." + word
		}
		if !validExtension.MatchString(word) || strings.Contains(word, "..") {
			return nil, fmt.Errorf("invalid extension %q", word)
		}
		if !seen[word] {
			out = append(out, word)
			seen[word] = true
		}
	}
	return out, nil
}
func tldList(input string) ([]string, error) {
	if len(input) > 1024 {
		return nil, errors.New("TLD list exceeds 1024 characters")
	}
	out := []string{}
	for _, entry := range strings.FieldsFunc(strings.ToLower(input), func(r rune) bool { return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\t' }) {
		e := strings.TrimPrefix(strings.TrimSpace(entry), ".")
		if e == "" || len(e) > 100 {
			return nil, fmt.Errorf("invalid TLD %q", entry)
		}
		for _, part := range strings.Split(e, ".") {
			if len(part) == 0 || len(part) > 63 || part[0] == '-' || part[len(part)-1] == '-' {
				return nil, fmt.Errorf("invalid TLD %q", entry)
			}
			for _, r := range part {
				if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
					return nil, fmt.Errorf("invalid TLD %q", entry)
				}
			}
		}
		out = append(out, e)
	}
	return out, nil
}
func matchesTLD(host, filter string) bool {
	if strings.TrimSpace(filter) == "" {
		return true
	}
	if net.ParseIP(host) != nil {
		return false
	}
	labels, _ := tldList(filter)
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	for _, entry := range labels {
		if strings.HasSuffix(host, "."+entry) {
			return true
		}
	}
	return false
}
func matchesFileExtension(raw, list string) bool {
	if strings.TrimSpace(list) == "" {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	path := strings.ToLower(u.Path)
	extensions, _ := extensionList(list)
	for _, extension := range extensions {
		if strings.HasSuffix(path, extension) {
			return true
		}
	}
	return false
}
func fileExtension(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	p := u.Path
	i := strings.LastIndexByte(p, '.')
	j := strings.LastIndexByte(p, '/')
	if i <= j {
		return ""
	}
	return strings.ToLower(p[i:])
}

func selectedFileExtension(raw, list string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	path := strings.ToLower(u.Path)
	types, _ := extensionList(list)
	longest := ""
	for _, ext := range types {
		if strings.HasSuffix(path, ext) && len(ext) > len(longest) {
			longest = ext
		}
	}
	return longest
}
