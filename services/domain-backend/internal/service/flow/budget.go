package flow

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BudgetCache struct {
	mu       sync.RWMutex
	entries  map[uuid.UUID]durationEntry
	pool     *pgxpool.Pool
	ttl      time.Duration
}

type durationEntry struct {
	duration  time.Duration
	expiresAt time.Time
}

func (c *BudgetCache) defaultDuration() time.Duration {
	return 5 * time.Minute
}

func NewBudgetCache(pool *pgxpool.Pool) *BudgetCache {
	return &BudgetCache{
		entries: make(map[uuid.UUID]durationEntry),
		pool:    pool,
		ttl:     30 * time.Second,
	}
}

func (c *BudgetCache) GetMaxDuration(ctx context.Context, orgID uuid.UUID) (time.Duration, error) {
	c.mu.RLock()
	entry, ok := c.entries[orgID]
	c.mu.RUnlock()
	if ok && time.Now().Before(entry.expiresAt) {
		return entry.duration, nil
	}

	dur, err := c.fetchFromDB(ctx, orgID)
	if err != nil {
		return 0, err
	}

	c.mu.Lock()
	c.entries[orgID] = durationEntry{
		duration:  dur,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()
	return dur, nil
}

// fetchFromDB lee la config de flow global (single-org, sin organization_id).
// El param orgID se conserva por compat de la cache key.
func (c *BudgetCache) fetchFromDB(ctx context.Context, orgID uuid.UUID) (time.Duration, error) {
	_ = orgID
	var seconds int
	err := c.pool.QueryRow(ctx,
		`SELECT max_flow_duration_seconds FROM org_flow_config LIMIT 1`,
	).Scan(&seconds)
	if err != nil {
		return 5 * time.Minute, nil
	}
	return time.Duration(seconds) * time.Second, nil
}

func (c *BudgetCache) Invalidate(orgID uuid.UUID) {
	c.mu.Lock()
	delete(c.entries, orgID)
	c.mu.Unlock()
}
