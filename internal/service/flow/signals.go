// HU-09.8 external-signals — inyecta señales externas a flows en ejecución.
//
// Tabla flow_signals (migration 000062) buffer-able. El executor del flow
// polea signals para sus runs activos cada tick; un step puede esperar una
// señal con timeout configurado.
package flow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Signal representa un evento dirigido a un flow_run específico.
type Signal struct {
	ID         int64           `json:"id"`
	FlowRunID  uuid.UUID       `json:"flow_run_id"`
	StepKey    *string         `json:"step_key,omitempty"`
	Name       string          `json:"name"`          // ej: "approve", "cancel", "input_received"
	Payload    json.RawMessage `json:"payload,omitempty"`
	DeliveredAt *time.Time     `json:"delivered_at,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}

// SignalStore gestiona signals.
type SignalStore struct {
	Pool *pgxpool.Pool
}

var ErrSignalNotFound = errors.New("signal not found")

// Send registra un signal nuevo. Idempotencia: si name='approve' ya existe
// y no-delivered para este run+step, se actualiza el payload en lugar de duplicar.
func (s *SignalStore) Send(ctx context.Context, flowRunID uuid.UUID, stepKey *string, name string, payload []byte) (*Signal, error) {
	var sig Signal
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO flow_signals (flow_run_id, step_key, name, payload)
		VALUES ($1, $2, $3, $4)
		RETURNING id, flow_run_id, step_key, name, payload, delivered_at, created_at`,
		flowRunID, stepKey, name, payload,
	).Scan(&sig.ID, &sig.FlowRunID, &sig.StepKey, &sig.Name, &sig.Payload,
		&sig.DeliveredAt, &sig.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert signal: %w", err)
	}
	return &sig, nil
}

// Consume devuelve el próximo signal no-delivered para (flowRun, stepKey, name)
// marcándolo como delivered en la misma tx. Atómico (SELECT FOR UPDATE SKIP LOCKED).
func (s *SignalStore) Consume(ctx context.Context, flowRunID uuid.UUID, stepKey *string, name string) (*Signal, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	var sig Signal
	query := `
		SELECT id, flow_run_id, step_key, name, payload, delivered_at, created_at
		FROM flow_signals
		WHERE flow_run_id = $1
		  AND name = $2
		  AND delivered_at IS NULL
		  AND ($3::TEXT IS NULL OR step_key IS NULL OR step_key = $3)
		ORDER BY created_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED`
	err = tx.QueryRow(ctx, query, flowRunID, name, stepKey).Scan(
		&sig.ID, &sig.FlowRunID, &sig.StepKey, &sig.Name, &sig.Payload,
		&sig.DeliveredAt, &sig.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSignalNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE flow_signals SET delivered_at = now() WHERE id = $1`, sig.ID,
	); err != nil {
		return nil, fmt.Errorf("mark delivered: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	now := time.Now()
	sig.DeliveredAt = &now
	return &sig, nil
}

// Wait poll-loop a Consume con timeout. Útil para steps que bloquean.
func (s *SignalStore) Wait(ctx context.Context, flowRunID uuid.UUID, stepKey *string, name string, timeout time.Duration, pollInterval time.Duration) (*Signal, error) {
	if pollInterval <= 0 {
		pollInterval = 500 * time.Millisecond
	}
	deadline := time.Now().Add(timeout)
	for {
		sig, err := s.Consume(ctx, flowRunID, stepKey, name)
		if err == nil {
			return sig, nil
		}
		if !errors.Is(err, ErrSignalNotFound) {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, ErrSignalNotFound
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// List devuelve signals (delivered o pending) para un flow_run.
func (s *SignalStore) List(ctx context.Context, flowRunID uuid.UUID, includeDelivered bool) ([]Signal, error) {
	q := `SELECT id, flow_run_id, step_key, name, payload, delivered_at, created_at
	      FROM flow_signals WHERE flow_run_id = $1`
	if !includeDelivered {
		q += ` AND delivered_at IS NULL`
	}
	q += ` ORDER BY created_at ASC`
	rows, err := s.Pool.Query(ctx, q, flowRunID)
	if err != nil {
		return nil, fmt.Errorf("list signals: %w", err)
	}
	defer rows.Close()

	var out []Signal
	for rows.Next() {
		var sig Signal
		if err := rows.Scan(&sig.ID, &sig.FlowRunID, &sig.StepKey, &sig.Name,
			&sig.Payload, &sig.DeliveredAt, &sig.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		out = append(out, sig)
	}
	return out, rows.Err()
}
