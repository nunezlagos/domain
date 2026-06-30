package ratelimit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

func TestOrgRateLimiter_AllowsUpToLimit(t *testing.T) {
	rl := NewOrgRateLimiter(Config{
		DefaultRatePerMinute: 100,
		DefaultBurst:         100,
	})
	allowed := 0
	for i := 0; i < 100; i++ {
		ok, _, _ := rl.Allow("org-a")
		if ok {
			allowed++
		}
	}
	require.Equal(t, 100, allowed)
}

func TestOrgRateLimiter_DeniesOverLimit(t *testing.T) {
	rl := NewOrgRateLimiter(Config{
		DefaultRatePerMinute: 100,
		DefaultBurst:         100,
	})
	allowed := 0
	denied := 0
	for i := 0; i < 150; i++ {
		ok, _, _ := rl.Allow("org-a")
		if ok {
			allowed++
		} else {
			denied++
		}
	}
	require.Equal(t, 100, allowed)
	require.Equal(t, 50, denied)
}

func TestOrgRateLimiter_PerOrgIsolation(t *testing.T) {
	rl := NewOrgRateLimiter(Config{
		DefaultRatePerMinute: 100,
		DefaultBurst:         100,
	})

	for i := 0; i < 200; i++ {
		rl.Allow("org-a")
	}

	ok, _, _ := rl.Allow("org-b")
	require.True(t, ok, "org-b should not be affected by org-a's overuse")
}

func TestOrgRateLimiter_DefaultFallback(t *testing.T) {
	rl := NewOrgRateLimiter(Config{
		DefaultRatePerMinute: 50,
		DefaultBurst:         50,
	})
	allowed := 0
	for i := 0; i < 100; i++ {
		ok, _, _ := rl.Allow("unknown-org")
		if ok {
			allowed++
		}
	}
	require.Equal(t, 50, allowed, "should use default rate limit")
}

func TestOrgRateLimiter_RetryAfter(t *testing.T) {
	rl := NewOrgRateLimiter(Config{
		DefaultRatePerMinute: 60,  // 1/sec
		DefaultBurst:         60,
	})

	for i := 0; i < 100; i++ {
		rl.Allow("org-a")
	}
	_, retryAfter, _ := rl.Allow("org-a")
	require.Greater(t, retryAfter, time.Duration(0))
	require.LessOrEqual(t, retryAfter, 60*time.Second)
}

func TestOrgRateLimiter_Headers(t *testing.T) {
	rl := NewOrgRateLimiter(Config{
		DefaultRatePerMinute: 100,
		DefaultBurst:         100,
	})
	_, _, info := rl.Allow("org-a")
	require.Equal(t, 100, info.Limit)
	require.Equal(t, 99, info.Remaining)
	require.False(t, info.ResetAt.IsZero())
}

func TestOrgRateLimiter_LRUEviction(t *testing.T) {
	if testing.Short() {
		t.Skip("timing-dependent test")
	}

	rl := NewOrgRateLimiter(Config{
		DefaultRatePerMinute:   100,
		DefaultBurst:           100,
		IdleEvictionAfter:      time.Nanosecond, // evict immediately
		EvictionInterval:       5 * time.Millisecond,
	})
	defer rl.Stop()


	rl.mu.Lock()
	rl.buckets["old-org"] = &orgBucket{
		limiter:  rate.NewLimiter(100, 100),
		lastUsed: time.Now().Add(-1 * time.Hour),
	}
	rl.mu.Unlock()

	time.Sleep(15 * time.Millisecond) // wait for eviction

	require.Equal(t, 0, rl.Size(), "old bucket should be evicted")
}

func TestOrgRateLimiter_BurstExceedsRate(t *testing.T) {
	rl := NewOrgRateLimiter(Config{
		DefaultRatePerMinute: 60,
		DefaultBurst:         120,
	})

	allowed := 0
	for i := 0; i < 120; i++ {
		ok, _, _ := rl.Allow("org-a")
		if ok {
			allowed++
		}
	}
	require.Equal(t, 120, allowed)


	ok, _, _ := rl.Allow("org-a")
	require.False(t, ok)
}
