// issue-11.2 selfhosted-runner — un runner externo (deployed por el customer
// en su VPC) consume tareas vía HTTP polling, ejecuta localmente, y devuelve
// resultados. Permite ejecutar cargas que no pueden salir del perímetro
// (privacy, compliance) sin que Domain SaaS las vea.
//
// Arquitectura:
//
//	Domain SaaS               Cliente VPC
//	┌─────────────┐           ┌──────────────────┐
//	│   queue     │   poll    │  selfhosted      │
//	│             │ ◀──────── │  runner agent    │
//	│             │  ─────▶   │  (binary deploy) │
//	│             │  task     │                  │
//	│             │  ◀───     │  result          │
//	└─────────────┘           └──────────────────┘
//
// Auth: API key dedicado al runner (RBAC con scope runners:claim/return).
package selfhosted

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

// RunnerStatus refleja la salud del runner.
type RunnerStatus string

const (
	StatusOnline   RunnerStatus = "online"
	StatusDegraded RunnerStatus = "degraded" // misses heartbeats pero sigue activo
	StatusOffline  RunnerStatus = "offline"
)

// Runner es un selfhosted deployed por la org.
type Runner struct {
	ID             uuid.UUID    `json:"id"`
	OrganizationID uuid.UUID    `json:"organization_id"`
	Name           string       `json:"name"`
	Labels         []string     `json:"labels"` // pool tags: gpu, eu-region, etc
	APIKeyHash     string       `json:"-"`
	LastHeartbeat  *time.Time   `json:"last_heartbeat,omitempty"`
	Status         RunnerStatus `json:"status"`
	CreatedAt      time.Time    `json:"created_at"`
}

// Task es trabajo encolado para algún runner.
type Task struct {
	ID             uuid.UUID       `json:"id"`
	OrganizationID uuid.UUID       `json:"organization_id"`
	Kind           string          `json:"kind"` // agent_run | flow_step | skill_call
	RequiredLabels []string        `json:"required_labels,omitempty"`
	Payload        json.RawMessage `json:"payload"`
	Status         string          `json:"status"` // queued | claimed | done | failed
	ClaimedBy      *uuid.UUID      `json:"claimed_by,omitempty"`
	ClaimedAt      *time.Time      `json:"claimed_at,omitempty"`
	CompletedAt    *time.Time      `json:"completed_at,omitempty"`
	Result         json.RawMessage `json:"result,omitempty"`
	Error          string          `json:"error,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}

// Service controla queue + runners.
type Service struct {
	Pool *pgxpool.Pool

	// HeartbeatTimeout marca runner offline si último heartbeat > N (default 60s).
	HeartbeatTimeout time.Duration
	// ClaimTimeout devuelve la task al queue si no se completa en N (default 5min).
	ClaimTimeout time.Duration
}

var (
	ErrTaskNotFound  = errors.New("task not found")
	ErrRunnerOffline = errors.New("runner offline")
	ErrNoTask        = errors.New("no task available")
)

// RegisterRunner persiste un selfhosted runner. Idempotent por name
// (single-org: el UNIQUE (org, name) se dropeó en 000145, ahora es solo
// name; el caller garantiza unicidad via app).
func (s *Service) RegisterRunner(ctx context.Context, orgID uuid.UUID, name, apiKeyHash string, labels []string) (*Runner, error) {
	_ = orgID
	var r Runner
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO runner_selfhosted (name, api_key_hash, labels)
		VALUES ($1, $2, $3)
		ON CONFLICT (name) DO UPDATE
		  SET api_key_hash = EXCLUDED.api_key_hash, labels = EXCLUDED.labels
		RETURNING id, name, labels, last_heartbeat, created_at`,
		name, apiKeyHash, labels,
	).Scan(&r.ID, &r.Name, &r.Labels, &r.LastHeartbeat, &r.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("register: %w", err)
	}
	r.Status = StatusOnline
	return &r, nil
}

// Heartbeat actualiza last_heartbeat.
func (s *Service) Heartbeat(ctx context.Context, runnerID uuid.UUID) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE runner_selfhosted SET last_heartbeat = now() WHERE id = $1`, runnerID,
	)
	return err
}

// EnqueueTask agrega una task a la queue.
func (s *Service) EnqueueTask(ctx context.Context, orgID uuid.UUID, kind string, requiredLabels []string, payload []byte) (*Task, error) {
	_ = orgID
	var t Task
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO runner_selfhosted_tasks (kind, required_labels, payload, status)
		VALUES ($1, $2, $3, 'queued')
		RETURNING id, kind, required_labels, payload, status, created_at`,
		kind, requiredLabels, payload,
	).Scan(&t.ID, &t.Kind, &t.RequiredLabels, &t.Payload, &t.Status, &t.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("enqueue: %w", err)
	}
	return &t, nil
}

// ClaimTask intenta reclamar una task disponible para el runner.
// Atómico via SELECT ... FOR UPDATE SKIP LOCKED. Filtra por labels matching.
func (s *Service) ClaimTask(ctx context.Context, runnerID uuid.UUID) (*Task, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	// Carga labels del runner
	// ISSUE-21.6: organization_id omitido del SELECT.
	var labels []string
	err = tx.QueryRow(ctx,
		`SELECT labels FROM runner_selfhosted WHERE id = $1`,
		runnerID,
	).Scan(&labels)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRunnerOffline
	}
	if err != nil {
		return nil, fmt.Errorf("lookup runner: %w", err)
	}

	var t Task
	// ISSUE-21.6 Fase D clean: single-org. SELECT sin organization_id.
	err = tx.QueryRow(ctx, `
		SELECT id, kind, required_labels, payload, status, created_at
		FROM runner_selfhosted_tasks
		WHERE status = 'queued'
		  AND (required_labels = '{}' OR required_labels <@ $1::text[])
		ORDER BY created_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT 1`,
		labels,
	).Scan(&t.ID, &t.Kind, &t.RequiredLabels, &t.Payload, &t.Status, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoTask
	}
	if err != nil {
		return nil, fmt.Errorf("claim select: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE runner_selfhosted_tasks
		SET status = 'claimed', claimed_by = $1, claimed_at = now()
		WHERE id = $2`,
		runnerID, t.ID,
	); err != nil {
		return nil, fmt.Errorf("claim update: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	now := time.Now()
	t.ClaimedBy = &runnerID
	t.ClaimedAt = &now
	t.Status = "claimed"
	return &t, nil
}

// ReturnResult marca la task como completed con result o failed con error.
func (s *Service) ReturnResult(ctx context.Context, runnerID, taskID uuid.UUID, result []byte, errMsg string) error {
	status := "done"
	if errMsg != "" {
		status = "failed"
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE runner_selfhosted_tasks
		SET status = $1, result = $2, error = NULLIF($3, ''), completed_at = now()
		WHERE id = $4 AND claimed_by = $5`,
		status, result, errMsg, taskID, runnerID,
	)
	if err != nil {
		return fmt.Errorf("return result: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrTaskNotFound
	}
	return nil
}

// ReclaimExpiredTasks devuelve al queue tasks claimed sin completar > ClaimTimeout.
// Cron debe llamar este método periódicamente.
func (s *Service) ReclaimExpiredTasks(ctx context.Context) (int, error) {
	timeout := s.ClaimTimeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	cutoff := time.Now().Add(-timeout)
	tag, err := s.Pool.Exec(ctx, `
		UPDATE runner_selfhosted_tasks
		SET status = 'queued', claimed_by = NULL, claimed_at = NULL
		WHERE status = 'claimed' AND claimed_at < $1`,
		cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("reclaim: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
