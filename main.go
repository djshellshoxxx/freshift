package main

import (
	"bytes"
	"crypto/subtle"
	"embed"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/djshellshoxxx/freshift/internal/freshsift"
)

//go:embed web/*
var assets embed.FS

const Version = "0.5.0"

// remoteSettings is only needed when a static GitHub Pages dashboard controls
// an externally hosted Go engine through a HTTPS reverse proxy.
type remoteSettings struct {
	AllowedOrigin string
	APIToken      string
}

func validateAllowedOrigin(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u == nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("FRESHSIFT_ALLOWED_ORIGIN must be a bare HTTPS origin (example: https://djshellshoxxx.github.io)")
	}
	return nil
}

func main() {
	port := flag.Int("port", 8787, "listening port")
	listenHost := flag.String("listen", "127.0.0.1", "listening address (default: loopback)")
	noBrowser := flag.Bool("no-browser", false, "do not open browser automatically")
	stateFile := flag.String("state-file", "", "perpetual scan restart checkpoint (default: user config directory)")
	flag.Parse()
	if *port < 1 || *port > 65535 {
		log.Fatal("port must be between 1 and 65535")
	}
	settings := remoteSettings{AllowedOrigin: strings.TrimRight(strings.TrimSpace(os.Getenv("FRESHSIFT_ALLOWED_ORIGIN")), "/"), APIToken: os.Getenv("FRESHSIFT_API_TOKEN")}
	if settings.AllowedOrigin != "" {
		if err := validateAllowedOrigin(settings.AllowedOrigin); err != nil {
			log.Fatal(err)
		}
	}
	if ip := net.ParseIP(*listenHost); ip == nil || !ip.IsLoopback() {
		if settings.APIToken == "" || settings.AllowedOrigin == "" {
			log.Fatal("non-loopback listening requires FRESHSIFT_API_TOKEN and FRESHSIFT_ALLOWED_ORIGIN")
		}
	}
	address := net.JoinHostPort(*listenHost, strconv.Itoa(*port))
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	endpoint := "http://" + address
	log.Printf("FreshSift v%s — %s", Version, endpoint)
	if !*noBrowser {
		go func() { time.Sleep(400 * time.Millisecond); openBrowser(endpoint) }()
	}
	if *stateFile == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			log.Fatal("cannot determine user config directory: ", err)
		}
		*stateFile = filepath.Join(configDir, "FreshSift", "perpetual.json")
	}
	engine := freshsift.NewEngine()
	engine.SetStateFile(*stateFile)
	if resumed, err := engine.ResumeSavedScan(); err != nil {
		log.Printf("Perpetual scan restore failed: %v", err)
	} else if resumed {
		log.Printf("Resumed saved perpetual scan")
	}
	app := &http.Server{Handler: newMuxWithSettings(engine, settings), ReadHeaderTimeout: 6 * time.Second, IdleTimeout: 60 * time.Second}
	log.Fatal(app.Serve(listener))
}
func openBrowser(endpoint string) {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", endpoint)
	case "darwin":
		command = exec.Command("open", endpoint)
	default:
		command = exec.Command("xdg-open", endpoint)
	}
	_ = command.Start()
}

func newMux(engine *freshsift.Engine) http.Handler {
	return newMuxWithSettings(engine, remoteSettings{})
}

func newMuxWithSettings(engine *freshsift.Engine, settings remoteSettings) http.Handler {
	mux := http.NewServeMux()
	sub, err := fs.Sub(assets, "web")
	if err != nil {
		panic(err)
	}
	static := http.FileServer(http.FS(sub))
	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) { jsonResponse(w, 200, engine.Snapshot()) })
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, map[string]string{"status": "ok", "version": Version})
	})
	mux.HandleFunc("GET /api/version", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, map[string]string{"version": Version, "build": "local"})
	})
	mux.HandleFunc("POST /api/start", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var cfg freshsift.Config
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&cfg); err != nil {
			jsonError(w, 400, "INVALID_REQUEST", err.Error())
			return
		}
		if err := engine.Start(cfg); err != nil {
			jsonError(w, 400, "INVALID_CONFIG", err.Error())
			return
		}
		jsonResponse(w, 202, map[string]string{"status": "accepted"})
	})
	mux.HandleFunc("POST /api/stop", func(w http.ResponseWriter, r *http.Request) {
		if err := engine.Stop(); err != nil {
			jsonError(w, 409, "NOT_RUNNING", err.Error())
			return
		}
		jsonResponse(w, 200, map[string]string{"status": "stopping"})
	})
	mux.HandleFunc("POST /api/pause", func(w http.ResponseWriter, r *http.Request) {
		if err := engine.Pause(); err != nil {
			jsonError(w, 409, "NOT_RUNNING", err.Error())
			return
		}
		jsonResponse(w, 200, map[string]string{"status": "paused"})
	})
	mux.HandleFunc("POST /api/resume", func(w http.ResponseWriter, r *http.Request) {
		if err := engine.Resume(); err != nil {
			jsonError(w, 409, "NOT_PAUSED", err.Error())
			return
		}
		jsonResponse(w, 200, map[string]string{"status": "running"})
	})
	mux.HandleFunc("GET /api/export", func(w http.ResponseWriter, r *http.Request) { exportResults(w, r, engine.AllResults()) })
	mux.HandleFunc("GET /api/subdomains", func(w http.ResponseWriter, r *http.Request) { exportSubdomains(w, r, engine.AllSubdomains()) })
	mux.HandleFunc("GET /api/dns-records", func(w http.ResponseWriter, r *http.Request) { exportDNSRecords(w, r, engine.AllDNSRecords()) })
	mux.HandleFunc("GET /api/reverse-dns", func(w http.ResponseWriter, r *http.Request) { exportReverseHosts(w, r, engine.AllReverseDNS()) })
	mux.HandleFunc("GET /api/ip", ipLookup)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			jsonError(w, 404, "NOT_FOUND", "unknown API route")
			return
		}
		static.ServeHTTP(w, r)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; font-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			origin := r.Header.Get("Origin")
			if origin != "" {
				parsed, e := url.Parse(origin)
				sameOrigin := e == nil && parsed.Host == r.Host && (parsed.Scheme == "http" || parsed.Scheme == "https")
				trustedRemote := settings.AllowedOrigin != "" && origin == settings.AllowedOrigin
				if !sameOrigin && !trustedRemote {
					jsonError(w, 403, "INVALID_ORIGIN", "cross-origin request blocked")
					return
				}
				if trustedRemote {
					w.Header().Set("Access-Control-Allow-Origin", settings.AllowedOrigin)
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
					w.Header().Set("Access-Control-Max-Age", "600")
					w.Header().Set("Vary", "Origin")
				}
			}
			if r.Method == http.MethodOptions {
				if origin == "" {
					jsonError(w, 403, "INVALID_ORIGIN", "origin required")
					return
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
			if settings.APIToken != "" {
				bearer := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
				if len(bearer) != len(settings.APIToken) || subtle.ConstantTimeCompare([]byte(bearer), []byte(settings.APIToken)) != 1 {
					jsonError(w, 401, "UNAUTHORIZED", "crawler API token missing or invalid")
					return
				}
			}
		}
		mux.ServeHTTP(w, r)
	})
}
func jsonResponse(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
func jsonError(w http.ResponseWriter, status int, code, msg string) {
	jsonResponse(w, status, map[string]any{"error": map[string]string{"code": code, "message": msg}})
}

func csvSafe(s string) string {
	if len(s) > 0 && strings.ContainsAny(s[:1], "=+-@\t\r\n") {
		return "'" + s
	}
	return s
}
func exportResults(w http.ResponseWriter, r *http.Request, matches []freshsift.Match) {
	format := r.URL.Query().Get("format")
	switch format {
	case "json":
		w.Header().Set("Content-Disposition", `attachment; filename="freshsift-results.json"`)
		jsonResponse(w, 200, matches)
	case "jsonl":
		w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="freshsift-results.jsonl"`)
		for _, m := range matches {
			_ = json.NewEncoder(w).Encode(m)
		}
	case "csv", "":
		var b bytes.Buffer
		writer := csv.NewWriter(&b)
		_ = writer.Write([]string{"URL", "IP", "Source", "Observed At", "Found At", "HTTP Status", "Pattern", "Matched String", "Before", "After", "Context", "Area", "Kind"})
		for _, m := range matches {
			observed := ""
			if !m.ObservedAt.IsZero() {
				observed = m.ObservedAt.Format(time.RFC3339)
			}
			_ = writer.Write([]string{csvSafe(m.URL), csvSafe(m.IP), csvSafe(m.Source), observed, m.FoundAt.Format(time.RFC3339), strconv.Itoa(m.Status), csvSafe(m.Pattern), csvSafe(m.Matched), csvSafe(m.Before), csvSafe(m.After), csvSafe(m.Context), csvSafe(m.Area), csvSafe(m.Kind)})
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			jsonError(w, 500, "EXPORT_FAILED", err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="freshsift-results.csv"`)
		_, _ = w.Write(b.Bytes())
	default:
		jsonError(w, 400, "INVALID_FORMAT", "supported formats: csv, json, jsonl")
	}
}

func exportSubdomains(w http.ResponseWriter, r *http.Request, rows []freshsift.Subdomain) {
	format := r.URL.Query().Get("format")
	switch format {
	case "", "csv":
		var buffer bytes.Buffer
		writer := csv.NewWriter(&buffer)
		_ = writer.Write([]string{"Hostname", "Source", "DNS Verified", "Addresses"})
		for _, item := range rows {
			_ = writer.Write([]string{csvSafe(item.Hostname), csvSafe(item.Source), strconv.FormatBool(item.Verified), csvSafe(strings.Join(item.Addresses, "; "))})
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			jsonError(w, 500, "EXPORT_FAILED", err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="freshsift-subdomains.csv"`)
		_, _ = w.Write(buffer.Bytes())
	case "json":
		w.Header().Set("Content-Disposition", `attachment; filename="freshsift-subdomains.json"`)
		jsonResponse(w, 200, rows)
	case "txt":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="freshsift-subdomains.txt"`)
		for _, item := range rows {
			_, _ = fmt.Fprintln(w, item.Hostname)
		}
	default:
		jsonError(w, 400, "INVALID_FORMAT", "supported subdomain formats: csv, json, txt")
	}
}

func exportDNSRecords(w http.ResponseWriter, r *http.Request, rows []freshsift.DNSRecord) {
	format := r.URL.Query().Get("format")
	if format == "" || format == "json" {
		jsonResponse(w, 200, rows)
		return
	}
	if format != "csv" {
		jsonError(w, 400, "INVALID_FORMAT", "supported formats: json, csv")
		return
	}
	var buf bytes.Buffer
	cw := csv.NewWriter(&buf)
	_ = cw.Write([]string{"Domain", "Type", "Value"})
	for _, row := range rows {
		_ = cw.Write([]string{csvSafe(row.Domain), csvSafe(row.Type), csvSafe(row.Value)})
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		jsonError(w, 500, "EXPORT_FAILED", err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="freshsift-dns-records.csv"`)
	_, _ = w.Write(buf.Bytes())
}

func exportReverseHosts(w http.ResponseWriter, r *http.Request, rows []freshsift.ReverseHost) {
	format := r.URL.Query().Get("format")
	if format == "" || format == "json" {
		jsonResponse(w, 200, rows)
		return
	}
	if format != "csv" {
		jsonError(w, 400, "INVALID_FORMAT", "supported formats: json, csv")
		return
	}
	var buf bytes.Buffer
	cw := csv.NewWriter(&buf)
	_ = cw.Write([]string{"IP", "PTR Hostname", "Origin", "Forward Verified"})
	for _, row := range rows {
		_ = cw.Write([]string{csvSafe(row.IP), csvSafe(row.Hostname), csvSafe(row.Origin), strconv.FormatBool(row.Verified)})
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		jsonError(w, 500, "EXPORT_FAILED", err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="freshsift-reverse-dns.csv"`)
	_, _ = w.Write(buf.Bytes())
}

func ipLookup(w http.ResponseWriter, r *http.Request) {
	ip := net.ParseIP(strings.TrimSpace(r.URL.Query().Get("ip")))
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		jsonError(w, 400, "INVALID_IP", "enter a public IP address")
		return
	}
	u := "https://internetdb.shodan.io/" + ip.String()
	req, err := http.NewRequestWithContext(r.Context(), "GET", u, nil)
	if err != nil {
		jsonError(w, 500, "REQUEST_ERROR", err.Error())
		return
	}
	client := &http.Client{Timeout: 8 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		jsonError(w, 502, "SOURCE_UNAVAILABLE", err.Error())
		return
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		jsonError(w, 502, "SOURCE_UNAVAILABLE", fmt.Sprintf("InternetDB HTTP %d", res.StatusCode))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(200)
	_, _ = io.Copy(w, io.LimitReader(res.Body, 250000))
}
