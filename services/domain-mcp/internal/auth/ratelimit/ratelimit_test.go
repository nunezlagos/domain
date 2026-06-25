

package ratelimit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// clockMock controlled time para tests.
type clockMock struct{ t time.Time }

func (c *clockMock) now() time.Time { return c.t }
func (c *clockMock) advance(d time.Duration) { c.t = c.t.Add(d) }

func TestLimiter_AllowsBurstUpToCapacity(t *testing.T) {
	l := New(5, 1.0)
	for i := 0; i < 5; i++ {
		require.True(t, l.Allow("k"), "should allow burst %d", i+1)
	}
	require.False(t, l.Allow("k"), "6th should be denied")
}

func TestLimiter_RefillsOverTime(t *testing.T) {
	c := &clockMock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	l := New(5, 1.0) // 1 token/sec
	l.SetClock(c.now)


	for i := 0; i < 5; i++ {
		require.True(t, l.Allow("k"))
	}
	require.False(t, l.Allow("k"))


	c.advance(3 * time.Second)
	for i := 0; i < 3; i++ {
		require.True(t, l.Allow("k"), "should allow after refill %d", i+1)
	}
	require.False(t, l.Allow("k"), "4th should fail (only 3 refilled)")
}

func TestLimiter_PerKey_Isolated(t *testing.T) {
	l := New(2, 1.0)
	require.True(t, l.Allow("alice"))
	require.True(t, l.Allow("alice"))
	require.False(t, l.Allow("alice"))

	require.True(t, l.Allow("bob"))
	require.True(t, l.Allow("bob"))
	require.False(t, l.Allow("bob"))
}

func TestLimiter_AllowN(t *testing.T) {
	l := New(10, 1.0)
	require.True(t, l.AllowN("k", 5))
	require.True(t, l.AllowN("k", 5))
	require.False(t, l.AllowN("k", 1))
}

func TestLimiter_DoesNotExceedCapacity(t *testing.T) {
	c := &clockMock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	l := New(5, 1.0)
	l.SetClock(c.now)

	require.True(t, l.Allow("k"))

	c.advance(100 * time.Second)
	require.Equal(t, float64(5), l.Tokens("k"))
}

func TestLimiter_Tokens_ReportsCurrent(t *testing.T) {
	c := &clockMock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	l := New(10, 1.0)
	l.SetClock(c.now)
	require.Equal(t, float64(10), l.Tokens("new"))

	l.Allow("k")
	require.Equal(t, float64(9), l.Tokens("k"))

	c.advance(2 * time.Second)
	tokens := l.Tokens("k")
	require.InDelta(t, 10.0, tokens, 0.01, "max 10 after 2s refill")
}

func TestLimiter_RetryAfter(t *testing.T) {
	c := &clockMock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	l := New(5, 1.0) // 1 token/sec
	l.SetClock(c.now)

	for i := 0; i < 5; i++ {
		l.Allow("k")
	}

	d := l.RetryAfter("k", 1)
	require.InDelta(t, time.Second.Seconds(), d.Seconds(), 0.01)

	d = l.RetryAfter("k", 3)
	require.InDelta(t, (3 * time.Second).Seconds(), d.Seconds(), 0.01)
}

func TestLimiter_RetryAfter_ZeroWhenAvailable(t *testing.T) {
	l := New(5, 1.0)
	require.Equal(t, time.Duration(0), l.RetryAfter("fresh", 1))
}

func TestLimiter_Cleanup_RemovesStale(t *testing.T) {
	c := &clockMock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	l := New(5, 1.0)
	l.SetClock(c.now)

	l.Allow("k1")
	l.Allow("k2")
	require.Equal(t, 2, l.Size())

	c.advance(2 * time.Hour)
	l.Allow("k3") // touch k3 lastRefill = ahora

	removed := l.Cleanup(1 * time.Hour)
	require.Equal(t, 2, removed)
	require.Equal(t, 1, l.Size())
}

// Sabotaje: 1000 allows secuenciales con refill 0.1/sec — solo capacity inicial passes.
func TestSabotage_NoFreeTokens(t *testing.T) {
	c := &clockMock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	l := New(3, 0.1)
	l.SetClock(c.now)

	allowed := 0
	for i := 0; i < 1000; i++ {
		if l.Allow("k") {
			allowed++
		}
	}
	require.Equal(t, 3, allowed, "exactly capacity allowed without time passing")
}

// Sabotaje: thread safety bajo concurrency.
func TestSabotage_ConcurrencySafety(t *testing.T) {
	l := New(100, 0)
	done := make(chan bool, 50)
	for i := 0; i < 50; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				l.Allow("shared")
			}
			done <- true
		}()
	}
	for i := 0; i < 50; i++ {
		<-done
	}
	require.LessOrEqual(t, l.Tokens("shared"), float64(100))
}
