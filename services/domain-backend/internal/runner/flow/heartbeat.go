// issue-09.6 — heartbeat goroutine 30s (de-004) + métrica (de-010).
package flowrunner

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/metrics"
)

// HeartbeatConfig configura el loop de heartbeat.
type HeartbeatConfig struct {
	Interval time.Duration // default 30s
	Pool     *pgxpool.Pool
	RunID    uuid.UUID
	Metrics  *metrics.Registry // nil = no metrics
}

// StartHeartbeat lanza una goroutine que actualiza last_heartbeat_at cada
// Interval. Retorna una cancel func para detenerla.
func StartHeartbeat(ctx context.Context, cfg HeartbeatConfig) context.CancelFunc {
	interval := cfg.Interval
	if interval <= 0 {
		interval = 30 * time.Second
	}

	ctx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		logger := slog.Default()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := beat(ctx, cfg.Pool, cfg.RunID); err != nil {
					logger.Error("flow heartbeat failed",
						slog.String("run_id", cfg.RunID.String()),
						slog.Any("err", err))
				}
				if cfg.Metrics != nil {
					emitHeartbeatAgeMetric(ctx, cfg.Pool, cfg.Metrics)
				}
			}
		}
	}()
	return cancel
}

// beat actualiza last_heartbeat_at para un flow_run.
func beat(ctx context.Context, pool *pgxpool.Pool, runID uuid.UUID) error {
	_, err := pool.Exec(ctx,
		`UPDATE flow_runs SET last_heartbeat_at = NOW() WHERE id = $1`, runID)
	return err
}

// emitHeartbeatAgeMetric publica domain_flow_heartbeat_age_seconds como el
// mínimo last_heartbeat_at entre los runs running (el más antiguo = más riesgo).
func emitHeartbeatAgeMetric(ctx context.Context, pool *pgxpool.Pool, m *metrics.Registry) {
	var ageSeconds float64
	err := pool.QueryRow(ctx, `
		SELECT COALESCE(
			EXTRACT(EPOCH FROM (NOW() - MIN(last_heartbeat_at))),
			0
		) FROM flow_runs
		WHERE status = 'running'
		  AND last_heartbeat_at IS NOT NULL
	`).Scan(&ageSeconds)
	if err != nil {
		return // swallow en métrica
	}
	m.FlowHeartbeatAgeSeconds.Set(ageSeconds)
}
