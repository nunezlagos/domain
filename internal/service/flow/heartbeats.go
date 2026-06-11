// issue-09.10 step-heartbeats — detecta steps long-running que se colgaron.
//
// Cada step en ejecución registra heartbeat cada N segundos. Watchdog
// background corre cada minute y marca steps sin heartbeat > threshold
// como failed (timeout). Permite a otros runners retomar (issue-26.2 leader).
package flow

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrHeartbeatMissed se retorna cuando un step supera el threshold sin heartbeat.
var ErrHeartbeatMissed = errors.New("heartbeat missed")

// ValidateProgress chequea que progress esté en [0,1].
func ValidateProgress(progress float64) error {
	if progress < 0 || progress > 1 {
		return fmt.Errorf("progress must be between 0 and 1, got %f", progress)
	}
	return nil
}

// HeartbeatStore tracks heartbeats de flow_run_steps en ejecución.
type HeartbeatStore struct {
	Pool *pgxpool.Pool
	// HeartbeatTimeout es el threshold post-cual un step se marca stuck.
	// Default 5 minutos si <=0.
	HeartbeatTimeout time.Duration
}

// Beat actualiza el timestamp del step. Llamado por el runner cada ~30s.
func (s *HeartbeatStore) Beat(ctx context.Context, stepID uuid.UUID) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE flow_run_steps SET last_heartbeat_at = now() WHERE id = $1`,
		stepID,
	)
	if err != nil {
		return fmt.Errorf("update heartbeat: %w", err)
	}
	return nil
}

// BeatWithProgress actualiza heartbeat + progress + mensaje en un step.
// progress debe estar en [0,1]; message es un texto libre (ej: "descargando 30%").
func (s *HeartbeatStore) BeatWithProgress(ctx context.Context, stepID uuid.UUID, progress float64, message string) error {
	if err := ValidateProgress(progress); err != nil {
		return err
	}
	_, err := s.Pool.Exec(ctx, `
		UPDATE flow_run_steps
		SET last_heartbeat_at = now(), progress = $2, progress_message = $3
		WHERE id = $1`,
		stepID, progress, message,
	)
	if err != nil {
		return fmt.Errorf("beat with progress: %w", err)
	}
	return nil
}

// UpdateProgress actualiza solo progress + message sin modificar heartbeat.
func (s *HeartbeatStore) UpdateProgress(ctx context.Context, stepID uuid.UUID, progress float64, message string) error {
	if err := ValidateProgress(progress); err != nil {
		return err
	}
	_, err := s.Pool.Exec(ctx,
		`UPDATE flow_run_steps SET progress = $2, progress_message = $3 WHERE id = $1`,
		stepID, progress, message,
	)
	if err != nil {
		return fmt.Errorf("update progress: %w", err)
	}
	return nil
}

// FindStuck devuelve steps in 'running' sin heartbeat reciente.
func (s *HeartbeatStore) FindStuck(ctx context.Context, limit int) ([]uuid.UUID, error) {
	timeout := s.HeartbeatTimeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	cutoff := time.Now().Add(-timeout)
	rows, err := s.Pool.Query(ctx,
		`SELECT id FROM flow_run_steps
		 WHERE status = 'running'
		   AND (last_heartbeat_at IS NULL OR last_heartbeat_at < $1)
		   AND started_at < $1
		 LIMIT $2`,
		cutoff, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("find stuck: %w", err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// MarkFailed marca steps stuck como failed con razón "heartbeat_timeout".
func (s *HeartbeatStore) MarkFailed(ctx context.Context, stepIDs []uuid.UUID) (int, error) {
	if len(stepIDs) == 0 {
		return 0, nil
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE flow_run_steps
		SET status = 'failed',
		    completed_at = now(),
		    error = COALESCE(error, 'heartbeat_timeout: no heartbeat in threshold')
		WHERE id = ANY($1::uuid[]) AND status = 'running'`,
		stepIDs,
	)
	if err != nil {
		return 0, fmt.Errorf("mark failed: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// FindStuckWithCustomThreshold usa el per-step heartbeat_threshold_seconds
// en vez del global HeartbeatTimeout. Respeta el valor individual de cada step.
func (s *HeartbeatStore) FindStuckWithCustomThreshold(ctx context.Context, limit int) ([]uuid.UUID, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id FROM flow_run_steps
		WHERE status = 'running'
		  AND last_heartbeat_at IS NOT NULL
		  AND last_heartbeat_at < NOW() - COALESCE(
		        make_interval(secs => heartbeat_threshold_seconds),
		        make_interval(secs => 120)
		      )
		LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("find stuck custom: %w", err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Watchdog corre periódicamente buscando steps stuck.
// Usado por leader-elected scheduler (issue-26.2).
func (s *HeartbeatStore) Watchdog(ctx context.Context, interval time.Duration, logger *slog.Logger) error {
	if interval <= 0 {
		interval = time.Minute
	}
	if logger == nil {
		logger = slog.Default()
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			stuck, err := s.FindStuck(ctx, 100)
			if err != nil {
				logger.Warn("heartbeat watchdog find stuck failed", slog.Any("err", err))
				continue
			}
			if len(stuck) == 0 {
				continue
			}
			n, err := s.MarkFailed(ctx, stuck)
			if err != nil {
				logger.Warn("heartbeat watchdog mark failed", slog.Any("err", err))
				continue
			}
			logger.Warn("heartbeat watchdog marked stuck steps as failed",
				slog.Int("count", n), slog.Int("candidates", len(stuck)))
		}
	}
}
