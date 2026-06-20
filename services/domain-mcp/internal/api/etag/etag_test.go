package etag

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCompute_StableForSameInputs(t *testing.T) {
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	a := Compute("abc", now)
	b := Compute("abc", now)
	if a != b {
		t.Fatalf("expected stable, got %s != %s", a, b)
	}
	if len(a) != 18 { // "16 hex chars"
		t.Fatalf("unexpected len: %d", len(a))
	}
}

func TestCompute_DiffersOnUpdate(t *testing.T) {
	t1 := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Nanosecond)
	if Compute("x", t1) == Compute("x", t2) {
		t.Fatal("ETag should change when updated_at changes")
	}
}

func TestIsNotModified_INM_Hit(t *testing.T) {
	now := time.Now().UTC()
	tag := Compute("id1", now)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("If-None-Match", tag)
	if !IsNotModified(r, tag, now) {
		t.Fatal("expected 304")
	}
}

func TestIsNotModified_INM_Miss(t *testing.T) {
	now := time.Now().UTC()
	tag := Compute("id1", now)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("If-None-Match", `"stale"`)
	if IsNotModified(r, tag, now) {
		t.Fatal("expected fresh body")
	}
}

func TestIsNotModified_IMS_Hit(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("If-Modified-Since", LastModified(now))
	if !IsNotModified(r, "", now) {
		t.Fatal("expected 304 on equal IMS")
	}
}

func TestMatchesIfMatch(t *testing.T) {
	tag := `"abc123"`
	r := httptest.NewRequest(http.MethodPatch, "/", nil)
	has, ok := MatchesIfMatch(r, tag)
	if has || ok {
		t.Fatal("no header → false/false")
	}
	r.Header.Set("If-Match", tag)
	has, ok = MatchesIfMatch(r, tag)
	if !has || !ok {
		t.Fatal("header matching → true/true")
	}
	r.Header.Set("If-Match", `"different"`)
	has, ok = MatchesIfMatch(r, tag)
	if !has || ok {
		t.Fatal("header mismatch → true/false")
	}
	r.Header.Set("If-Match", "*")
	if _, ok := MatchesIfMatch(r, tag); !ok {
		t.Fatal("wildcard matches")
	}
}
