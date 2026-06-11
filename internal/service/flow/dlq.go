// issue-09.4 — Dead Letter Queue para steps con fallo permanente
// (retries agotados sin política de recuperación).
package flow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DLQEntry es un fallo permanente registrado.
type DLQEntry struct {
	ID         uuid.UUID  `json:"id"`
	FlowRunID  *uuid.UUID `json:"flow_run_id,omitempty"`
	FlowSlug   string     `json:"flow_slug"`
	StepKey    string     `json:"step_key"`
	Error      string     `json:"error"`
	Errors     []string   `json:"errors"` // mensajes de cada intento
	RetryCount int        `json:"retry_count"`
	FailedAt   time.Time  `json:"failed_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

// DLQStore persiste y consulta la dead letter queue.
type DLQStore struct {
	Pool *pgxpool.Pool
}

var ErrDLQNotFound = errors.New("dlq entry not found")

// Insert registra un fallo permanente. Best-effort desde el runner.
func (s *DLQStore) Insert(ctx context.Context, orgID uuid.UUID, runID *uuid.UUID,
	flowSlug, stepKey, errMsg string, attemptErrors []string, retryCount int) (*DLQEntry, error) {
	errorsJSON, _ := json.Marshal(attemptErrors)
	var e DLQEntry
	var errorsRaw []byte
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO dead_letter_queue
			(organization_id, flow_run_id, flow_slug, step_key, error, errors, retry_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, flow_run_id, flow_slug, step_key, error, errors, retry_count, failed_at, resolved_at`,
		orgID, runID, flowSlug, stepKey, errMsg, errorsJSON, retryCount,
	).Scan(&e.ID, &e.FlowRunID, &e.FlowSlug, &e.StepKey, &e.Error, &errorsRaw,
		&e.RetryCount, &e.FailedAt, &e.ResolvedAt)
	if err != nil {
		return nil, fmt.Errorf("dlq insert: %w", err)
	}
	_ = json.Unmarshal(errorsRaw, &e.Errors)
	return &e, nil
}

// List devuelve entries pendientes (no resueltas) de la org, más recientes primero.
func (s *DLQStore) List(ctx context.Context, orgID uuid.UUID, limit int) ([]DLQEntry, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id, flow_run_id, flow_slug, step_key, error, errors, retry_count, failed_at, resolved_at
		FROM dead_letter_queue
		WHERE organization_id = $1 AND resolved_at IS NULL
		ORDER BY failed_at DESC LIMIT $2`, orgID, limit)
	if err != nil {
		return nil, fmt.Errorf("dlq list: %w", err)
	}
	defer rows.Close()
	var out []DLQEntry
	for rows.Next() {
		var e DLQEntry
		var errorsRaw []byte
		if err := rows.Scan(&e.ID, &e.FlowRunID, &e.FlowSlug, &e.StepKey, &e.Error,
			&errorsRaw, &e.RetryCount, &e.FailedAt, &e.ResolvedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(errorsRaw, &e.Errors)
		out = append(out, e)
	}
	return out, rows.Err()
}

// Resolve marca la entry como resuelta (DELETE /api/v1/dlq/:id).
func (s *DLQStore) Resolve(ctx context.Context, orgID, id uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE dead_letter_queue SET resolved_at = NOW()
		WHERE id = $1 AND organization_id = $2 AND resolved_at IS NULL`, id, orgID)
	if err != nil {
		return fmt.Errorf("dlq resolve: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrDLQNotFound
	}
	return nil
}
