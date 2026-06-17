// issue-09.6 — durable execution recovery scanner (de-005).
//
// Libera flow_runs en status='running' cuyo last_heartbeat_at sea > N min
// (probable crash del worker original). El próximo worker que claimée el
// run lo reanudará desde su cursor con ResumeRun.
//
// recovery_count se incrementa para tracking. Si recovery_count supera un
// umbral, el run se marca como failed (crash loop detection).
package flowrunner

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// RecoveryConfig parámetros del scanner.
type RecoveryConfig struct {
	StaleAfter    time.Duration // si last_heartbeat_at + StaleAfter < NOW → stale
	PollInterval  time.Duration // default 1min
	MaxRecoveries int           // 0 = unlimited, >0 = crash-loop threshold
}

// RunRecovery loop periódico que libera runs stale para que otro worker
// pueda reclamarlos y reanudarlos.
// Pensado para correr en el pod leader (issue-26.2).
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
	logger.Info("flow recovery scanner started",
		slog.Duration("stale_after", stale),
		slog.Duration("poll", poll))

	ticker := time.NewTicker(poll)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("flow recovery scanner stopping")
			return
		case <-ticker.C:
			released, failed, err := r.ReleaseStaleRuns(ctx, stale, cfg.MaxRecoveries)
			if err != nil {
				logger.Error("recovery sweep failed", slog.Any("err", err))
				continue
			}
			if released > 0 {
				logger.Warn("released stale flow_runs for recovery",
					slog.Int64("released", released),
					slog.Int64("crash_loop_failed", failed))
			}

			cancelled, err := r.ReleaseMaxDurationRuns(ctx)
			if err != nil {
				logger.Error("max duration sweep failed", slog.Any("err", err))
				continue
			}
			if cancelled > 0 {
				logger.Warn("cancelled flow_runs exceeding max duration",
					slog.Int64("cancelled", cancelled))
			}
		}
	}
}

// ReleaseStaleRuns libera runs stale y marca los que exceden crash-loop
// threshold como failed. Retorna (released, crashLoopFailed, err).
func (r *Runner) ReleaseStaleRuns(ctx context.Context, stale time.Duration, maxRecoveries int) (int64, int64, error) {
	// Primero: marcar como failed los que exceden el crash-loop threshold
	var crashLoopCount int64
	if maxRecoveries > 0 {
		tag, err := r.Pool.Exec(ctx, `
			UPDATE flow_runs
			SET status = 'failed',
			    error = COALESCE(error, '') || ' [crash-loop: recovery_count >= ' || $1::text || ']',
			    finished_at = NOW(),
			    worker_id = NULL
			WHERE status = 'running'
			  AND last_heartbeat_at IS NOT NULL
			  AND last_heartbeat_at < NOW() - $2::interval
			  AND recovery_count >= $1
		`, maxRecoveries, stale)
		if err != nil {
			return 0, 0, err
		}
		crashLoopCount = tag.RowsAffected()
	}

	// Liberar los demás: limpiar worker_id, incrementar recovery_count,
	// dejar en running para que ClaimRun los tome.
	tag, err := r.Pool.Exec(ctx, `
		UPDATE flow_runs
		SET worker_id = NULL,
		    last_heartbeat_at = NOW(),
		    recovery_count = recovery_count + 1
		WHERE status = 'running'
		  AND last_heartbeat_at IS NOT NULL
		  AND last_heartbeat_at < NOW() - $1::interval
		  AND (worker_id IS NOT NULL OR recovery_count = 0)
	`, stale)
	if err != nil {
		return 0, crashLoopCount, err
	}

	// Log audit por cada run liberado (opcional, se podría hacer con RETURNING)
	return tag.RowsAffected(), crashLoopCount, nil
}

type maxDurationRun struct {
	ID         uuid.UUID
	StartedAt  time.Time
	MaxSeconds int
}

// ReleaseMaxDurationRuns marca como failed los flow_runs que exceden el
// max_flow_duration_seconds configurado per-org (issue-33.3).
// Cancela el context de los runs que están siendo ejecutados localmente.
// Retorna (cancelled, err).
func (r *Runner) ReleaseMaxDurationRuns(ctx context.Context) (int64, error) {
	rows, err := r.Pool.Query(ctx, `
		UPDATE flow_runs fr
		SET status = 'failed',
		    error = COALESCE(error, '') || ' [max_duration_exceeded]',
		    finished_at = NOW(),
		    worker_id = NULL
		FROM org_flow_config ofc
		WHERE fr.status = 'running'
		  AND fr.started_at IS NOT NULL
		  AND fr.started_at < NOW() - (ofc.max_flow_duration_seconds * INTERVAL '1 second')
		RETURNING fr.id, fr.started_at, ofc.max_flow_duration_seconds
	`)
	if err != nil {
		return 0, fmt.Errorf("max duration release query: %w", err)
	}
	defer rows.Close()

	var cancelled int64
	var runs []maxDurationRun
	for rows.Next() {
		var run maxDurationRun
		if err := rows.Scan(&run.ID, &run.StartedAt, &run.MaxSeconds); err != nil {
			return cancelled, fmt.Errorf("scan max duration run: %w", err)
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return cancelled, err
	}

	logger := slog.Default()
	for _, run := range runs {
		actualDuration := time.Since(run.StartedAt).Round(time.Second)
		budgetDuration := time.Duration(run.MaxSeconds) * time.Second

		logger.Warn("flow_run cancelled by max_duration",
			slog.String("flow_run_id", run.ID.String()),
			slog.String("duration_seconds", actualDuration.String()),
			slog.String("budget_seconds", budgetDuration.String()),
		)

		r.runContextsMu.Lock()
		cancel, ok := r.runContexts[run.ID]
		r.runContextsMu.Unlock()
		if ok {
			cancel()
		}

		if r.Metrics != nil && r.Metrics.FlowRunCancelledByMaxDuration != nil {
			r.Metrics.FlowRunCancelledByMaxDuration.WithLabelValues("").Inc()
		}
		cancelled++
	}
	return cancelled, nil
}
