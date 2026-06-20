package sources

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/service/wizardplan"
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

	q := strings.TrimSpace(env.RawPrompt)
	if q == "" {
		return nil
	}
	if len(q) > 300 {
		q = q[:300]
	}

	rows, err := s.Pool.Query(ctx, `
		SELECT id, slug, title, status,
		       ts_rank(to_tsvector('spanish', coalesce(title, '') || ' ' || coalesce(description, '')),
		               plainto_tsquery('spanish', $1)) AS score
		FROM issues
		WHERE to_tsvector('spanish', coalesce(title, '') || ' ' || coalesce(description, ''))
		      @@ plainto_tsquery('spanish', $1)
		ORDER BY score DESC
		LIMIT $2`,
		q, limit,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	candidates := []wizardplan.HUDedupCandidate{}
	for rows.Next() {
		var c wizardplan.HUDedupCandidate
		if err := rows.Scan(&c.HUID, &c.Slug, &c.Title, &c.Status, &c.Similarity); err != nil {
			continue
		}
		c.Reason = "FTS match vs title+description"
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	env.HUMatches = &wizardplan.HUDedupFinding{Candidates: candidates}

	// Si hay match strong, sugerir req_parent del candidate top.
	if len(candidates) > 0 && candidates[0].Similarity > 0.3 {
		// Lookup REQ del HU top.
		var reqSlug string
		err := s.Pool.QueryRow(ctx, `
			SELECT r.slug
			FROM issues us
			JOIN sdd_requirements r ON r.id = us.req_id
			WHERE us.id = $1`, candidates[0].HUID,
		).Scan(&reqSlug)
		if err == nil && reqSlug != "" {
			env.Touch(wizardplan.SlotREQParent, reqSlug, "hu_dedup",
				candidates[0].Similarity*0.9,
				"sugerido por HU similar '"+candidates[0].Title+"'")
		}
	}
	return nil
}
