package freshsift

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

const MaxStoredMatches = 50000

type Engine struct {
	mu         sync.RWMutex
	stats      Stats
	matches    []Match
	urls       map[string]FoundURL
	subdomains map[string]Subdomain
	logs       []string
	cfg        Config
	cancel     context.CancelFunc
	paused     bool
	seq        int64
}

func NewEngine() *Engine {
	return &Engine{stats: Stats{Status: "idle"}, matches: []Match{}, urls: map[string]FoundURL{}, subdomains: map[string]Subdomain{}, logs: []string{}}
}
func (e *Engine) Start(c Config) error {
	if err := c.Normalize(); err != nil {
		return err
	}
	seeds, err := parseSeeds(c.Seeds)
	if err != nil {
		return err
	}
	ms, err := compileMatchers(c)
	if err != nil {
		return err
	}
	e.mu.Lock()
	if e.stats.Status == "running" || e.stats.Status == "discovering" || e.stats.Status == "paused" || e.stats.Status == "stopping" {
		e.mu.Unlock()
		return errors.New("a scan is already active")
	}
	ctx, cancel := context.WithCancel(context.Background())
	e.stats = Stats{Status: "discovering", StartedAt: time.Now()}
	e.matches = []Match{}
	e.urls = map[string]FoundURL{}
	e.subdomains = map[string]Subdomain{}
	e.logs = []string{}
	e.seq = 0
	e.cfg = c
	e.cancel = cancel
	e.paused = false
	e.mu.Unlock()
	go e.run(ctx, c, seeds, ms)
	return nil
}
func (e *Engine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cancel == nil || e.stats.Status == "idle" || e.stats.Status == "completed" || e.stats.Status == "cancelled" {
		return errors.New("no active scan")
	}
	e.stats.Status = "stopping"
	e.paused = false
	e.cancel()
	return nil
}
func (e *Engine) Pause() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.stats.Status != "running" {
		return errors.New("scan is not running")
	}
	e.paused = true
	e.stats.Status = "paused"
	return nil
}
func (e *Engine) Resume() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.stats.Status != "paused" {
		return errors.New("scan is not paused")
	}
	e.paused = false
	e.stats.Status = "running"
	return nil
}
func (e *Engine) awaitResume(ctx context.Context) bool {
	for {
		e.mu.RLock()
		paused := e.paused
		e.mu.RUnlock()
		if !paused {
			return ctx.Err() == nil
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(150 * time.Millisecond):
		}
	}
}
func (e *Engine) log(line string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.logs = append(e.logs, time.Now().Format("15:04:05")+"  "+line)
	if len(e.logs) > 80 {
		e.logs = e.logs[len(e.logs)-80:]
	}
}
func (e *Engine) err(err error) {
	if err == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stats.Errors++
	e.stats.LastError = err.Error()
	e.logs = append(e.logs, time.Now().Format("15:04:05")+"  ERROR  "+err.Error())
	if len(e.logs) > 80 {
		e.logs = e.logs[len(e.logs)-80:]
	}
}
func (e *Engine) Snapshot() Snapshot {
	e.mu.RLock()
	s := Snapshot{Stats: e.stats, Matches: append([]Match{}, e.matches...), Subdomains: []Subdomain{}, Logs: append([]string{}, e.logs...)}
	s.URLs = make([]FoundURL, 0, len(e.urls))
	for _, item := range e.urls {
		s.URLs = append(s.URLs, item)
	}
	for _, item := range e.subdomains {
		s.Subdomains = append(s.Subdomains, item)
	}
	e.mu.RUnlock()
	if !s.Stats.StartedAt.IsZero() {
		end := time.Now()
		if !s.Stats.FinishedAt.IsZero() {
			end = s.Stats.FinishedAt
		}
		s.Stats.ElapsedSeconds = end.Sub(s.Stats.StartedAt).Seconds()
		if s.Stats.ElapsedSeconds > 0 {
			s.Stats.PagesPerMinute = float64(s.Stats.PagesScanned) / s.Stats.ElapsedSeconds * 60
		}
	}
	s.Stats.MBScanned = float64(s.Stats.BytesScanned) / 1000000
	s.Stats.TextMB = float64(s.Stats.TextBytes) / 1000000
	sort.Slice(s.URLs, func(i, j int) bool { return s.URLs[i].LastMatch.After(s.URLs[j].LastMatch) })
	sort.Slice(s.Subdomains, func(i, j int) bool { return s.Subdomains[i].Hostname < s.Subdomains[j].Hostname })
	// Live updates stay bounded. Full results remain available via /api/export.
	if len(s.Matches) > 500 {
		s.Matches = s.Matches[len(s.Matches)-500:]
	}
	if len(s.URLs) > 500 {
		s.URLs = s.URLs[:500]
	}
	if len(s.Subdomains) > 500 {
		s.Subdomains = s.Subdomains[:500]
	}
	return s
}
func (e *Engine) AllResults() []Match {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return append([]Match{}, e.matches...)
}

func (e *Engine) recordSubdomain(row Subdomain) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if old, ok := e.subdomains[row.Hostname]; ok && old.Verified && !row.Verified {
		return // Never replace a DNS-confirmed hostname with a historical CT observation.
	}
	e.subdomains[row.Hostname] = row
	e.stats.Subdomains = len(e.subdomains)
}

func (e *Engine) AllSubdomains() []Subdomain {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]Subdomain, 0, len(e.subdomains))
	for _, row := range e.subdomains {
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Hostname < out[j].Hostname })
	return out
}

type taskQueue struct {
	mu          sync.Mutex
	cond        *sync.Cond
	items       []Candidate
	head        int
	outstanding int
	seen        map[string]bool
	hostCount   map[string]int
	maxPages    int
	perHost     int
	closed      bool
}

func newQueue(max, perHost int) *taskQueue {
	q := &taskQueue{seen: map[string]bool{}, hostCount: map[string]int{}, maxPages: max, perHost: perHost}
	q.cond = sync.NewCond(&q.mu)
	return q
}
func (q *taskQueue) add(c Candidate) bool {
	u, err := url.Parse(c.URL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed || q.seen[c.URL] || len(q.seen) >= q.maxPages || q.hostCount[host] >= q.perHost {
		return false
	}
	q.seen[c.URL] = true
	q.hostCount[host]++
	q.items = append(q.items, c)
	q.outstanding++
	q.cond.Signal()
	return true
}
func (q *taskQueue) next() (Candidate, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for q.head == len(q.items) && !q.closed && q.outstanding > 0 {
		q.cond.Wait()
	}
	if q.closed || q.outstanding == 0 {
		return Candidate{}, false
	}
	c := q.items[q.head]
	q.head++
	if q.head > 256 && q.head*2 >= len(q.items) {
		q.items = append([]Candidate{}, q.items[q.head:]...)
		q.head = 0
	}
	return c, true
}
func (q *taskQueue) done()        { q.mu.Lock(); q.outstanding--; q.cond.Broadcast(); q.mu.Unlock() }
func (q *taskQueue) close()       { q.mu.Lock(); q.closed = true; q.cond.Broadcast(); q.mu.Unlock() }
func (q *taskQueue) pending() int { q.mu.Lock(); defer q.mu.Unlock(); return q.outstanding }

type hostGate struct {
	token chan struct{}
	mu    sync.Mutex
	last  time.Time
	began time.Time
}
type hostGates struct {
	mu    sync.Mutex
	hosts map[string]*hostGate
	limit int
}

func (g *hostGates) get(host string) *hostGate {
	g.mu.Lock()
	defer g.mu.Unlock()
	h := g.hosts[host]
	if h == nil {
		h = &hostGate{token: make(chan struct{}, g.limit)}
		g.hosts[host] = h
	}
	return h
}
func (g *hostGate) acquire(ctx context.Context) bool {
	select {
	case g.token <- struct{}{}:
	case <-ctx.Done():
		return false
	}
	g.mu.Lock()
	remaining := time.Until(g.last.Add(250 * time.Millisecond))
	if remaining < 0 {
		remaining = 0
	}
	if remaining > 0 {
		select {
		case <-time.After(remaining):
		case <-ctx.Done():
			g.mu.Unlock()
			<-g.token
			return false
		}
	}
	g.last = time.Now()
	if g.began.IsZero() {
		g.began = time.Now()
	}
	g.mu.Unlock()
	return true
}
func (g *hostGate) release() { <-g.token }
func (g *hostGate) expired(minutes int) bool {
	if minutes <= 0 {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return !g.began.IsZero() && time.Since(g.began) >= time.Duration(minutes)*time.Minute
}

func (e *Engine) run(ctx context.Context, c Config, seeds []Candidate, ms []matcher) {
	defer func() {
		e.mu.Lock()
		e.stats.FinishedAt = time.Now()
		if ctx.Err() != nil {
			e.stats.Status = "cancelled"
		} else {
			e.stats.Status = "completed"
		}
		e.cancel = nil
		e.paused = false
		e.stats.ActiveWorkers = 0
		e.stats.Queued = 0
		e.mu.Unlock()
	}()
	client := newHTTPClient(time.Duration(c.TimeoutSeconds)*time.Second, c.AllowPrivate)
	discoveryClient := newHTTPClient(15*time.Second, false)
	sources := append([]Candidate{}, seeds...)
	log := func(line string) { e.log(line) }
	for _, provider := range c.Providers {
		if ctx.Err() != nil {
			return
		}
		var found []Candidate
		var err error
		e.log("Discovery source: " + provider)
		switch provider {
		case "urlscan":
			found, err = discoverURLScan(ctx, discoveryClient, c, log)
		case "hn":
			found, err = discoverHN(ctx, discoveryClient, c, log)
		case "brave":
			found, err = discoverBrave(ctx, discoveryClient, c, log)
		case "sitemap":
			found, err = discoverSitemaps(ctx, client, c, seeds, log)
		case "ct":
			found, err = discoverCT(ctx, discoveryClient, c, seeds, log)
		case "rss":
			found, err = discoverRSS(ctx, client, c, seeds, log)
		}
		if err != nil {
			e.err(err)
		}
		sources = append(sources, found...)
	}
	if ctx.Err() != nil {
		return
	}
	if c.EnumerateSubdomains {
		e.log("Subdomain discovery: DNS wordlist and historical CT names for seed domains")
		found, err := discoverSubdomains(ctx, c, seeds, discoveryClient, e.recordSubdomain, log)
		if err != nil {
			e.err(err)
		}
		sources = append(sources, found...)
	}
	if ctx.Err() != nil {
		return
	}
	q := newQueue(c.MaxPages, c.MaxPagesPerHost)
	for _, candidate := range sources {
		candidate.BasePathSegments = pathSegments(candidate.URL)
		if candidateAllowed(c, candidate.URL) && q.add(candidate) {
			e.mu.Lock()
			e.stats.Discovered++
			e.mu.Unlock()
		}
	}
	if q.pending() == 0 {
		e.err(errors.New("no candidate URLs: check seeds, filters, and discovery source quotas"))
		return
	}
	e.mu.Lock()
	e.stats.Status = "running"
	e.stats.Queued = q.pending()
	e.mu.Unlock()
	e.log(fmt.Sprintf("Starting %d workers for %d candidate URLs", c.Concurrency, q.pending()))
	var robots robotsCache
	gates := &hostGates{hosts: map[string]*hostGate{}, limit: c.PerHostConcurrency}
	// Cancellation also wakes any worker waiting for another worker to enqueue URLs.
	stopWatcher := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			q.close()
		case <-stopWatcher:
		}
	}()
	var wg sync.WaitGroup
	for i := 0; i < c.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				if !e.awaitResume(ctx) {
					return
				}
				item, ok := q.next()
				if !ok {
					return
				}
				e.mu.Lock()
				e.stats.Queued = q.pending() - 1
				e.stats.ActiveWorkers++
				e.mu.Unlock()
				e.process(ctx, c, ms, client, &robots, gates, q, item)
				q.done()
				e.mu.Lock()
				e.stats.ActiveWorkers--
				e.stats.Queued = q.pending()
				e.mu.Unlock()
			}
		}()
	}
	wg.Wait()
	close(stopWatcher)
}
func candidateAllowed(c Config, raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := u.Hostname()
	if host == "" {
		return false
	}
	if c.AllowedTLDs != "" && !matchesTLD(host, c.AllowedTLDs) {
		return false
	}
	if c.ExcludeFileTypes != "" && matchesFileExtension(raw, c.ExcludeFileTypes) {
		return false
	}
	if u.Scheme == "ftp" && !c.EnableFTP {
		return false
	}
	if c.AllowDomains != "" && !hostMatches(host, c.AllowDomains) {
		return false
	}
	if c.ExcludeDomains != "" && hostMatches(host, c.ExcludeDomains) {
		return false
	}
	if c.URLContains != "" && !strings.Contains(strings.ToLower(raw), strings.ToLower(c.URLContains)) {
		return false
	}
	if c.URLExcludes != "" && strings.Contains(strings.ToLower(raw), strings.ToLower(c.URLExcludes)) {
		return false
	}
	return pathSegments(raw) <= c.MaxPathSegments
}

// appendMatches stores both textual occurrences and verified file-URL discoveries.
func (e *Engine) appendMatches(item Candidate, res fetchResult, host string, hits []Match, area, kind string) {
	if len(hits) == 0 {
		return
	}
	foundAt := time.Now()
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.urls[res.URL]; !exists {
		e.stats.MatchedPages++
	}
	previous := e.urls[res.URL]
	previous.URL = res.URL
	previous.Source = item.Source
	previous.IP = item.IP
	previous.Count += len(hits)
	previous.LastMatch = foundAt
	e.urls[res.URL] = previous
	for _, m := range hits {
		e.seq++
		m.ID = e.seq
		m.URL = res.URL
		m.Host = host
		m.IP = item.IP
		m.Source = item.Source
		m.Area = area
		m.Kind = kind
		m.FoundAt = foundAt
		m.ObservedAt = item.ObservedAt
		m.Status = res.Status
		e.stats.Matches++
		if kind == "file" {
			e.stats.FilesFound++
		}
		if len(e.matches) < MaxStoredMatches {
			e.matches = append(e.matches, m)
		}
	}
}

func (e *Engine) process(ctx context.Context, c Config, ms []matcher, client *http.Client, robots *robotsCache, gates *hostGates, q *taskQueue, item Candidate) {
	u, err := url.Parse(item.URL)
	if err != nil {
		e.err(err)
		return
	}
	gate := gates.get(strings.ToLower(u.Hostname()))
	if gate.expired(c.SiteMinutes) {
		e.mu.Lock()
		e.stats.Skipped++
		e.mu.Unlock()
		return
	}
	if !gate.acquire(ctx) {
		return
	}
	defer gate.release()
	if gate.expired(c.SiteMinutes) {
		e.mu.Lock()
		e.stats.Skipped++
		e.mu.Unlock()
		return
	}
	if c.RespectRobots && u.Scheme != "ftp" && !robots.allowed(ctx, client, item.URL) {
		e.mu.Lock()
		e.stats.Skipped++
		e.mu.Unlock()
		return
	}
	var res fetchResult
	matchingFile := c.SearchMode == "files" && matchesFileExtension(item.URL, c.FileTypes)
	if c.SearchMode == "files" && item.Resource && !matchingFile && !discoveryTextResource(item.URL) {
		e.mu.Lock()
		e.stats.Skipped++
		e.mu.Unlock()
		return
	}
	if matchingFile {
		if u.Scheme == "ftp" {
			res, err = probeFTPFile(ctx, item.URL, c.AllowPrivate, time.Duration(c.TimeoutSeconds)*time.Second)
		}
		if u.Scheme != "ftp" {
			res, err = probeHTTPFile(ctx, client, item.URL)
		}
	} else {
		if u.Scheme == "ftp" {
			res, err = fetchFTP(ctx, item.URL, c.AllowPrivate, time.Duration(c.TimeoutSeconds)*time.Second)
		}
		if u.Scheme != "ftp" {
			res, err = fetchPage(ctx, client, item.URL)
		}
	}
	if err != nil {
		if ctx.Err() == nil {
			e.err(fmt.Errorf("%s: %w", item.URL, err))
		}
		return
	}
	// Redirects are subject to the same TLD, extension and domain restrictions.
	if !candidateAllowed(c, res.URL) {
		e.mu.Lock()
		e.stats.Skipped++
		e.mu.Unlock()
		return
	}
	e.mu.Lock()
	e.stats.PagesScanned++
	e.stats.BytesScanned += int64(res.Size)
	e.stats.LastURL = res.URL
	e.mu.Unlock()
	if c.Status2xxOnly && (res.Status < 200 || res.Status >= 300) {
		return
	}
	if matchingFile {
		// A file URL is counted only after HTTP/FTP confirmed it is reachable.
		// Preserve the discovered URL's extension even if its HEAD request
		// redirected to a download handler without a filename suffix.
		res.URL = item.URL
		ext := selectedFileExtension(item.URL, c.FileTypes)
		if ext != "" {
			name := item.URL
			if parsed, parseErr := url.Parse(item.URL); parseErr == nil {
				name = parsed.Path
			}
			e.appendMatches(item, res, u.Hostname(), []Match{{Pattern: ext, Matched: name, Before: "Discovered file:", After: "(verified available)", Context: "Discovered file: " + name + " (verified available)"}}, "File URL", "file")
		}
		return
	}
	if c.SearchMode == "source" && acceptableContentType(res.ContentType, c.OnlyHTML) {
		e.mu.Lock()
		e.stats.TextBytes += int64(len(res.Body))
		e.mu.Unlock()
		hits := findMatches(res.Body, ms, 200)
		if len(hits) == 200 {
			e.mu.Lock()
			e.stats.TruncatedPages++
			e.mu.Unlock()
			e.log("Match cap reached (200/resource): " + res.URL)
		}
		e.appendMatches(item, res, u.Hostname(), hits, "Raw source", "text")
	} else if c.SearchMode == "visible" && acceptableContentType(res.ContentType, true) && !res.IsDir {
		text := res.Body
		if strings.Contains(strings.ToLower(res.ContentType), "html") || strings.HasSuffix(strings.ToLower(u.Path), ".html") || strings.HasSuffix(strings.ToLower(u.Path), ".htm") || res.ContentType == "" {
			text = visibleText(text)
		}
		e.mu.Lock()
		e.stats.TextBytes += int64(len(text))
		e.mu.Unlock()
		hits := findMatches(text, ms, 200)
		if len(hits) == 200 {
			e.mu.Lock()
			e.stats.TruncatedPages++
			e.mu.Unlock()
		}
		e.appendMatches(item, res, u.Hostname(), hits, "Visible text", "text")
	}
	// Extract navigable pages and embedded resources separately. Resources such
	// as CSS, JS, images, downloads and FTP files are fetched at the current
	// depth, even when the page-recursion depth is zero.
	pages, assets := []string{}, []string{}
	if res.IsDir {
		for _, child := range res.Links {
			if strings.HasSuffix(child, "/") {
				pages = append(pages, child)
			} else {
				assets = append(assets, child)
			}
		}
	} else if strings.Contains(strings.ToLower(res.ContentType), "html") || strings.HasSuffix(strings.ToLower(u.Path), ".html") || strings.HasSuffix(strings.ToLower(u.Path), ".htm") {
		pages, assets = extractHTMLLinks(res.URL, res.Body)
	} else if strings.Contains(strings.ToLower(res.ContentType), "css") || strings.HasSuffix(strings.ToLower(u.Path), ".css") {
		assets = extractCSSLinks(res.URL, res.Body)
	} else if strings.Contains(strings.ToLower(res.ContentType), "javascript") || strings.HasSuffix(strings.ToLower(u.Path), ".js") || strings.HasSuffix(strings.ToLower(u.Path), ".mjs") {
		assets = extractJSLinks(res.URL, res.Body)
	}
	// Visible text mode only searches the requested pages, never linked assets.
	if c.SearchMode == "visible" {
		assets = nil
	}
	add := func(child string, resource bool) {
		target, parseErr := url.Parse(child)
		if parseErr != nil {
			return
		}
		if !resource && !c.FollowExternal && !strings.EqualFold(target.Host, u.Host) {
			return
		}
		if !candidateAllowed(c, child) {
			return
		}
		if c.MaxExtraDirs > 0 && pathSegments(child)-item.BasePathSegments > c.MaxExtraDirs {
			return
		}
		baseSegments := item.BasePathSegments
		if !strings.EqualFold(target.Host, u.Host) {
			baseSegments = pathSegments(child)
		}
		depth := item.Depth
		if !resource {
			depth++
		}
		if q.add(Candidate{URL: child, Source: "linked", Depth: depth, Resource: resource, BasePathSegments: baseSegments}) {
			e.mu.Lock()
			e.stats.Discovered++
			e.mu.Unlock()
		}
	}
	for _, asset := range assets {
		add(asset, true)
	}
	if item.Depth < c.Depth {
		for _, page := range pages {
			add(page, false)
		}
	}
}

// Non-target binary objects need not be downloaded to discover another file URL.
// HTML, XML, CSS, JS and text resources may contain additional references.
func discoveryTextResource(raw string) bool {
	ext := fileExtension(raw)
	if ext == "" {
		return true
	}
	switch ext {
	case ".html", ".htm", ".xhtml", ".shtml", ".php", ".asp", ".aspx", ".jsp", ".cfm", ".xml", ".css", ".js", ".mjs", ".json", ".txt", ".rss", ".atom", ".map", ".svg":
		return true
	}
	return false
}
