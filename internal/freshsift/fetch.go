package freshsift

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const MaxResponseBytes int64 = 8 << 20

// safeIP rejects non-public, internal, multicast and reserved destinations by default.
func safeIP(ip net.IP, allowPrivate bool) bool {
	if ip == nil || ip.IsUnspecified() || ip.IsMulticast() {
		return false
	}
	if allowPrivate {
		return true
	}
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false
	}
	// IP.IsGlobalUnicast still includes some special-use ranges.
	if v4 := ip.To4(); v4 != nil {
		blocked := []string{"0.0.0.0/8", "100.64.0.0/10", "169.254.0.0/16", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4"}
		for _, s := range blocked {
			_, subnet, _ := net.ParseCIDR(s)
			if subnet.Contains(ip) {
				return false
			}
		}
	} else {
		for _, s := range []string{"2001:db8::/32", "fc00::/7", "fe80::/10", "::ffff:0:0/96"} {
			_, subnet, _ := net.ParseCIDR(s)
			if subnet.Contains(ip) {
				return false
			}
		}
	}
	return true
}

func validatedTarget(ctx context.Context, host string, allowPrivate bool) (net.IP, error) {
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil {
		if !safeIP(ip, allowPrivate) {
			return nil, fmt.Errorf("private or reserved IP blocked: %s", host)
		}
		return ip, nil
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") || strings.HasSuffix(host, ".test") || strings.HasSuffix(host, ".invalid") {
		if !allowPrivate {
			return nil, errors.New("local or special-use hostname blocked")
		}
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, errors.New("no DNS addresses")
	}
	for _, entry := range ips {
		if !safeIP(entry.IP, allowPrivate) {
			return nil, fmt.Errorf("hostname %s resolves to a blocked address", host)
		}
	}
	for _, entry := range ips {
		if entry.IP.To4() != nil {
			return entry.IP, nil
		}
	}
	return ips[0].IP, nil
}

func newHTTPClient(timeout time.Duration, allowPrivate bool) *http.Client {
	dialer := &net.Dialer{Timeout: 8 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy: nil, MaxIdleConns: 128, MaxIdleConnsPerHost: 8, MaxConnsPerHost: 12,
		IdleConnTimeout: 60 * time.Second, TLSHandshakeTimeout: 8 * time.Second,
		ResponseHeaderTimeout: timeout, ForceAttemptHTTP2: true,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ip, err := validatedTarget(ctx, host, allowPrivate)
			if err != nil {
				return nil, err
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		},
	}
	return &http.Client{Timeout: timeout, Transport: transport, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many redirects")
		}
		if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
			return errors.New("unsupported redirect scheme")
		}
		if req.URL.User != nil || req.URL.Hostname() == "" {
			return errors.New("unsafe redirect")
		}
		return nil // transport validates every DNS resolution / target IP
	}}
}

type fetchResult struct {
	URL         string
	Body        string
	Status      int
	ContentType string
	Size        int
	IsDir       bool
	Links       []string
}

func fetchPage(ctx context.Context, client *http.Client, target string) (fetchResult, error) {
	var result fetchResult
	_, err := canonicalURL(target)
	if err != nil {
		return result, err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", target, nil)
	if err != nil {
		return result, err
	}
	req.Header.Set("User-Agent", "FreshSift/0.1 (+https://github.com/djshellshoxxx/FreshSift; respects robots.txt)")
	req.Header.Set("Accept", "text/html, text/plain;q=0.8, */*;q=0.1")
	res, err := client.Do(req)
	if err != nil {
		return result, err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, MaxResponseBytes+1))
	if err != nil {
		return result, err
	}
	if len(data) > int(MaxResponseBytes) {
		return result, fmt.Errorf("response exceeds %d MiB limit", MaxResponseBytes/(1<<20))
	}
	result = fetchResult{URL: res.Request.URL.String(), Body: string(data), Status: res.StatusCode, ContentType: res.Header.Get("Content-Type"), Size: len(data)}
	return result, nil
}

func acceptableContentType(ct string, onlyHTML bool) bool {
	if !onlyHTML {
		return true
	}
	ct = strings.ToLower(strings.Split(ct, ";")[0])
	ct = strings.TrimSpace(ct)
	return ct == "" || ct == "text/html" || ct == "application/xhtml+xml" || ct == "text/plain"
}

// safeAPIURL permits only an HTTPS URL at a fixed, trusted discovery host.
func safeAPIURL(raw string, host string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Hostname() == host && u.User == nil
}

// probeHTTPFile checks file availability without transferring large binaries.
// When HEAD is unsupported, a bounded range GET is used.
func probeHTTPFile(ctx context.Context, client *http.Client, target string) (fetchResult, error) {
	var result fetchResult
	if _, err := canonicalURL(target); err != nil {
		return result, err
	}
	for _, method := range []string{http.MethodHead, http.MethodGet} {
		req, err := http.NewRequestWithContext(ctx, method, target, nil)
		if err != nil {
			return result, err
		}
		req.Header.Set("User-Agent", "FreshSift/0.2")
		if method == http.MethodGet {
			req.Header.Set("Range", "bytes=0-0")
		}
		res, err := client.Do(req)
		if err != nil {
			return result, err
		}
		if method == http.MethodHead && (res.StatusCode == http.StatusMethodNotAllowed || res.StatusCode == http.StatusNotImplemented) {
			res.Body.Close()
			continue
		}
		if method == http.MethodGet {
			_, _ = io.CopyN(io.Discard, res.Body, 1)
		}
		res.Body.Close()
		result = fetchResult{URL: res.Request.URL.String(), Status: res.StatusCode, ContentType: res.Header.Get("Content-Type")}
		return result, nil
	}
	return result, errors.New("file probe failed")
}
