package orchestrator

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// SkillsRecommended wraps recommended skills for a phase.
type SkillsRecommended struct {
	Skills    []SkillRecommendation `json:"skills"`
	Threshold float64               `json:"threshold"`
}

// SkillRecommendation is a single skill recommended for the next phase.
type SkillRecommendation struct {
	Slug  string  `json:"slug"`
	Name  string  `json:"name"`
	Score float64 `json:"score"`
}

// fetchRecommendedSkills busca skills relevantes para la próxima fase
// usando SearchHybrid (issue-05.4 auto-engine). Si el threshold ≤ 0 o
// s.Skills es nil, devuelve nil sin error (skill-001 deshabilitado).
//
// El query se construye desde el slug del template de la fase para que
// SearchHybrid encuentre skills con descripción similar. Opcionalmente
// se podría enriquecer con el output de la fase actual, pero como D3 es
// informativo (no bloqueante), un query simple alcanza.
func (s *Service) fetchRecommendedSkills(ctx context.Context, orgID, projectID uuid.UUID, agentTemplateSlug string, threshold float64) (*SkillsRecommended, error) {
	if s.Skills == nil || threshold <= 0 {
		return nil, nil
	}
	query := fmt.Sprintf("skills for %s phase", agentTemplateSlug)
	results, err := s.Skills.SearchHybrid(ctx, orgID, query, 5)
	if err != nil {
		return nil, fmt.Errorf("search skills: %w", err)
	}
	// Scoping por proyecto: solo se recomiendan skills ENLAZADAS al proyecto
	// (project_skills). Si projectID es Nil, no se filtra (compat sin scope).
	linked, err := s.Skills.LinkedSkillIDs(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("linked skills: %w", err)
	}
	recs := &SkillsRecommended{Threshold: threshold}
	for _, r := range results {
		if r.Score < threshold {
			continue
		}
		if linked != nil && !linked[r.ID] {
			continue // skill no enlazada a este proyecto → no usable
		}
		recs.Skills = append(recs.Skills, SkillRecommendation{
			Slug:  r.Slug,
			Name:  r.Name,
			Score: r.Score,
		})
	}
	return recs, nil
}
