// Package sources — fuentes del analyzer pipeline.
package sources

import (
	"context"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/service/search"
	"nunezlagos/domain/internal/service/wizardplan"
)

// MemorySource ejecuta search.Service.Search sobre el prompt para encontrar
// observations + knowledge_docs relacionados.
type MemorySource struct {
	Search  *search.Service
	OrgID   uuid.UUID
	Limit   int // default 5
}

// Name implements wizardplan.Source.
func (s *MemorySource) Name() string { return "memory" }

// Run ejecuta la búsqueda y popula env.Memory.
func (s *MemorySource) Run(ctx context.Context, env *wizardplan.ContextEnvelope) error {
	if s.Search == nil {
		return nil
	}
	limit := s.Limit
	if limit <= 0 {
		limit = 5
	}
	results, err := s.Search.Search(ctx, s.OrgID, env.RawPrompt, limit, search.Filter{})
	if err != nil {
		return err
	}

	matches := make([]wizardplan.MemoryMatch, 0, len(results))
	for _, r := range results {
		m := wizardplan.MemoryMatch{
			EntityType: string(r.EntityType),
			ID:         r.ID,
			Title:      r.Title,
			Snippet:    r.Snippet,
			Score:      r.Score,
		}
		matches = append(matches, m)
	}
	env.Memory = &wizardplan.MemoryFinding{Matches: matches}
	return nil
}
