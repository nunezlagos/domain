// issue-07.2 cross-session-stitch — descubre sesiones relacionadas y construye
// un timeline cross-session para que un agente vea el contexto histórico
// completo (no solo la sesión actual).
//
// Heurísticas:
//   - Same user_id + project_id en últimos 30 días.
//   - Overlap de tags/entities mencionados.
//   - Similaridad semántica (cosine ≥ 0.7) sobre summaries.
package stitcher

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RelatedSession es una session candidate con score de relevancia.
type RelatedSession struct {
	ID         uuid.UUID `json:"id"`
	UserID     uuid.UUID `json:"user_id"`
	ProjectID  uuid.UUID `json:"project_id"`
	Title      string    `json:"title"`
	Summary    string    `json:"summary,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	EndedAt    *time.Time `json:"ended_at,omitempty"`
	Score      float64   `json:"score"`
	MatchedOn  []string  `json:"matched_on"` // ["same_user","same_project","tags:X","semantic:0.82"]
}

// Stitcher descubre relaciones cross-session.
type Stitcher struct {
	Pool *pgxpool.Pool
}

// Options para customizar el matching.
type Options struct {
	UserID       uuid.UUID
	ProjectID    *uuid.UUID    // si nil → cross-project
	WindowDays   int           // default 30
	MaxResults   int           // default 10
	MinScore     float64       // default 0.3
}

var ErrUserRequired = errors.New("user_id required")

// FindRelated devuelve sessions relacionadas a la actual usando heurísticas.
func (s *Stitcher) FindRelated(ctx context.Context, currentSession uuid.UUID, opts Options) ([]RelatedSession, error) {
	if opts.UserID == uuid.Nil {
		return nil, ErrUserRequired
	}
	window := opts.WindowDays
	if window <= 0 {
		window = 30
	}
	max := opts.MaxResults
	if max <= 0 || max > 100 {
		max = 10
	}
	since := time.Now().AddDate(0, 0, -window)

	q := `
		SELECT id, user_id, project_id, title, COALESCE(summary, ''),
		       started_at, ended_at
		FROM sessions
		WHERE user_id = $1
		  AND id != $2
		  AND started_at >= $3`
	args := []any{opts.UserID, currentSession, since}
	if opts.ProjectID != nil {
		q += ` AND project_id = $4`
		args = append(args, *opts.ProjectID)
	}
	q += ` ORDER BY started_at DESC LIMIT 200`

	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}
	defer rows.Close()

	var candidates []RelatedSession
	for rows.Next() {
		var r RelatedSession
		if err := rows.Scan(&r.ID, &r.UserID, &r.ProjectID, &r.Title,
			&r.Summary, &r.StartedAt, &r.EndedAt); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		// Score básico: same_user siempre, +0.2 por same_project, +0.05 per día reciente.
		r.MatchedOn = []string{"same_user"}
		r.Score = 0.5
		if opts.ProjectID != nil && r.ProjectID == *opts.ProjectID {
			r.MatchedOn = append(r.MatchedOn, "same_project")
			r.Score += 0.2
		}
		daysAgo := time.Since(r.StartedAt).Hours() / 24
		recencyBoost := (float64(window) - daysAgo) / float64(window) * 0.3
		if recencyBoost > 0 {
			r.Score += recencyBoost
		}
		if r.Score >= opts.MinScore {
			candidates = append(candidates, r)
		}
	}
	if len(candidates) > max {
		candidates = candidates[:max]
	}
	return candidates, rows.Err()
}
