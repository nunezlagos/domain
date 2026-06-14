// Historial de ejecuciones de crons — issue-10.1 (tabla cron_executions).
package cron

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Execution una entrada del historial.
type Execution struct {
	ID         int64      `json:"id"`
	CronID     uuid.UUID  `json:"cron_id"`
	Status     string     `json:"status"`
	TargetType string     `json:"target_type"`
	Error      string     `json:"error,omitempty"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	DurationMS *int       `json:"duration_ms,omitempty"`
}

// StartExecution registra el inicio. Si ya hay una ejecución running para
// el mismo cron (overlap), no inserta running: registra skipped_overlap y
// devuelve skipped=true para que el scheduler no dispare el target.
func (s *Service) StartExecution(ctx context.Context, cronID uuid.UUID, targetType string) (id int64, skipped bool, err error) {
	// INSERT condicional: solo si NO existe otra running. La subquery dentro
	// del INSERT evita la race entre check y registro (statement único).
	err = s.Pool.QueryRow(ctx,
		`INSERT INTO cron_executions (cron_id, status, target_type)
		 SELECT $1, 'running', $2
		 WHERE NOT EXISTS (
		   SELECT 1 FROM cron_executions WHERE cron_id = $1 AND status = 'running')
		 RETURNING id`,
		cronID, targetType).Scan(&id)
	if err == nil {
		return id, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, false, fmt.Errorf("start execution: %w", err)
	}
	// Overlap: dejar rastro en el historial
	err = s.Pool.QueryRow(ctx,
		`INSERT INTO cron_executions (cron_id, status, target_type, finished_at, duration_ms)
		 VALUES ($1, 'skipped_overlap', $2, NOW(), 0)
		 RETURNING id`,
		cronID, targetType).Scan(&id)
	if err != nil {
		return 0, true, fmt.Errorf("record overlap skip: %w", err)
	}
	return id, true, nil
}

// FinishExecution cierra la entrada con completed o failed.
func (s *Service) FinishExecution(ctx context.Context, id int64, execErr error) error {
	status := "completed"
	var errMsg any
	if execErr != nil {
		status = "failed"
		errMsg = execErr.Error()
	}
	_, err := s.Pool.Exec(ctx,
		`UPDATE cron_executions
		 SET status = $2, error = $3, finished_at = NOW(),
		     duration_ms = (EXTRACT(EPOCH FROM (NOW() - started_at)) * 1000)::INT
		 WHERE id = $1 AND status = 'running'`,
		id, status, errMsg)
	if err != nil {
		return fmt.Errorf("finish execution: %w", err)
	}
	return nil
}

// History ejecuciones de un cron, más reciente primero.
func (s *Service) History(ctx context.Context, cronID uuid.UUID, limit int) ([]Execution, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.Pool.Query(ctx,
		`SELECT id, cron_id, status, target_type, COALESCE(error,''),
		        started_at, finished_at, duration_ms
		 FROM cron_executions WHERE cron_id = $1
		 ORDER BY started_at DESC, id DESC LIMIT $2`, cronID, limit)
	if err != nil {
		return nil, fmt.Errorf("history: %w", err)
	}
	defer rows.Close()
	var out []Execution
	for rows.Next() {
		var e Execution
		if err := rows.Scan(&e.ID, &e.CronID, &e.Status, &e.TargetType, &e.Error,
			&e.StartedAt, &e.FinishedAt, &e.DurationMS); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
