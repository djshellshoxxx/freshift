package freshsift

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// DNSRecord is an independent DNS observation; it is not proof that a web service exists.
type DNSRecord struct {
	Domain string `json:"domain"`
	Type   string `json:"type"`
	Value  string `json:"value"`
}

// ReverseHost is a forward-confirmed PTR hostname for a DNS-resolved address.
// PTR cannot enumerate every HTTP virtual host sharing an IP.
type ReverseHost struct {
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
	Origin   string `json:"origin"`
	Verified bool   `json:"verified"`
}

func queryDNSRecordsNative(ctx context.Context, domain string) []DNSRecord {
	if !validSubdomainRoot(domain) {
		return nil
	}
	resolver := net.DefaultResolver
	var mu sync.Mutex
	var wg sync.WaitGroup
	out := []DNSRecord{}
	add := func(kind, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if len(value) > 1024 {
			value = value[:1024]
		}
		mu.Lock()
		if len(out) < 250 {
			out = append(out, DNSRecord{Domain: domain, Type: kind, Value: value})
		}
		mu.Unlock()
	}
	task := func(fn func(context.Context)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			q, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			if q.Err() == nil {
				fn(q)
			}
		}()
	}
	task(func(q context.Context) {
		ips, err := resolver.LookupIPAddr(q, domain)
		if err == nil {
			for _, v := range ips {
				if v.IP.To4() != nil {
					add("A", v.IP.String())
				} else {
					add("AAAA", v.IP.String())
				}
			}
		}
	})
	task(func(q context.Context) {
		mx, err := resolver.LookupMX(q, domain)
		if err == nil {
			for _, v := range mx {
				add("MX", fmt.Sprintf("%d %s", v.Pref, strings.TrimSuffix(v.Host, ".")))
			}
		}
	})
	task(func(q context.Context) {
		ns, err := resolver.LookupNS(q, domain)
		if err == nil {
			for _, v := range ns {
				add("NS", strings.TrimSuffix(v.Host, "."))
			}
		}
	})
	task(func(q context.Context) {
		value, err := resolver.LookupCNAME(q, domain)
		if err == nil && strings.TrimSuffix(value, ".") != domain {
			add("CNAME", strings.TrimSuffix(value, "."))
		}
	})
	task(func(q context.Context) {
		txt, err := resolver.LookupTXT(q, domain)
		if err == nil {
			for _, v := range txt {
				add("TXT", v)
			}
		}
	})
	task(func(q context.Context) {
		_, srv, err := resolver.LookupSRV(q, "", "", domain)
		if err == nil {
			for _, v := range srv {
				add("SRV", fmt.Sprintf("%d %d %d %s", v.Priority, v.Weight, v.Port, strings.TrimSuffix(v.Target, ".")))
			}
		}
	})
	wg.Wait()
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		return out[i].Value < out[j].Value
	})
	return out
}

func dnsMatches(records []DNSRecord, patterns []matcher) []Match {
	out := []Match{}
	for _, row := range records {
		hits := findMatches(row.Value, patterns, 200)
		for _, hit := range hits {
			hit.Before = row.Type + " " + hit.Before
			hit.Context = row.Type + " " + row.Value
			out = append(out, hit)
		}
	}
	return out
}

type reverseLookup func(context.Context, string) ([]string, error)
type forwardLookup func(context.Context, string) ([]net.IP, error)

// verifiedPTRHosts strips malformed labels and accepts only names whose current
// forward DNS result contains the requested address.
func verifiedPTRHosts(ctx context.Context, ip net.IP, ptr reverseLookup, forward forwardLookup) []string {
	if ip == nil {
		return nil
	}
	q, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	values, err := ptr(q, ip.String())
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	out := []string{}
	for _, raw := range values {
		host := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(raw), "."))
		if !validSubdomainRoot(host) || seen[host] {
			continue
		}
		seen[host] = true
		fctx, stop := context.WithTimeout(q, 4*time.Second)
		addresses, lookupErr := forward(fctx, host)
		stop()
		if lookupErr != nil {
			continue
		}
		for _, a := range addresses {
			if a.Equal(ip) {
				out = append(out, host)
				break
			}
		}
		if len(out) >= 12 {
			break
		}
	}
	sort.Strings(out)
	return out
}

func reverseChildAllowed(c Config, item Candidate, child string, asset bool) bool {
	if item.ReverseOrigin == "" {
		if asset {
			return true
		}
		return item.Depth < c.Depth
	}
	if !asset && item.Depth >= item.ReverseDepth {
		return false
	}
	target, err := url.Parse(child)
	if err != nil || !strings.EqualFold(target.Hostname(), item.ReverseHost) {
		return false
	}
	return true
}

// reverseIP is scan-wide deduplicated and limited so shared IPs do not cause
// unbounded recursive PTR lookups. IPs are DNS-observed, not the HTTP peer IP.
func (e *Engine) reverseIP(ctx context.Context, c Config, ip net.IP, origin string, depth int) []Candidate {
	if !safeIP(ip, c.AllowPrivate) || ctx.Err() != nil {
		return nil
	}
	key := origin + "|" + ip.String()
	e.mu.Lock()
	if e.reverseSeen[key] || e.reverseQueries >= 64 {
		e.mu.Unlock()
		return nil
	}
	e.reverseSeen[key] = true
	e.reverseQueries++
	e.mu.Unlock()
	ptr := func(q context.Context, addr string) ([]string, error) { return net.DefaultResolver.LookupAddr(q, addr) }
	forward := func(q context.Context, name string) ([]net.IP, error) {
		return net.DefaultResolver.LookupIP(q, "ip", name)
	}
	names := verifiedPTRHosts(ctx, ip, ptr, forward)
	out := []Candidate{}
	for _, name := range names {
		record := ReverseHost{IP: ip.String(), Hostname: name, Origin: origin, Verified: true}
		e.mu.Lock()
		id := origin + "|" + ip.String() + "|" + name
		if _, known := e.reverseHosts[id]; !known {
			e.reverseHosts[id] = record
			e.stats.ReverseHosts = len(e.reverseHosts)
		}
		e.mu.Unlock()
		raw := "https://" + name + "/"
		if !candidateAllowed(c, raw) {
			continue
		}
		out = append(out, Candidate{URL: raw, Source: "reverse-ptr-" + origin, ReverseOrigin: origin, ReverseHost: name, ReverseDepth: depth, BasePathSegments: 0})
	}
	return out
}

func lookupPublicIPs(ctx context.Context, name string, allowPrivate bool) []net.IP {
	q, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIP(q, "ip", name)
	if err != nil {
		return nil
	}
	out := []net.IP{}
	for _, ip := range ips {
		if safeIP(ip, allowPrivate) {
			out = append(out, ip)
		}
	}
	return out
}

func differentIPs(base, candidate []net.IP) []net.IP {
	seen := map[string]bool{}
	for _, ip := range base {
		seen[ip.String()] = true
	}
	out := []net.IP{}
	for _, ip := range candidate {
		if !seen[ip.String()] {
			out = append(out, ip)
			seen[ip.String()] = true
		}
	}
	return out
}

func (e *Engine) reverseSubdomainHosts(ctx context.Context, c Config, seeds []Candidate) []Candidate {
	if !c.ReverseDNSSubdomains {
		return nil
	}
	roots := []string{}
	for _, seed := range seeds {
		parsed, err := url.Parse(seed.URL)
		if err != nil {
			continue
		}
		host := strings.ToLower(parsed.Hostname())
		if validSubdomainRoot(host) {
			roots = append(roots, host)
		}
	}
	rows := e.AllSubdomains()
	out := []Candidate{}
	checked := 0
	for _, row := range rows {
		if ctx.Err() != nil || checked >= 64 {
			break
		}
		root := ""
		for _, s := range roots {
			if row.Hostname != s && strings.HasSuffix(row.Hostname, "."+s) && len(s) > len(root) {
				root = s
			}
		}
		if root == "" {
			continue
		}
		checked++
		baseIPs := lookupPublicIPs(ctx, root, c.AllowPrivate)
		subIPs := lookupPublicIPs(ctx, row.Hostname, c.AllowPrivate)
		if len(baseIPs) == 0 || len(subIPs) == 0 {
			continue
		}
		for _, ip := range differentIPs(baseIPs, subIPs) {
			out = append(out, e.reverseIP(ctx, c, ip, "subdomain", c.ReverseDNSSubdomainDepth)...)
		}
	}
	return out
}
