package metrics

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// reportPoolStats exporta stats de un pool a los gauges con label "name".
func reportPoolStats(pool *pgxpool.Pool, reg *Registry, name string) {
	st := pool.Stat()
	reg.DBPoolInUse.WithLabelValues(name).Set(float64(st.AcquiredConns()))
	reg.DBPoolIdle.WithLabelValues(name).Set(float64(st.IdleConns()))
	reg.DBPoolTotal.WithLabelValues(name).Set(float64(st.MaxConns()))
}

// RunPoolStatsReporter corre cada 15s exportando stats de todos los pools.
func RunPoolStatsReporter(ctx context.Context, reg *Registry, app, auth *pgxpool.Pool, ro *pgxpool.Pool, logger *slog.Logger) {
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		report := func() {
			reportPoolStats(app, reg, "app")
			reportPoolStats(auth, reg, "auth")
			if ro != nil {
				reportPoolStats(ro, reg, "readonly")
			}
		}
		report()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				report()
				logger.Debug("pool stats reported")
			}
		}
	}()
}
