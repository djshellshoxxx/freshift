package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/djshellshoxxx/freshift/internal/freshsift"
)

func TestIndexAndAPISecurityHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()
	newMux(freshsift.NewEngine()).ServeHTTP(res, req)
	if res.Code != 200 || !strings.Contains(res.Body.String(), "FreshSift") {
		t.Fatalf("index failed: %d", res.Code)
	}
	if res.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("CSP missing")
	}
}
func TestStartRejectsBadRegex(t *testing.T) {
	body := []byte(`{"seeds":"example.org","patterns":"[","regex":true}`)
	req := httptest.NewRequest("POST", "/api/start", bytes.NewReader(body))
	res := httptest.NewRecorder()
	newMux(freshsift.NewEngine()).ServeHTTP(res, req)
	if res.Code != 400 || !strings.Contains(res.Body.String(), "invalid regular expression") {
		t.Fatalf("expected bad regex response: %d %s", res.Code, res.Body.String())
	}
}
func TestStatusAndExport(t *testing.T) {
	h := newMux(freshsift.NewEngine())
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/status", nil))
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["stats"]; !ok {
		t.Fatal("missing stats")
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/export?format=csv", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Matched String") {
		t.Fatal("CSV export failed")
	}
}
func TestRejectCrossOriginStart(t *testing.T) {
	req := httptest.NewRequest("POST", "http://127.0.0.1:8787/api/start", strings.NewReader(`{}`))
	req.Header.Set("Origin", "http://evil.test")
	w := httptest.NewRecorder()
	newMux(freshsift.NewEngine()).ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}
func TestCSVFormulaEscaping(t *testing.T) {
	if csvSafe("=1+2") != "'=1+2" {
		t.Fatal("CSV formula not escaped")
	}
}

func TestRemoteAPITokenAndPagesCORS(t *testing.T) {
	server := newMuxWithSettings(freshsift.NewEngine(), remoteSettings{AllowedOrigin: "https://djshellshoxxx.github.io", APIToken: "a-secret-test-token"})
	tests := []struct {
		method, path, origin, token string
		want                        int
	}{
		{"OPTIONS", "/api/start", "https://djshellshoxxx.github.io", "", 204},
		{"GET", "/api/status", "https://djshellshoxxx.github.io", "", 401},
		{"GET", "/api/status", "https://djshellshoxxx.github.io", "Bearer wrong", 401},
		{"GET", "/api/status", "https://djshellshoxxx.github.io", "Bearer a-secret-test-token", 200},
		{"GET", "/api/status", "https://evil.test", "Bearer a-secret-test-token", 403},
		{"POST", "/api/stop", "https://evil.test", "Bearer a-secret-test-token", 403},
		{"OPTIONS", "/api/start", "https://evil.test", "", 403},
	}
	for _, tc := range tests {
		req := httptest.NewRequest(tc.method, "http://127.0.0.1:8787"+tc.path, nil)
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		if tc.token != "" {
			req.Header.Set("Authorization", tc.token)
		}
		if tc.method == "OPTIONS" {
			req.Header.Set("Access-Control-Request-Method", "POST")
		}
		r := httptest.NewRecorder()
		server.ServeHTTP(r, req)
		if r.Code != tc.want {
			t.Errorf("%s %s origin=%s expected %d got %d: %s", tc.method, tc.path, tc.origin, tc.want, r.Code, r.Body.String())
		}
		if tc.origin == "https://djshellshoxxx.github.io" && r.Header().Get("Access-Control-Allow-Origin") != tc.origin {
			t.Error("missing Pages CORS header")
		}
	}
}

func TestRemoteRejectsURLWithInvalidAllowedOrigin(t *testing.T) {
	if err := validateAllowedOrigin("http://evil.example/a"); err == nil {
		t.Fatal("path in allowed origin accepted")
	}
	if err := validateAllowedOrigin("https://djshellshoxxx.github.io"); err != nil {
		t.Fatal(err)
	}
}
