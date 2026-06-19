// issue-07.2 cross-session-stitch — descubría sesiones relacionadas para que un
// agente viera el contexto histórico cross-session.
//
// REQ-42.3: la tabla sessions fue dropeada (feature legacy duplicada de
// auth_sessions). El stitching dejó de tener fuente de datos: FindRelated
// devuelve una lista vacía. El paquete se conserva (API estable) por si el
// stitching se reimplementa sobre otra fuente (p.ej. observations/timeline).
package stitcher

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RelatedSession es una session candidate con score de relevancia.
type RelatedSession struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	ProjectID uuid.UUID  `json:"project_id"`
	Title     string     `json:"title"`
	Summary   string     `json:"summary,omitempty"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	Score     float64    `json:"score"`
	MatchedOn []string   `json:"matched_on"` // ["same_user","same_project","tags:X","semantic:0.82"]
}

// Stitcher descubre relaciones cross-session.
type Stitcher struct {
	Pool *pgxpool.Pool
}

// Options para customizar el matching.
type Options struct {
	UserID     uuid.UUID
	ProjectID  *uuid.UUID // si nil → cross-project
	WindowDays int        // default 30
	MaxResults int        // default 10
	MinScore   float64    // default 0.3
}

var ErrUserRequired = errors.New("user_id required")

// FindRelated devolvía sessions relacionadas a la actual usando heurísticas.
// REQ-42.3: la tabla sessions fue dropeada — sin fuente de datos, devuelve
// una lista vacía. Mantiene la validación de entrada (user_id requerido).
func (s *Stitcher) FindRelated(ctx context.Context, currentSession uuid.UUID, opts Options) ([]RelatedSession, error) {
	_ = ctx
	_ = currentSession
	if opts.UserID == uuid.Nil {
		return nil, ErrUserRequired
	}
	return nil, nil
}
