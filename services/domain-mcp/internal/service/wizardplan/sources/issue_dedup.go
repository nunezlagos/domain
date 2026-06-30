package sources

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/service/wizardplan"
	"nunezlagos/domain/internal/service/wizardplan/wizardplandb"
)

// IssueDedupSource compara el prompt vs issues existentes via FTS sobre
// title/description. (Implementación naive — versión vector pendiente para
// cuando el embedder sea wired al wizard.)
type IssueDedupSource struct {
	Pool  *pgxpool.Pool
	Limit int // default 5
}

func (s *IssueDedupSource) Name() string { return "hu_dedup" }

func (s *IssueDedupSource) Run(ctx context.Context, env *wizardplan.ContextEnvelope) error {
	if s.Pool == nil {
		return nil
	}
	limit := s.Limit
	if limit <= 0 {
		limit = 5
	}

	query := strings.TrimSpace(env.RawPrompt)
	if query == "" {
		return nil
	}
	if len(query) > 300 {
		query = query[:300]
	}

	q := wizardplandb.New(s.Pool)

	issueRows, err := q.ListIssuesByFTS(ctx, wizardplandb.ListIssuesByFTSParams{
		Query:       query,
		ResultLimit: int32(limit),
	})
	if err != nil {
		return err
	}

	candidates := []wizardplan.HUDedupCandidate{}
	for _, row := range issueRows {
		candidates = append(candidates, wizardplan.HUDedupCandidate{
			HUID:       row.ID,
			Slug:       row.Slug,
			Title:      row.Title,
			Status:     row.Status,
			Similarity: float64(row.Score),
			Reason:     "FTS match vs title+description",
		})
	}

	env.HUMatches = &wizardplan.HUDedupFinding{Candidates: candidates}

	if len(candidates) > 0 && candidates[0].Similarity > 0.3 {
		reqSlug, err := q.GetRequirementSlugByIssueID(ctx, candidates[0].HUID)
		if err == nil && reqSlug != "" {
			env.Touch(wizardplan.SlotREQParent, reqSlug, "hu_dedup",
				candidates[0].Similarity*0.9,
				"sugerido por HU similar '"+candidates[0].Title+"'")
		}
	}
	return nil
}
