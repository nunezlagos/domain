package dlock

import (
	"testing"
)

func TestHashKey_Stable(t *testing.T) {
	a := HashKey("cron_scheduler")
	b := HashKey("cron_scheduler")
	if a != b {
		t.Fatalf("expected stable hash, got %d != %d", a, b)
	}
}

func TestHashKey_DistinctNames(t *testing.T) {
	a := HashKey("foo")
	b := HashKey("bar")
	if a == b {
		t.Fatalf("collision between distinct names: %d", a)
	}
}

func TestHashKey_EmptyVsNonEmpty(t *testing.T) {
	a := HashKey("")
	b := HashKey("x")
	if a == b {
		t.Fatal("empty and non-empty must hash distinct")
	}
}
