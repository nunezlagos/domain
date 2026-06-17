// Package session — issue-03.2 sessions lifecycle.
//
// Una session agrupa observations + prompts de una conversación/run.
// Lifecycle: Start → (durante) → End con summary.
// Active = ended_at IS NULL AND deleted_at IS NULL.
//
// HU-28.1: Service depende de Repository (interfaz) en vez de *pgxpool.Pool
// directo. Pool se mantiene público como deprecated para Strangler Fig.
package session

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
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
	// Pool — DEPRECATED (HU-28.1). Se mantiene público como Strangler Fig.
	Pool  *pgxpool.Pool
	Audit audit.Recorder

	repo Repository
}

// NewService construye el Service. Si repo es nil, se construye un
// pgRepository wrappeando pool (back-compat con struct literal).
func NewService(pool *pgxpool.Pool, audit audit.Recorder, repo Repository) *Service {
	if repo == nil && pool != nil {
		repo = NewPgRepository(pool)
	}
	return &Service{Pool: pool, Audit: audit, repo: repo}
}

func (s *Service) repository() Repository {
	if s.repo != nil {
		return s.repo
	}
	s.repo = NewPgRepository(s.Pool)
	return s.repo
}

func (s *Service) Start(ctx context.Context, in StartInput) (*Session, error) {
	if strings.TrimSpace(in.Title) == "" {
		return nil, ErrTitleRequired
	}
	if in.Tags == nil {
		in.Tags = []string{}
	}
	sess, err := s.repository().Insert(ctx, InsertParams{
		OrganizationID: in.OrganizationID,
		ProjectID:      in.ProjectID,
		UserID:         in.UserID,
		Title:          in.Title,
		Tags:           in.Tags,
	})
	if err != nil {
		return nil, err
	}
	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			OrganizationID: &in.OrganizationID,
			ActorID:        &in.UserID,
			ActorType:      audit.ActorUser,
			Action:         "session.started",
			EntityType:     "session",
			EntityID:       &sess.ID,
			NewValues:      map[string]any{"title": in.Title},
		})
	}
	return sess, nil
}

// End cierra una session. summary opcional (puede ser ""). Si ya estaba cerrada
// retorna ErrAlreadyEnded (no idempotente — el caller decide qué hacer).
//
// El repo maneja la tx con FOR UPDATE (reusa tx-context si existe).
func (s *Service) End(ctx context.Context, id, actorID uuid.UUID, summary string) (*Session, error) {
	now := time.Now().UTC()
	sess, err := s.repository().EndAndLoad(ctx, id, summary, now)
	if err != nil {
		return nil, err
	}
	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorID:        &actorID,
			ActorType:      audit.ActorUser,
			Action:         "session.ended",
			EntityType:     "session",
			EntityID:       &id,
			NewValues:      map[string]any{"summary_chars": len(summary)},
		})
	}
	return sess, nil
}

// GetByID retorna sesión sin importar status.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Session, error) {
	return s.repository().GetByID(ctx, id)
}

// GetActive devuelve la session activa más reciente del user en el project.
// Si projectID == uuid.Nil devuelve la activa más reciente del user sin filtro.
func (s *Service) GetActive(ctx context.Context, userID, projectID uuid.UUID) (*Session, error) {
	return s.repository().GetActive(ctx, userID, projectID)
}

// List devuelve sessions del user (más recientes primero).
func (s *Service) List(ctx context.Context, userID uuid.UUID, limit int) ([]Session, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.repository().List(ctx, userID, limit)
}

// CloseInactive cierra sesiones activas que no tuvieron actividad en >idle.
// Retorna IDs de sesiones cerradas. Usado por cron leader (issue-03.2).
func (s *Service) CloseInactive(ctx context.Context, idle time.Duration) ([]uuid.UUID, error) {
	now := time.Now().UTC()
	cutoff := now.Add(-idle)
	ids, err := s.repository().CloseInactive(ctx, cutoff, now)
	if err != nil {
		return nil, err
	}
	if len(ids) > 0 && s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorType:  audit.ActorSystem,
			Action:     "session.auto-closed",
			EntityType: "session",
			NewValues:  map[string]any{"count": len(ids), "idle_hours": idle.Hours()},
		})
	}
	return ids, nil
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
