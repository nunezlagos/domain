// Package search — issue-03.7 búsqueda global cross-entity.
//
// Une observations + prompts + sessions en una sola query rankeada. Scoped
// por organization_id del principal (RBAC enforcement por query, sin
// post-filtering).
//
// Scoring: cada entity calcula su ts_rank propio. Se normaliza al mismo dominio
// y se ordena DESC. NO usamos RRF aquí porque las entities son distintas
// (no estamos fusionando dos rankings sobre la misma entity).
//
//go:generate go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
package search

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/service/search/searchdb"
	"nunezlagos/domain/internal/store/txctx"
)

// EntityType discrimina el tipo del resultado.
type EntityType string

const (
	EntityObservation EntityType = "observation"
	EntityPrompt      EntityType = "prompt"

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
func (s *Service) q(ctx context.Context) *searchdb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return searchdb.New(tx)
	}
	return searchdb.New(s.Pool)
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

	if wantKnowledge {
		r, err := s.searchKnowledgeDocs(ctx, orgID, query, limit, f)
		if err != nil {
			return nil, fmt.Errorf("knowledge_docs: %w", err)
		}
		all = append(all, r...)
	}

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
	rows, err := s.q(ctx).SearchObservations(ctx, searchdb.SearchObservationsParams{
		Query:       query,
		ProjectIds:  optProjectIDs(f.ProjectIDs),
		Tags:        optTags(f.Tags),
		DateFrom:    optTime(f.DateFrom),
		DateTo:      optTime(f.DateTo),
		ResultLimit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]Result, 0, len(rows))
	for _, row := range rows {
		projectID := row.ProjectID
		out = append(out, Result{
			EntityType: EntityObservation,
			ID:         row.ID,
			Title:      row.ObservationType,
			Snippet:    truncate(row.Content, 200),
			Score:      row.Score,
			ProjectID:  &projectID,
			Tags:       row.Tags,
			CreatedAt:  row.CreatedAt,
		})
	}
	return out, nil
}

func (s *Service) searchPrompts(ctx context.Context, orgID uuid.UUID, query string, limit int, f Filter) ([]Result, error) {
	rows, err := s.q(ctx).SearchPrompts(ctx, searchdb.SearchPromptsParams{
		Query:       query,
		ProjectIds:  optProjectIDs(f.ProjectIDs),
		DateFrom:    optTime(f.DateFrom),
		DateTo:      optTime(f.DateTo),
		ResultLimit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]Result, 0, len(rows))
	for _, row := range rows {
		out = append(out, Result{
			EntityType: EntityPrompt,
			ID:         row.ID,
			Title:      row.Slug,
			Snippet:    truncate(row.Body, 200),
			Score:      row.Score,
			ProjectID:  row.ProjectID,
			Tags:       row.Tags,
			CreatedAt:  row.CreatedAt,
		})
	}
	return out, nil
}

func (s *Service) searchKnowledgeDocs(ctx context.Context, orgID uuid.UUID, query string, limit int, f Filter) ([]Result, error) {
	rows, err := s.q(ctx).SearchKnowledgeDocs(ctx, searchdb.SearchKnowledgeDocsParams{
		Query:       query,
		ProjectIds:  optProjectIDs(f.ProjectIDs),
		DateFrom:    optTime(f.DateFrom),
		DateTo:      optTime(f.DateTo),
		ResultLimit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]Result, 0, len(rows))
	for _, row := range rows {
		projectID := row.ProjectID
		out = append(out, Result{
			EntityType: EntityKnowledgeDoc,
			ID:         row.ID,
			Title:      row.Title,
			Snippet:    truncate(row.Body, 200),
			Score:      row.Score,
			ProjectID:  &projectID,
			CreatedAt:  row.CreatedAt,
		})
	}
	return out, nil
}

// optProjectIDs devuelve nil (sin filtro) cuando el slice viene vacío, lo que
// satisface el guard `$N::uuid[] IS NULL` de la query.
func optProjectIDs(ids []uuid.UUID) []uuid.UUID {
	if len(ids) == 0 {
		return nil
	}
	return ids
}

// optTags idem optProjectIDs para el filtro de tags (solo observations).
func optTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	return tags
}

// optTime mapea un *time.Time opcional al pgtype.Timestamptz que espera el
// param generado: nil => NULL (sin filtro).
func optTime(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

func mergeSortByScore(rs []Result) {

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
