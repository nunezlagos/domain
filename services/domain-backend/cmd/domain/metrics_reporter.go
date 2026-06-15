package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/cache"
	"nunezlagos/domain/internal/events"
	"nunezlagos/domain/internal/metrics"
)

// runReq70Reporter (REQ-70) actualiza gauges no-evento-driven:
//   - domain_mcp_cache_size (LRU.Stats().Size)
//   - domain_sse_subscribers (bus.Stats().total)
//   - domain_tickets_locked_active (SELECT COUNT)
//
// Frecuencia: cada 10s. Cancelable via ctx.
func runReq70Reporter(
	ctx context.Context,
	reg *metrics.Registry,
	lru *cache.LRU,
	bus *events.Bus,
	pool *pgxpool.Pool,
	logger *slog.Logger,
) {
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if lru != nil {
				reg.MCPCacheSize.Set(float64(lru.Stats().Size))
			}
			if bus != nil {
				total, _ := bus.Stats()
				reg.SSESubscribers.Set(float64(total))
			}
			if pool != nil {
				var n int
				err := pool.QueryRow(ctx, `
					SELECT COUNT(*) FROM project_tickets
					WHERE locked_by IS NOT NULL
					  AND locked_until > NOW()
					  AND deleted_at IS NULL`).Scan(&n)
				if err != nil {
					logger.Debug("locked_active query failed", slog.String("err", err.Error()))
					continue
				}
				reg.TicketsLockedActive.Set(float64(n))
			}
		}
	}
}
