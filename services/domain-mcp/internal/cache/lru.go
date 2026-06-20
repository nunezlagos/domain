// Package cache: LRU + TTL in-memory thread-safe.
//
// REQ-67. Diseñado para envolver tool handlers MCP READ-only y devolver
// respuestas cacheadas durante un TTL corto (5-30s típicamente).
//
// El cache es RLS-aware: la key incluye org_id, así dos orgs nunca ven
// el resultado de la otra. Las mutaciones invalidan via Flush(orgID).
//
// No usa goroutine de cleanup periódico — los expires se descartan
// lazy en Get(). Cuando se llena se evicta el LRU.
package cache

import (
	"container/list"
	"sync"
	"time"
)

type entry struct {
	key       string
	value     []byte
	expiresAt time.Time
}

// LRU es un cache thread-safe con tamaño máximo y TTL por entry.
type LRU struct {
	mu      sync.Mutex
	maxSize int
	ll      *list.List
	items   map[string]*list.Element
	now     func() time.Time

	// stats
	hits, misses, evictions, expirations uint64
}

func New(maxSize int) *LRU {
	if maxSize <= 0 {
		maxSize = 1024
	}
	return &LRU{
		maxSize: maxSize,
		ll:      list.New(),
		items:   map[string]*list.Element{},
		now:     time.Now,
	}
}

// Get devuelve (value, true) si hay entry vigente, sino (nil, false).
// Refresca el orden LRU al hit.
func (c *LRU) Get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		c.misses++
		return nil, false
	}
	e := el.Value.(*entry)
	if c.now().After(e.expiresAt) {
		c.ll.Remove(el)
		delete(c.items, key)
		c.expirations++
		c.misses++
		return nil, false
	}
	c.ll.MoveToFront(el)
	c.hits++
	// Devolvemos copia para que el caller no pueda mutar el slice
	// interno (handlers escriben sobre buffers reciclados a veces).
	cp := make([]byte, len(e.value))
	copy(cp, e.value)
	return cp, true
}

// Set guarda value bajo key con TTL. Evicta el menos usado si está lleno.
func (c *LRU) Set(key string, value []byte, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		e := el.Value.(*entry)
		e.value = append(e.value[:0], value...)
		e.expiresAt = c.now().Add(ttl)
		c.ll.MoveToFront(el)
		return
	}
	if c.ll.Len() >= c.maxSize {
		oldest := c.ll.Back()
		if oldest != nil {
			c.ll.Remove(oldest)
			delete(c.items, oldest.Value.(*entry).key)
			c.evictions++
		}
	}
	cp := make([]byte, len(value))
	copy(cp, value)
	el := c.ll.PushFront(&entry{
		key:       key,
		value:     cp,
		expiresAt: c.now().Add(ttl),
	})
	c.items[key] = el
}

// FlushPrefix borra todas las entries cuya key empiece con prefix.
// Pensado para "invalida todo del org X" pasando prefix=orgID+":".
func (c *LRU) FlushPrefix(prefix string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for key, el := range c.items {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			c.ll.Remove(el)
			delete(c.items, key)
			n++
		}
	}
	return n
}

// Stats snapshot para exponer en /metrics.
type Stats struct {
	Size        int
	MaxSize     int
	Hits        uint64
	Misses      uint64
	Evictions   uint64
	Expirations uint64
}

func (c *LRU) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return Stats{
		Size:        c.ll.Len(),
		MaxSize:     c.maxSize,
		Hits:        c.hits,
		Misses:      c.misses,
		Evictions:   c.evictions,
		Expirations: c.expirations,
	}
}
