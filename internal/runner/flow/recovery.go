// HU-09.6 — durable execution recovery worker.
//
// Identifica flow_runs en status='running' cuyo last_heartbeat_at sea > N min
// (probable crash del worker original). Los marca como 'failed' con
// recovery_reason — política simple. Una versión más sofisticada podría
// reanudar desde cursor pero requiere step-level idempotency garantizado
// para todos los step types (no aplica para http_request, agent_run con
// side effects).

package flowrunner

import (
	"context"
	"log/slog"
	"time"
)

// RecoveryConfig parámetros del worker.
type RecoveryConfig struct {
	StaleAfter   time.Duration // si last_heartbeat_at + StaleAfter < NOW → stale
	PollInterval time.Duration // default 1min
}

// RunRecovery loop periódico que marca runs stale como failed.
// Pensado para correr en el pod leader (HU-26.2).
func (r *Runner) RunRecovery(ctx context.Context, cfg RecoveryConfig) {
	stale := cfg.StaleAfter
	if stale == 0 {
		stale = 5 * time.Minute
	}
	poll := cfg.PollInterval
	if poll == 0 {
		poll = 60 * time.Second
	}
	logger := slog.Default()
	logger.Info("flow recovery worker started",
		slog.Duration("stale_after", stale),
		slog.Duration("poll", poll))

	ticker := time.NewTicker(poll)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("flow recovery worker stopping")
			return
		case <-ticker.C:
			n, err := r.markStaleAsFailed(ctx, stale)
			if err != nil {
				logger.Error("recovery sweep failed", slog.Any("err", err))
				continue
			}
			if n > 0 {
				logger.Warn("recovered stale flow_runs", slog.Int64("count", n))
			}
		}
	}
}

func (r *Runner) markStaleAsFailed(ctx context.Context, stale time.Duration) (int64, error) {
	tag, err := r.Pool.Exec(ctx, `
UPDATE flow_runs
SET status = 'failed',
    error = COALESCE(error, '') || ' [recovery: worker stale > ' || $1::text || ']',
    finished_at = NOW(),
    recovery_count = recovery_count + 1
WHERE status = 'running'
  AND last_heartbeat_at IS NOT NULL
  AND last_heartbeat_at < NOW() - $2::interval
`, stale.String(), stale)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
