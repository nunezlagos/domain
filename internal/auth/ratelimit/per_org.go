package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type LimitInfo struct {
	Limit     int
	Remaining int
	ResetAt   time.Time
}

type Config struct {
	DefaultRatePerMinute int
	DefaultBurst         int
	EvictionInterval     time.Duration
	IdleEvictionAfter    time.Duration
}

type orgBucket struct {
	limiter  *rate.Limiter
	lastUsed time.Time
	rate     rate.Limit
	burst    int
}

type OrgRateLimiter struct {
	mu         sync.Mutex
	buckets    map[string]*orgBucket
	config     Config
	clock      func() time.Time
	stopCh     chan struct{}
	evictionWg sync.WaitGroup
}

func NewOrgRateLimiter(cfg Config) *OrgRateLimiter {
	if cfg.DefaultRatePerMinute <= 0 {
		cfg.DefaultRatePerMinute = 1000
	}
	if cfg.DefaultBurst <= 0 {
		cfg.DefaultBurst = cfg.DefaultRatePerMinute * 2
	}
	if cfg.EvictionInterval <= 0 {
		cfg.EvictionInterval = 10 * time.Minute
	}
	if cfg.IdleEvictionAfter <= 0 {
		cfg.IdleEvictionAfter = 1 * time.Hour
	}

	rl := &OrgRateLimiter{
		buckets: make(map[string]*orgBucket),
		config:  cfg,
		clock:   time.Now,
		stopCh:  make(chan struct{}),
	}

	rl.evictionWg.Add(1)
	go rl.evictionLoop()
	return rl
}

func (rl *OrgRateLimiter) Allow(orgID string) (bool, time.Duration, LimitInfo) {
	rl.mu.Lock()
	b := rl.getOrCreate(orgID)
	now := rl.clock()
	rl.mu.Unlock()

	b.lastUsed = now
	ok := b.limiter.Allow()

	tokens := b.limiter.Tokens()
	limit := b.burst
	remaining := int(tokens)
	resetAt := now.Add(
		time.Duration(float64(time.Second) * float64(limit-int(tokens)) / float64(b.rate)))

	info := LimitInfo{Limit: limit, Remaining: remaining, ResetAt: resetAt}
	if ok {
		return true, 0, info
	}

	retryAfter := time.Duration(float64(time.Second) / float64(b.rate))
	return false, retryAfter, info
}

func (rl *OrgRateLimiter) getOrCreate(orgID string) *orgBucket {
	b, exists := rl.buckets[orgID]
	if !exists {
		r := rate.Every(time.Minute / time.Duration(rl.config.DefaultRatePerMinute))
		b = &orgBucket{
			limiter:  rate.NewLimiter(r, rl.config.DefaultBurst),
			lastUsed: rl.clock(),
			rate:     r,
			burst:    rl.config.DefaultBurst,
		}
		rl.buckets[orgID] = b
	}
	return b
}

func (rl *OrgRateLimiter) Size() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return len(rl.buckets)
}

func (rl *OrgRateLimiter) SetClock(fn func() time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.clock = fn
}

func (rl *OrgRateLimiter) evictionLoop() {
	defer rl.evictionWg.Done()
	ticker := time.NewTicker(rl.config.EvictionInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.evictIdle()
		case <-rl.stopCh:
			return
		}
	}
}

func (rl *OrgRateLimiter) evictIdle() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := rl.clock()
	for k, b := range rl.buckets {
		if now.Sub(b.lastUsed) > rl.config.IdleEvictionAfter {
			delete(rl.buckets, k)
		}
	}
}

func (rl *OrgRateLimiter) Stop() {
	close(rl.stopCh)
	rl.evictionWg.Wait()
}
