package cache

import (
	"testing"
	"time"
)

func TestLRU_BasicGetSet(t *testing.T) {
	c := New(10)
	c.Set("k1", []byte("v1"), time.Second)
	v, ok := c.Get("k1")
	if !ok || string(v) != "v1" {
		t.Fatalf("expected v1, got %v ok=%v", v, ok)
	}
	if _, ok := c.Get("missing"); ok {
		t.Fatal("missing key should be miss")
	}
}

func TestLRU_TTLExpiry(t *testing.T) {
	c := New(10)
	fake := time.Now()
	c.now = func() time.Time { return fake }
	c.Set("k", []byte("v"), 5*time.Second)
	fake = fake.Add(6 * time.Second)
	if _, ok := c.Get("k"); ok {
		t.Fatal("entry should have expired")
	}
	if c.Stats().Expirations != 1 {
		t.Fatalf("expected 1 expiration, got %d", c.Stats().Expirations)
	}
}

func TestLRU_Eviction(t *testing.T) {
	c := New(2)
	c.Set("a", []byte("1"), time.Hour)
	c.Set("b", []byte("2"), time.Hour)
	// a is now LRU; promote it
	c.Get("a")
	c.Set("c", []byte("3"), time.Hour) // should evict b
	if _, ok := c.Get("b"); ok {
		t.Fatal("b should have been evicted (LRU)")
	}
	if _, ok := c.Get("a"); !ok {
		t.Fatal("a should still be present (was recently accessed)")
	}
}

func TestLRU_FlushPrefix(t *testing.T) {
	c := New(10)
	c.Set("org1:tool:x", []byte("1"), time.Hour)
	c.Set("org1:tool:y", []byte("2"), time.Hour)
	c.Set("org2:tool:x", []byte("3"), time.Hour)
	n := c.FlushPrefix("org1:")
	if n != 2 {
		t.Fatalf("expected 2 deletions, got %d", n)
	}
	if _, ok := c.Get("org2:tool:x"); !ok {
		t.Fatal("org2 entry should survive")
	}
	if _, ok := c.Get("org1:tool:x"); ok {
		t.Fatal("org1 entry should be gone")
	}
}

func TestLRU_ReturnsCopy(t *testing.T) {
	c := New(10)
	c.Set("k", []byte("original"), time.Hour)
	v, _ := c.Get("k")
	v[0] = 'X'
	v2, _ := c.Get("k")
	if string(v2) != "original" {
		t.Fatalf("cache should return a copy; got %s", v2)
	}
}
