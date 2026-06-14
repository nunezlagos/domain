package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

func newRequest(t *testing.T, method, path, origin string, preflightMethod string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if preflightMethod != "" {
		req.Header.Set("Access-Control-Request-Method", preflightMethod)
		req.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type")
	}
	return req
}

// Escenario 4: sin env var → default deny. No headers CORS.
func TestCORS_DefaultDeny_NoOriginsConfigured(t *testing.T) {
	c := NewCORS(nil, nil)
	if c.Enabled() {
		t.Fatalf("Enabled() = true, want false (no origins)")
	}
	rr := httptest.NewRecorder()
	c.Wrap(newHandler()).ServeHTTP(rr, newRequest(t, "GET", "/api/v1/x", "https://app.example.com", ""))
	if h := rr.Header().Get("Access-Control-Allow-Origin"); h != "" {
		t.Fatalf("Allow-Origin = %q, want empty (default deny)", h)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (pasa transparente)", rr.Code)
	}
}

// Escenario 1: origin en allowlist → headers correctos.
func TestCORS_OriginInAllowlist_AddsHeaders(t *testing.T) {
	c := NewCORS([]string{"https://app.example.com", "https://staging.example.com"}, nil)
	rr := httptest.NewRecorder()
	c.Wrap(newHandler()).ServeHTTP(rr, newRequest(t, "GET", "/api/v1/x", "https://app.example.com", ""))
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("Allow-Origin = %q, want app", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("Allow-Credentials = %q, want true", got)
	}
	if got := rr.Header().Get("Vary"); !strings.Contains(got, "Origin") {
		t.Fatalf("Vary = %q, want contains Origin", got)
	}
}

// Escenario 2: origin fuera de allowlist → sin headers + log warn.
func TestCORS_OriginNotInAllowlist_NoHeaders(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	c := NewCORS([]string{"https://app.example.com"}, logger)
	rr := httptest.NewRecorder()
	c.Wrap(newHandler()).ServeHTTP(rr, newRequest(t, "GET", "/api/v1/x", "https://evil.com", ""))
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Allow-Origin = %q, want empty (denied)", got)
	}
	if !strings.Contains(buf.String(), "CORS denied origin") {
		t.Fatalf("log no contiene 'CORS denied origin': %s", buf.String())
	}
	if !strings.Contains(buf.String(), "evil.com") {
		t.Fatalf("log no incluye el origin: %s", buf.String())
	}
}

// Escenario 3: preflight OPTIONS válido devuelve 204 con todos los headers.
func TestCORS_PreflightAllowed_204WithHeaders(t *testing.T) {
	c := NewCORS([]string{"https://app.example.com"}, nil)
	rr := httptest.NewRecorder()
	c.Wrap(newHandler()).ServeHTTP(rr, newRequest(t, "OPTIONS", "/api/v1/x", "https://app.example.com", "POST"))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", rr.Code)
	}
	if got := rr.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, "POST") {
		t.Fatalf("Allow-Methods = %q, want contiene POST", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "Authorization") {
		t.Fatalf("Allow-Headers = %q, want contiene Authorization", got)
	}
	if got := rr.Header().Get("Access-Control-Max-Age"); got == "" {
		t.Fatalf("Max-Age = empty, want set")
	}
}

// Preflight denegado → 403, sin headers.
func TestCORS_PreflightDenied_403(t *testing.T) {
	c := NewCORS([]string{"https://app.example.com"}, nil)
	rr := httptest.NewRecorder()
	c.Wrap(newHandler()).ServeHTTP(rr, newRequest(t, "OPTIONS", "/api/v1/x", "https://evil.com", "POST"))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("preflight denied status = %d, want 403", rr.Code)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Allow-Origin = %q, want empty", got)
	}
}

// Escenario 5: wildcard sin credentials + warn.
func TestCORS_Wildcard_NoCredentials_LogsWarn(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	c := NewCORS([]string{"*"}, logger)
	if !strings.Contains(buf.String(), "CORS wildcard enabled") {
		t.Fatalf("log no incluye warn wildcard: %s", buf.String())
	}
	rr := httptest.NewRecorder()
	c.Wrap(newHandler()).ServeHTTP(rr, newRequest(t, "GET", "/api/v1/x", "https://anywhere.example", ""))
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Allow-Origin = %q, want *", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("Allow-Credentials = %q, want empty (wildcard + creds is invalid)", got)
	}
}

// Escenario 6: múltiples origins → Vary: Origin presente para evitar cache poisoning.
func TestCORS_MultipleOrigins_AddsVaryOrigin(t *testing.T) {
	c := NewCORS([]string{"https://a.example.com", "https://b.example.com"}, nil)
	rr := httptest.NewRecorder()
	c.Wrap(newHandler()).ServeHTTP(rr, newRequest(t, "GET", "/api/v1/x", "https://a.example.com", ""))
	if got := rr.Header().Get("Vary"); !strings.Contains(got, "Origin") {
		t.Fatalf("Vary = %q, want contains Origin", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://a.example.com" {
		t.Fatalf("Allow-Origin = %q, want a", got)
	}
}

// Escenario 8: origin con puerto distinto NO matchea host bare.
func TestCORS_OriginWithPort_StrictMatch(t *testing.T) {
	c := NewCORS([]string{"https://app.example.com"}, nil)
	rr := httptest.NewRecorder()
	c.Wrap(newHandler()).ServeHTTP(rr, newRequest(t, "GET", "/api/v1/x", "https://app.example.com:3000", ""))
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Allow-Origin = %q, want empty (port mismatch)", got)
	}
}

// Request sin Origin (server-to-server, curl) → pasa transparente.
func TestCORS_NoOriginHeader_PassesThrough(t *testing.T) {
	c := NewCORS([]string{"https://app.example.com"}, nil)
	rr := httptest.NewRecorder()
	c.Wrap(newHandler()).ServeHTTP(rr, newRequest(t, "GET", "/api/v1/x", "", ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Allow-Origin = %q, want empty (no Origin header)", got)
	}
}

// NewCORS trimea espacios y descarta vacíos en la lista.
func TestCORS_NewCORS_TrimsAndDiscardsEmpty(t *testing.T) {
	c := NewCORS([]string{"  https://a.example  ", "", "  ", "https://b.example"}, nil)
	if got, want := len(c.AllowedOrigins), 2; got != want {
		t.Fatalf("len(AllowedOrigins) = %d, want %d", got, want)
	}
	if c.AllowedOrigins[0] != "https://a.example" {
		t.Fatalf("first = %q, want trimmed", c.AllowedOrigins[0])
	}
}
