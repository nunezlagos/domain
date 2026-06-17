// Package systemcron — crons internos de salud operacional (issue-08.11, issue-08.12).
//
// NO confundir con crons user-defined (tabla `crons`, REQ-10). Estos son
// hardcoded en código + se enable por config flag. Sólo el leader del cluster
// los ejecuta (vía internal/scheduler/leader).
package systemcron

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/metrics"
)

// HeartbeatWatcher detecta flow_run_steps stuck (status=running + heartbeat>timeout)
// y los marca como failed disparando saga_compensation_log según retry_policy.
// issue-08.11.
//
// Diseño: NO maneja leader election propia. Se asume que el caller lo invoca
// dentro de un block RunAsLeader (ver internal/scheduler/leader). Esto sigue
// el patrón del cron scheduler existente (issue-10.1) y evita doble-lock.
type HeartbeatWatcher struct {
	Pool    *pgxpool.Pool
	Metrics *metrics.Registry
	Timeout time.Duration // default 5min
	Tick    time.Duration // default 60s
	Batch   int           // default 100
	Logger  *slog.Logger
}

// StuckStep representa un step detectado como stuck (para tests + logging).
type StuckStep struct {
	StepID      string
	FlowRunID   string
	StepKey     string
	RetryPolicy string
}

// Start corre el loop hasta que ctx se cancele. Sólo ejecuta tick si es leader.
func (w *HeartbeatWatcher) Start(ctx context.Context) {
	if w.Tick == 0 {
		w.Tick = 60 * time.Second
	}
	if w.Timeout == 0 {
		w.Timeout = 5 * time.Minute
	}
	if w.Batch == 0 {
		w.Batch = 100
	}
	logger := w.Logger
	if logger == nil {
		logger = slog.Default()
	}

	logger.Info("heartbeat-watcher started",
		slog.Duration("tick", w.Tick),
		slog.Duration("timeout", w.Timeout))

	ticker := time.NewTicker(w.Tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("heartbeat-watcher stopping")
			return
		case <-ticker.C:
			w.runTick(ctx, logger)
		}
	}
}

func (w *HeartbeatWatcher) runTick(ctx context.Context, logger *slog.Logger) {
	stuck, err := w.DetectAndMark(ctx)
	if err != nil {
		logger.Error("heartbeat-watcher tick failed", slog.Any("err", err))
		w.observeTick("error")
		return
	}
	if len(stuck) > 0 {
		logger.Info("heartbeat-watcher detected stuck steps",
			slog.Int("count", len(stuck)))
	}
	w.observeTick("ok")
}

// DetectAndMark ejecuta una pasada: detecta stuck + marca failed + dispara saga.
// Exportado para tests + invocación manual.
func (w *HeartbeatWatcher) DetectAndMark(ctx context.Context) ([]StuckStep, error) {
	tx, err := w.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Detección con FOR UPDATE SKIP LOCKED para race-safety con clientes
	// que están actualizando last_heartbeat_at concurrentemente.
	query := `
		SELECT
			s.id, s.flow_run_id, s.step_key,
			COALESCE(at.metadata->>'retry_policy', 'idempotent')
		FROM flow_run_steps s
		JOIN flow_runs fr ON fr.id = s.flow_run_id
		LEFT JOIN agents a ON a.slug = s.step_key
		LEFT JOIN agent_templates at ON at.slug = s.step_key
		WHERE s.status = 'running'
		  AND (
		    s.last_heartbeat_at < NOW() - $1::interval
		    OR (s.last_heartbeat_at IS NULL AND s.started_at < NOW() - $1::interval)
		  )
		FOR UPDATE OF s SKIP LOCKED
		LIMIT $2
	`
	rows, err := tx.Query(ctx, query, w.Timeout.String(), w.Batch)
	if err != nil {
		return nil, fmt.Errorf("detect query: %w", err)
	}
	var stuck []StuckStep
	for rows.Next() {
		var s StuckStep
		if err := rows.Scan(&s.StepID, &s.FlowRunID, &s.StepKey, &s.RetryPolicy); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan stuck step: %w", err)
		}
		stuck = append(stuck, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}

	if len(stuck) == 0 {
		_ = tx.Commit(ctx)
		return nil, nil
	}

	// Marcar failed + saga event por cada stuck step
	for _, s := range stuck {
		if err := w.markFailed(ctx, tx, s); err != nil {
			return nil, fmt.Errorf("mark failed step %s: %w", s.StepID, err)
		}
		w.observeStuck(s)
	}

	// Cerrar flow_runs si todos sus steps están terminales.
	// Usamos un set de IDs únicos en lugar de array typing (más portable).
	seen := make(map[string]struct{}, len(stuck))
	for _, s := range stuck {
		if _, ok := seen[s.FlowRunID]; ok {
			continue
		}
		seen[s.FlowRunID] = struct{}{}
		if _, err := tx.Exec(ctx, `
			UPDATE flow_runs SET status = 'failed', updated_at = NOW()
			WHERE id = $1
			  AND status = 'running'
			  AND NOT EXISTS (
			    SELECT 1 FROM flow_run_steps s
			    WHERE s.flow_run_id = flow_runs.id
			      AND s.status NOT IN ('completed', 'failed', 'skipped', 'cancelled')
			  )
		`, s.FlowRunID); err != nil {
			return nil, fmt.Errorf("update flow_run %s: %w", s.FlowRunID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return stuck, nil
}

func (w *HeartbeatWatcher) markFailed(ctx context.Context, tx pgx.Tx, s StuckStep) error {
	// Marca step como failed con error reason
	_, err := tx.Exec(ctx, `
		UPDATE flow_run_steps
		SET status = 'failed',
		    error = 'heartbeat_timeout',
		    completed_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
	`, s.StepID)
	if err != nil {
		return fmt.Errorf("update step: %w", err)
	}

	// Saga event según retry policy
	sagaEvent := sagaEventFor(s.RetryPolicy)
	_, err = tx.Exec(ctx, `
		INSERT INTO saga_compensation_log
		  (run_id, original_step, compensate_ran, success, payload)
		VALUES ($1, $2, $3, false, $4)
	`, s.FlowRunID, s.StepKey, sagaEvent,
		fmt.Sprintf(`{"reason":"heartbeat_timeout","retry_policy":"%s"}`, s.RetryPolicy))
	if err != nil {
		return fmt.Errorf("insert saga: %w", err)
	}
	return nil
}

func sagaEventFor(retryPolicy string) string {
	switch retryPolicy {
	case "require-cleanup":
		return "cleanup_required"
	case "re-emit":
		return "reemit_eligible"
	default:
		return "auto_retry_eligible"
	}
}

func (w *HeartbeatWatcher) observeTick(result string) {
	if w.Metrics != nil && w.Metrics.HeartbeatWatcherTicksTotal != nil {
		w.Metrics.HeartbeatWatcherTicksTotal.WithLabelValues(result).Inc()
	}
}

func (w *HeartbeatWatcher) observeStuck(s StuckStep) {
	if w.Metrics != nil && w.Metrics.HeartbeatWatcherStuckTotal != nil {
		w.Metrics.HeartbeatWatcherStuckTotal.
			WithLabelValues("unknown", s.StepKey, "heartbeat_timeout").Inc()
	}
}

// ErrNotLeader sentinel reservado (no usado actualmente, el watcher
// asume estar dentro de un block RunAsLeader del caller).
var ErrNotLeader = errors.New("not leader")
