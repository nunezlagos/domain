package apikey

import (
	"context"
	"sync"
	"time"
)

type cacheEntry struct {
	principal *Principal
	expiresAt time.Time
}

type CachedResolver struct {
	inner  Resolver
	mu     sync.RWMutex
	cache  map[string]*cacheEntry
	ttl    time.Duration
	clock  func() time.Time
}

func NewCachedResolver(inner Resolver, ttl time.Duration) *CachedResolver {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &CachedResolver{
		inner: inner,
		cache: make(map[string]*cacheEntry),
		ttl:   ttl,
		clock: time.Now,
	}
}

func (c *CachedResolver) Resolve(ctx context.Context, plaintext string) (*Principal, error) {
	c.mu.RLock()
	entry, ok := c.cache[plaintext]
	c.mu.RUnlock()
	if ok && c.clock().Before(entry.expiresAt) {
		if entry.principal == nil {
			return nil, ErrNotFound
		}
		return entry.principal, nil
	}
	p, err := c.inner.Resolve(ctx, plaintext)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.cache[plaintext] = &cacheEntry{
		principal: p,
		expiresAt: c.clock().Add(c.ttl),
	}
	if len(c.cache) > 10000 {
		go c.evict()
	}
	c.mu.Unlock()
	return p, nil
}

func (c *CachedResolver) evict() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.clock()
	for k, entry := range c.cache {
		if now.After(entry.expiresAt) {
			delete(c.cache, k)
		}
	}
}
