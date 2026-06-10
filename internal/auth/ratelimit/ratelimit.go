// Package ratelimit — issue-02.5 rate-limit + issue-26.6 backpressure compat.
//
// Token bucket por key (string), state en memoria local con TTL cleanup.
// Para cross-pod (issue-26 horizontal-scalability), usar Postgres-backed (futuro:
// tabla auth_rate_limits particionada por hora).
package ratelimit

import (
	"sync"
	"time"
)

// Bucket estado de un token bucket individual.
type Bucket struct {
	tokens     float64
	lastRefill time.Time
}

// Limiter token bucket per key.
type Limiter struct {
	mu       sync.Mutex
	buckets  map[string]*Bucket
	capacity float64       // tokens max
	refillRate float64     // tokens/sec
	clock    func() time.Time
}

// New crea Limiter con N tokens iniciales y refillRate tokens/sec.
// Ejemplo: NewLimiter(5, 1/60.0) = 5 burst + 1 token cada 60s = max 5/min.
func New(capacity int, refillRate float64) *Limiter {
	return &Limiter{
		buckets:    make(map[string]*Bucket),
		capacity:   float64(capacity),
		refillRate: refillRate,
		clock:      time.Now,
	}
}

// Allow consume 1 token. Retorna true si hubo, false si rate limited.
func (l *Limiter) Allow(key string) bool {
	return l.AllowN(key, 1)
}

// AllowN consume N tokens. Retorna true si había suficientes.
func (l *Limiter) AllowN(key string, n int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.clock()
	b, exists := l.buckets[key]
	if !exists {
		b = &Bucket{tokens: l.capacity, lastRefill: now}
		l.buckets[key] = b
	}

	// Refill: tokens += elapsed * refillRate, cap a capacity
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * l.refillRate
	if b.tokens > l.capacity {
		b.tokens = l.capacity
	}
	b.lastRefill = now

	if b.tokens >= float64(n) {
		b.tokens -= float64(n)
		return true
	}
	return false
}

// Tokens retorna tokens disponibles actuales (post-refill).
func (l *Limiter) Tokens(key string) float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.clock()
	b, exists := l.buckets[key]
	if !exists {
		return l.capacity
	}
	elapsed := now.Sub(b.lastRefill).Seconds()
	cur := b.tokens + elapsed*l.refillRate
	if cur > l.capacity {
		cur = l.capacity
	}
	return cur
}

// RetryAfter retorna tiempo hasta que haya N tokens, o 0 si ya hay.
func (l *Limiter) RetryAfter(key string, n int) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.clock()
	b, exists := l.buckets[key]
	if !exists {
		return 0
	}
	elapsed := now.Sub(b.lastRefill).Seconds()
	cur := b.tokens + elapsed*l.refillRate
	if cur >= float64(n) {
		return 0
	}
	needed := float64(n) - cur
	secs := needed / l.refillRate
	return time.Duration(secs * float64(time.Second))
}

// SetClock para tests deterministas.
func (l *Limiter) SetClock(fn func() time.Time) { l.clock = fn }

// Cleanup remueve buckets viejos (lastRefill < now - maxAge).
// Llamar periódicamente con goroutine cron.
func (l *Limiter) Cleanup(maxAge time.Duration) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.clock()
	removed := 0
	for k, b := range l.buckets {
		if now.Sub(b.lastRefill) > maxAge {
			delete(l.buckets, k)
			removed++
		}
	}
	return removed
}

// Size cantidad de buckets activos (para métricas).
func (l *Limiter) Size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}
