// Package session — HU-03.2 sessions lifecycle.
//
// Una session agrupa observations + prompts de una conversación/run.
// Lifecycle: Start → (durante) → End con summary.
// Active = ended_at IS NULL AND deleted_at IS NULL.
package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/saargo/domain/internal/audit"
)

var (
	ErrNotFound      = errors.New("session not found")
	ErrAlreadyEnded  = errors.New("session already ended")
	ErrTitleRequired = errors.New("title required")
)

const (
	StatusActive    = "active"
	StatusCompleted = "completed"
)

type Session struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	ProjectID      *uuid.UUID
	UserID         uuid.UUID
	Title          string
	Summary        string
	Tags           []string
	StartedAt      time.Time
	EndedAt        *time.Time
	CreatedAt      time.Time
}

func (s *Session) Status() string {
	if s.EndedAt != nil {
		return StatusCompleted
	}
	return StatusActive
}

type StartInput struct {
	OrganizationID uuid.UUID
	UserID         uuid.UUID
	ProjectID      *uuid.UUID
	Title          string
	Tags           []string
}

type Service struct {
	Pool  *pgxpool.Pool
	Audit audit.Recorder
}

func (s *Service) Start(ctx context.Context, in StartInput) (*Session, error) {
	if strings.TrimSpace(in.Title) == "" {
		return nil, ErrTitleRequired
	}
	if in.Tags == nil {
		in.Tags = []string{}
	}
	var sess Session
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO sessions (organization_id, project_id, user_id, title, tags)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, organization_id, project_id, user_id, title, COALESCE(summary,''),
		           tags, started_at, ended_at, created_at`,
		in.OrganizationID, in.ProjectID, in.UserID, in.Title, in.Tags,
	).Scan(&sess.ID, &sess.OrganizationID, &sess.ProjectID, &sess.UserID, &sess.Title, &sess.Summary,
		&sess.Tags, &sess.StartedAt, &sess.EndedAt, &sess.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
			OrganizationID: &in.OrganizationID,
			ActorID:        &in.UserID,
			ActorType:      audit.ActorUser,
			Action:         "session.started",
			EntityType:     "session",
			EntityID:       &sess.ID,
			NewValues:      map[string]any{"title": in.Title},
		})
	}
	return &sess, nil
}

// End cierra una session. summary opcional (puede ser ""). Si ya estaba cerrada
// retorna ErrAlreadyEnded (no idempotente — el caller decide qué hacer).
func (s *Service) End(ctx context.Context, id, actorID uuid.UUID, summary string) (*Session, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var sess Session
	err = tx.QueryRow(ctx,
		`SELECT id, organization_id, project_id, user_id, title, COALESCE(summary,''),
		        tags, started_at, ended_at, created_at
		 FROM sessions WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, id,
	).Scan(&sess.ID, &sess.OrganizationID, &sess.ProjectID, &sess.UserID, &sess.Title, &sess.Summary,
		&sess.Tags, &sess.StartedAt, &sess.EndedAt, &sess.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query session: %w", err)
	}
	if sess.EndedAt != nil {
		return nil, ErrAlreadyEnded
	}

	now := time.Now().UTC()
	_, err = tx.Exec(ctx,
		`UPDATE sessions SET ended_at = $2, summary = $3 WHERE id = $1`,
		id, now, nullStr(summary))
	if err != nil {
		return nil, fmt.Errorf("end session: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	sess.EndedAt = &now
	sess.Summary = summary

	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
			OrganizationID: &sess.OrganizationID,
			ActorID:        &actorID,
			ActorType:      audit.ActorUser,
			Action:         "session.ended",
			EntityType:     "session",
			EntityID:       &id,
			NewValues:      map[string]any{"summary_chars": len(summary)},
		})
	}
	return &sess, nil
}

// GetByID retorna sesión sin importar status.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Session, error) {
	return s.queryOne(ctx, `WHERE id = $1 AND deleted_at IS NULL`, id)
}

// GetActive devuelve la session activa más reciente del user en el project.
// Si projectID == uuid.Nil devuelve la activa más reciente del user sin filtro.
func (s *Service) GetActive(ctx context.Context, userID, projectID uuid.UUID) (*Session, error) {
	if projectID == uuid.Nil {
		return s.queryOne(ctx,
			`WHERE user_id = $1 AND ended_at IS NULL AND deleted_at IS NULL
			 ORDER BY started_at DESC LIMIT 1`, userID)
	}
	return s.queryOne(ctx,
		`WHERE user_id = $1 AND project_id = $2 AND ended_at IS NULL
		   AND deleted_at IS NULL
		 ORDER BY started_at DESC LIMIT 1`, userID, projectID)
}

// List devuelve sessions del user (más recientes primero).
func (s *Service) List(ctx context.Context, userID uuid.UUID, limit int) ([]Session, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.Pool.Query(ctx,
		`SELECT id, organization_id, project_id, user_id, title, COALESCE(summary,''),
		        tags, started_at, ended_at, created_at
		 FROM sessions
		 WHERE user_id = $1 AND deleted_at IS NULL
		 ORDER BY started_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		var sess Session
		if err := rows.Scan(&sess.ID, &sess.OrganizationID, &sess.ProjectID, &sess.UserID, &sess.Title, &sess.Summary,
			&sess.Tags, &sess.StartedAt, &sess.EndedAt, &sess.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	return out, rows.Err()
}

func (s *Service) queryOne(ctx context.Context, where string, args ...any) (*Session, error) {
	var sess Session
	q := `SELECT id, organization_id, project_id, user_id, title, COALESCE(summary,''),
	        tags, started_at, ended_at, created_at FROM sessions ` + where
	err := s.Pool.QueryRow(ctx, q, args...).Scan(
		&sess.ID, &sess.OrganizationID, &sess.ProjectID, &sess.UserID, &sess.Title, &sess.Summary,
		&sess.Tags, &sess.StartedAt, &sess.EndedAt, &sess.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query session: %w", err)
	}
	return &sess, nil
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
