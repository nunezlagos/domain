// Package timeline — issue-03.5 cross-entity feed.
//
// Combina observations + prompts en respuestas estructuradas para que un
// agente IA pueda recuperar contexto rápido sin múltiples roundtrips.
// REQ-42.3: la tabla sessions fue dropeada — el feed ya no incluye sesiones
// (los campos ActiveSession/RecentSessions del snapshot quedan vacíos por
// compatibilidad de API).
package timeline

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/store/txctx"
)

var ErrObservationNotFound = errors.New("anchor observation not found")

// EntryKind tipo de entrada del feed.
type EntryKind string

const (
	// KindSession se conserva por compatibilidad histórica del enum, pero ya
	// no se emite (REQ-42.3: tabla sessions dropeada).
	KindSession     EntryKind = "session"
	KindObservation EntryKind = "observation"
	KindPrompt      EntryKind = "prompt"
)

// Entry registro abstracto en la timeline.
type Entry struct {
	Kind      EntryKind  `json:"kind"`
	ID        uuid.UUID  `json:"id"`
	Title     string     `json:"title,omitempty"`   // session.title, prompt.slug, observation type
	Preview   string     `json:"preview,omitempty"` // content/body truncado
	CreatedAt time.Time  `json:"created_at"`
	ProjectID *uuid.UUID `json:"project_id,omitempty"`
	UserID    *uuid.UUID `json:"user_id,omitempty"`
}

// Snapshot agrupa secciones del contexto del project (devuelto por Context).
type Snapshot struct {
	ProjectID          *uuid.UUID `json:"project_id"`
	ActiveSession      *Entry     `json:"active_session,omitempty"`
	RecentSessions     []Entry    `json:"recent_sessions"`
	RecentObservations []Entry    `json:"recent_observations"`
	RecentPrompts      []Entry    `json:"recent_prompts"`
}

type Service struct {
	Pool *pgxpool.Pool
}

// q retorna la tx con SET LOCAL si el middleware HTTP la inyecto
// (issue-25.14), o el pool como fallback. issue-25.14 wireup.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (s *Service) q(ctx context.Context) querier {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return tx
	}
	return s.Pool
}

// Context arma un snapshot del project. Si projectID == uuid.Nil, scope =
// org-wide (cross-project). userID requerido para filtrar active_session.
//
// Límites: 5 sesiones, 10 observations, 5 prompts. Si limit override > 0 ajusta.
func (s *Service) Context(ctx context.Context, orgID, userID, projectID uuid.UUID) (*Snapshot, error) {
	snap := &Snapshot{}
	if projectID != uuid.Nil {
		snap.ProjectID = &projectID
	}

	// REQ-42.3: sessions dropeada — sin active/recent sessions.
	_ = userID
	snap.ActiveSession = nil
	snap.RecentSessions = nil

	// Recent observations
	obs, err := s.queryObservations(ctx, orgID, projectID, 10)
	if err != nil {
		return nil, fmt.Errorf("recent observations: %w", err)
	}
	snap.RecentObservations = obs

	// Recent active prompts
	prompts, err := s.queryPrompts(ctx, orgID, projectID, 5)
	if err != nil {
		return nil, fmt.Errorf("recent prompts: %w", err)
	}
	snap.RecentPrompts = prompts

	return snap, nil
}

// Timeline devuelve N entradas antes y después de la observación ancla,
// ordenadas cronológicamente. Combina observations + prompts del mismo project.
// before+after = ventana total; anchor entry incluida.
func (s *Service) Timeline(ctx context.Context, orgID, observationID uuid.UUID, before, after int) ([]Entry, error) {
	if before < 0 {
		before = 3
	}
	if after < 0 {
		after = 3
	}
	if before > 50 {
		before = 50
	}
	if after > 50 {
		after = 50
	}

	// Lookup anchor (project + created_at)
	var (
		anchorCreatedAt time.Time
		anchorProjectID uuid.UUID
		anchorContent   string
	)
	err := s.q(ctx).QueryRow(ctx,
		`SELECT created_at, project_id, content
		 FROM observations WHERE id = $1 AND deleted_at IS NULL`, observationID,
	).Scan(&anchorCreatedAt, &anchorProjectID, &anchorContent)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrObservationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("anchor lookup: %w", err)
	}

	// Before: observaciones + prompts ANTERIORES
	priorObs, err := s.queryEntriesAround(ctx, anchorProjectID, anchorCreatedAt, before, true)
	if err != nil {
		return nil, err
	}
	// After: posteriores (excluyendo el anchor)
	nextObs, err := s.queryEntriesAround(ctx, anchorProjectID, anchorCreatedAt, after, false)
	if err != nil {
		return nil, err
	}

	anchor := Entry{
		Kind:      KindObservation,
		ID:        observationID,
		Title:     "observation",
		Preview:   truncate(anchorContent, 200),
		CreatedAt: anchorCreatedAt,
		ProjectID: &anchorProjectID,
	}

	all := append(priorObs, anchor)
	all = append(all, nextObs...)
	sort.SliceStable(all, func(i, j int) bool {
		return all[i].CreatedAt.Before(all[j].CreatedAt)
	})
	return all, nil
}

// --- helpers ---

func (s *Service) queryObservations(ctx context.Context, orgID, projectID uuid.UUID, limit int) ([]Entry, error) {
	var q string
	var args []any
	if projectID == uuid.Nil {
		q = `SELECT id, observation_type, content, created_at
		     FROM observations
		     WHERE deleted_at IS NULL
		     ORDER BY created_at DESC LIMIT $1`
		args = []any{limit}
	} else {
		q = `SELECT id, observation_type, content, created_at
		     FROM observations
		     WHERE project_id = $1 AND deleted_at IS NULL
		     ORDER BY created_at DESC LIMIT $2`
		args = []any{projectID, limit}
	}
	rows, err := s.q(ctx).Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		var content string
		if err := rows.Scan(&e.ID, &e.Title, &content, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Kind = KindObservation
		e.Preview = truncate(content, 200)
		if projectID != uuid.Nil {
			pid := projectID
			e.ProjectID = &pid
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Service) queryPrompts(ctx context.Context, orgID, projectID uuid.UUID, limit int) ([]Entry, error) {
	var q string
	var args []any
	if projectID == uuid.Nil {
		q = `SELECT id, slug, body, created_at FROM prompts
		     WHERE is_active = true AND deleted_at IS NULL
		     ORDER BY created_at DESC LIMIT $1`
		args = []any{limit}
	} else {
		q = `SELECT id, slug, body, created_at FROM prompts
		     WHERE project_id = $1
		       AND is_active = true AND deleted_at IS NULL
		     ORDER BY created_at DESC LIMIT $2`
		args = []any{projectID, limit}
	}
	rows, err := s.q(ctx).Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		var body string
		if err := rows.Scan(&e.ID, &e.Title, &body, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Kind = KindPrompt
		e.Preview = truncate(body, 200)
		if projectID != uuid.Nil {
			pid := projectID
			e.ProjectID = &pid
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// queryEntriesAround retorna observations + prompts antes o después de ts,
// limitadas a `limit` entries. before=true → created_at < ts; else > ts.
func (s *Service) queryEntriesAround(ctx context.Context, projectID uuid.UUID, ts time.Time, limit int, before bool) ([]Entry, error) {
	if limit == 0 {
		return nil, nil
	}
	cmp := ">"
	order := "ASC"
	if before {
		cmp = "<"
		order = "DESC"
	}
	q := fmt.Sprintf(`
		SELECT 'observation' AS kind, id, observation_type, content, created_at
		FROM observations
		WHERE project_id = $1 AND created_at %s $2 AND deleted_at IS NULL
		UNION ALL
		SELECT 'prompt' AS kind, id, slug, body, created_at
		FROM prompts
		WHERE project_id = $1 AND created_at %s $2 AND is_active = true AND deleted_at IS NULL
		ORDER BY 5 %s LIMIT $3
	`, cmp, cmp, order)
	rows, err := s.q(ctx).Query(ctx, q, projectID, ts, limit)
	if err != nil {
		return nil, fmt.Errorf("around: %w", err)
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var (
			kind    string
			id      uuid.UUID
			title   string
			preview string
			created time.Time
		)
		if err := rows.Scan(&kind, &id, &title, &preview, &created); err != nil {
			return nil, err
		}
		pid := projectID
		out = append(out, Entry{
			Kind:      EntryKind(kind),
			ID:        id,
			Title:     title,
			Preview:   truncate(preview, 200),
			CreatedAt: created,
			ProjectID: &pid,
		})
	}
	return out, rows.Err()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
