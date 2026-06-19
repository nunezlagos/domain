// Package search — issue-03.7 búsqueda global cross-entity.
//
// Une observations + prompts + sessions en una sola query rankeada. Scoped
// por organization_id del principal (RBAC enforcement por query, sin
// post-filtering).
//
// Scoring: cada entity calcula su ts_rank propio. Se normaliza al mismo dominio
// y se ordena DESC. NO usamos RRF aquí porque las entities son distintas
// (no estamos fusionando dos rankings sobre la misma entity).
package search

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/store/txctx"
)

// EntityType discrimina el tipo del resultado.
type EntityType string

const (
	EntityObservation EntityType = "observation"
	EntityPrompt      EntityType = "prompt"
	// EntitySession se conserva por compatibilidad del enum, pero ya no se
	// busca (REQ-42.3: tabla sessions dropeada).
	EntitySession      EntityType = "session"
	EntityKnowledgeDoc EntityType = "knowledge_doc"
)

// Result entrada del feed de resultados unificada.
type Result struct {
	EntityType   EntityType `json:"entity_type"`
	ID           uuid.UUID  `json:"id"`
	Title        string     `json:"title,omitempty"`
	Snippet      string     `json:"snippet,omitempty"`
	Score        float64    `json:"score"`
	ProjectID    *uuid.UUID `json:"project_id,omitempty"`
	Tags         []string   `json:"tags,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	MatchedTerms []string   `json:"matched_terms,omitempty"`
}

// Filter opcional para acotar resultados.
type Filter struct {
	EntityTypes []EntityType // si vacío incluye todos
	ProjectIDs  []uuid.UUID  // si vacío sin filtro
	Tags        []string     // tags requeridos (AND)
	DateFrom    *time.Time
	DateTo      *time.Time
}

type Service struct {
	Pool *pgxpool.Pool
}

// q retorna la tx con SET LOCAL si el middleware HTTP la inyecto
// (issue-25.14), o el pool como fallback.
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

var (
	ErrEmptyQuery = errors.New("query required")
)

// Search ejecuta búsqueda global filtrada al org del caller.
// limit total entre 1..200 (cap). Cada entity contribuye hasta limit
// resultados, luego se mergea + trunca.
func (s *Service) Search(ctx context.Context, orgID uuid.UUID, query string, limit int, f Filter) ([]Result, error) {
	if strings.TrimSpace(query) == "" {
		return nil, ErrEmptyQuery
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	wantObs, wantPrompt, wantKnowledge := entitySelection(f.EntityTypes)

	var all []Result
	if wantObs {
		r, err := s.searchObservations(ctx, orgID, query, limit, f)
		if err != nil {
			return nil, fmt.Errorf("observations: %w", err)
		}
		all = append(all, r...)
	}
	if wantPrompt {
		r, err := s.searchPrompts(ctx, orgID, query, limit, f)
		if err != nil {
			return nil, fmt.Errorf("prompts: %w", err)
		}
		all = append(all, r...)
	}
	// REQ-42.3: sessions dropeada — sin búsqueda sobre sesiones.
	if wantKnowledge {
		r, err := s.searchKnowledgeDocs(ctx, orgID, query, limit, f)
		if err != nil {
			return nil, fmt.Errorf("knowledge_docs: %w", err)
		}
		all = append(all, r...)
	}

	// Sort por score DESC, truncar a limit
	mergeSortByScore(all)
	if len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

func entitySelection(types []EntityType) (obs, prompt, knowledge bool) {
	if len(types) == 0 {
		return true, true, true
	}
	for _, t := range types {
		switch t {
		case EntityObservation:
			obs = true
		case EntityPrompt:
			prompt = true
		case EntityKnowledgeDoc:
			knowledge = true
		}
	}
	return
}

func (s *Service) searchObservations(ctx context.Context, orgID uuid.UUID, query string, limit int, f Filter) ([]Result, error) {
	q := `
SELECT o.id, o.observation_type, o.content, o.tags, o.project_id, o.created_at,
       ts_rank(o.content_tsv, qry)::float8 AS score
FROM observations o, plainto_tsquery('spanish', $1) AS qry
WHERE o.deleted_at IS NULL AND o.content_tsv @@ qry
`
	args := []any{query}
	if len(f.ProjectIDs) > 0 {
		q += fmt.Sprintf(" AND o.project_id = ANY($%d)", len(args)+1)
		args = append(args, f.ProjectIDs)
	}
	if len(f.Tags) > 0 {
		q += fmt.Sprintf(" AND o.tags @> $%d", len(args)+1)
		args = append(args, f.Tags)
	}
	if f.DateFrom != nil {
		q += fmt.Sprintf(" AND o.created_at >= $%d", len(args)+1)
		args = append(args, *f.DateFrom)
	}
	if f.DateTo != nil {
		q += fmt.Sprintf(" AND o.created_at < $%d", len(args)+1)
		args = append(args, *f.DateTo)
	}
	q += fmt.Sprintf(" ORDER BY score DESC LIMIT $%d", len(args)+1)
	args = append(args, limit)

	rows, err := s.q(ctx).Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Result
	for rows.Next() {
		var r Result
		r.EntityType = EntityObservation
		var content string
		var projectID uuid.UUID
		if err := rows.Scan(&r.ID, &r.Title, &content, &r.Tags, &projectID, &r.CreatedAt, &r.Score); err != nil {
			return nil, err
		}
		r.Snippet = truncate(content, 200)
		r.ProjectID = &projectID
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Service) searchPrompts(ctx context.Context, orgID uuid.UUID, query string, limit int, f Filter) ([]Result, error) {
	q := `
SELECT p.id, p.slug, p.body, p.tags, p.project_id, p.created_at,
       ts_rank(p.body_tsv, qry)::float8 AS score
FROM prompts p, plainto_tsquery('spanish', $1) AS qry
WHERE p.deleted_at IS NULL AND p.body_tsv @@ qry
`
	args := []any{query}
	if len(f.ProjectIDs) > 0 {
		q += fmt.Sprintf(" AND p.project_id = ANY($%d)", len(args)+1)
		args = append(args, f.ProjectIDs)
	}
	if f.DateFrom != nil {
		q += fmt.Sprintf(" AND p.created_at >= $%d", len(args)+1)
		args = append(args, *f.DateFrom)
	}
	if f.DateTo != nil {
		q += fmt.Sprintf(" AND p.created_at < $%d", len(args)+1)
		args = append(args, *f.DateTo)
	}
	q += fmt.Sprintf(" ORDER BY score DESC LIMIT $%d", len(args)+1)
	args = append(args, limit)

	rows, err := s.q(ctx).Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Result
	for rows.Next() {
		var r Result
		r.EntityType = EntityPrompt
		var body string
		var projectID *uuid.UUID
		if err := rows.Scan(&r.ID, &r.Title, &body, &r.Tags, &projectID, &r.CreatedAt, &r.Score); err != nil {
			return nil, err
		}
		r.Snippet = truncate(body, 200)
		r.ProjectID = projectID
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Service) searchKnowledgeDocs(ctx context.Context, orgID uuid.UUID, query string, limit int, f Filter) ([]Result, error) {
	q := `
SELECT kd.id, kd.title, kd.body, kd.project_id, kd.created_at,
       ts_rank(kd.body_tsv, qry)::float8 AS score
FROM knowledge_docs kd, plainto_tsquery('spanish', $1) AS qry
WHERE kd.deleted_at IS NULL AND kd.body_tsv @@ qry
`
	args := []any{query}
	if len(f.ProjectIDs) > 0 {
		q += fmt.Sprintf(" AND kd.project_id = ANY($%d)", len(args)+1)
		args = append(args, f.ProjectIDs)
	}
	if f.DateFrom != nil {
		q += fmt.Sprintf(" AND kd.created_at >= $%d", len(args)+1)
		args = append(args, *f.DateFrom)
	}
	if f.DateTo != nil {
		q += fmt.Sprintf(" AND kd.created_at < $%d", len(args)+1)
		args = append(args, *f.DateTo)
	}
	q += fmt.Sprintf(" ORDER BY score DESC LIMIT $%d", len(args)+1)
	args = append(args, limit)

	rows, err := s.q(ctx).Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Result
	for rows.Next() {
		var r Result
		r.EntityType = EntityKnowledgeDoc
		var body string
		var projectID *uuid.UUID
		if err := rows.Scan(&r.ID, &r.Title, &body, &projectID, &r.CreatedAt, &r.Score); err != nil {
			return nil, err
		}
		r.Snippet = truncate(body, 200)
		r.ProjectID = projectID
		out = append(out, r)
	}
	return out, rows.Err()
}

func mergeSortByScore(rs []Result) {
	// insertion sort: tamaño esperado <= 3*limit (200) — simple y estable
	for i := 1; i < len(rs); i++ {
		for j := i; j > 0 && rs[j].Score > rs[j-1].Score; j-- {
			rs[j], rs[j-1] = rs[j-1], rs[j]
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
