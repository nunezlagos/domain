package session

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestTempToken_RoundTrip(t *testing.T) {
	uid := uuid.New()
	now := time.Now()
	tok, err := generateTempToken(uid, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	got, err := parseTempToken(tok, now)
	if err != nil {
		t.Fatal(err)
	}
	if got != uid {
		t.Fatalf("expected %s, got %s", uid, got)
	}
}

func TestTempToken_Expired(t *testing.T) {
	uid := uuid.New()
	now := time.Now()
	tok, _ := generateTempToken(uid, now.Add(-time.Second))
	if _, err := parseTempToken(tok, now); err == nil {
		t.Fatal("expected expired token to fail")
	}
}

func TestTempToken_Tampered(t *testing.T) {
	uid := uuid.New()
	now := time.Now()
	tok, _ := generateTempToken(uid, now.Add(time.Minute))
	bad := tok[:len(tok)-2] + "AA"
	if _, err := parseTempToken(bad, now); err == nil {
		t.Fatal("expected tampered token to fail")
	}
}

func TestSessionToken_PrefixAndUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		tok, err := newSessionToken()
		if err != nil {
			t.Fatal(err)
		}
		if tok[:5] != TokenPrefix {
			t.Fatalf("unexpected prefix: %q", tok)
		}
		if seen[tok] {
			t.Fatal("collision in session token generator")
		}
		seen[tok] = true
	}
}

func TestHashToken_Deterministic(t *testing.T) {
	a := hashToken("foo")
	b := hashToken("foo")
	if !equalBytes(a, b) {
		t.Fatal("hashToken should be deterministic")
	}
	c := hashToken("foo2")
	if equalBytes(a, c) {
		t.Fatal("hashToken should differ for different inputs")
	}
}
