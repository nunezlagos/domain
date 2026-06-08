package versioning

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExtractVersionSlug(t *testing.T) {
	cases := map[string]string{
		"/api/v1/observations": "v1",
		"/api/v1/":             "v1",
		"/api/v2/x/y":          "v2",
		"/api/version":         "version",
		"/health":              "",
		"/api":                 "",
	}
	for in, want := range cases {
		if got := extractVersionSlug(in); got != want {
			t.Fatalf("%q: got %q, want %q", in, got, want)
		}
	}
}

func TestMiddleware_ActiveVersion_NoHeaders(t *testing.T) {
	cat := NewCatalog("v1", Version{Slug: "v1", State: StateActive})
	h := cat.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/x", nil)
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Deprecation") != "" {
		t.Fatal("active version must NOT include Deprecation header")
	}
}

func TestMiddleware_DeprecatedVersion_Headers(t *testing.T) {
	dep := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	sun := time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC)
	cat := NewCatalog("v2",
		Version{Slug: "v1", State: StateDeprecated,
			DeprecatedAt: dep, SunsetAt: sun,
			MigrationDocsURL: "https://docs.example.com/migrate-v2"},
		Version{Slug: "v2", State: StateActive},
	)
	h := cat.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/x", nil)
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Deprecation") == "" {
		t.Fatal("missing Deprecation header")
	}
	if rec.Header().Get("Sunset") == "" {
		t.Fatal("missing Sunset header")
	}
	if !contains(rec.Header().Get("Link"), "migrate-v2") {
		t.Fatalf("Link missing migration url: %s", rec.Header().Get("Link"))
	}
}

func TestMiddleware_SunsetVersion_410(t *testing.T) {
	past := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	cat := NewCatalog("v2",
		Version{Slug: "v1", State: StateDeprecated, SunsetAt: past,
			MigrationDocsURL: "https://docs.example.com/migrate-v2"},
		Version{Slug: "v2", State: StateActive},
	)
	called := false
	h := cat.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/x", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusGone {
		t.Fatalf("want 410, got %d", rec.Code)
	}
	if called {
		t.Fatal("next handler must NOT be called for sunset version")
	}
}

func TestVersionInfoHandler(t *testing.T) {
	cat := NewCatalog("v2",
		Version{Slug: "v1", State: StateDeprecated,
			SunsetAt: time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC)},
		Version{Slug: "v2", State: StateActive},
	)
	rec := httptest.NewRecorder()
	cat.VersionInfoHandler(rec, httptest.NewRequest(http.MethodGet, "/api/version", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	var out map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["current"] != "v2" {
		t.Fatalf("current: %v", out["current"])
	}
	if versions, ok := out["versions"].([]any); !ok || len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %v", out["versions"])
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || (len(sub) > 0 && hasSubstr(s, sub)))
}

func hasSubstr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
