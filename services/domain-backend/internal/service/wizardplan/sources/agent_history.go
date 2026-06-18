package sources

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/service/wizardplan"
)

// AgentHistorySource busca agent_runs recientes del usuario con outputs
// que mencionen palabras clave del prompt.
type AgentHistorySource struct {
	Pool   *pgxpool.Pool
	UserID *uuid.UUID
	OrgID  uuid.UUID
	Days   int // default 14
	Limit  int // default 5
}

func (s *AgentHistorySource) Name() string { return "history" }

func (s *AgentHistorySource) Run(ctx context.Context, env *wizardplan.ContextEnvelope) error {
	if s.Pool == nil {
		return nil
	}
	days := s.Days
	if days <= 0 {
		days = 14
	}
	limit := s.Limit
	if limit <= 0 {
		limit = 5
	}

	keywords := extractKeywords(env.RawPrompt, 5)
	if len(keywords) == 0 {
		return nil
	}

	// Query agent_runs recientes; usa ILIKE sobre nombre de agent + outputs
	// JSONB. Sin embedding por ahora — heurística simple.
	// ISSUE-21.6 Fase D clean: single-org, WHERE sin organization_id.
	_ = s // unused-org scoping
	q := `
		SELECT ar.id, COALESCE(a.slug, ''), ar.started_at,
		       LEFT(COALESCE(ar.outputs::text, ''), 200) AS summary
		FROM agent_runs ar
		LEFT JOIN agents a ON a.id = ar.agent_id
		WHERE ar.started_at >= now() - $1::INTERVAL`
	args := []any{days}
	if s.UserID != nil {
		q += ` AND ar.triggered_by = $2`
		args = append(args, *s.UserID)
	}
	q += ` ORDER BY ar.started_at DESC LIMIT $`
	if s.UserID != nil {
		q += "3"
	} else {
		q += "2"
	}
	args = append(args, limit*3) // sobre-fetch para filtrar después

	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	related := []wizardplan.RelatedRun{}
	for rows.Next() {
		var r wizardplan.RelatedRun
		var summary string
		if err := rows.Scan(&r.AgentRunID, &r.AgentSlug, &r.StartedAt, &summary); err != nil {
			continue
		}
		// Filtrar por keyword match.
		summaryLower := lowerOnAscii(summary)
		if !matchesAnyKeyword(summaryLower, keywords) {
			continue
		}
		r.Summary = summary
		related = append(related, r)
		if len(related) >= limit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	env.History = &wizardplan.AgentHistoryFinding{RelatedRuns: related}
	return nil
}

func lowerOnAscii(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b[i] = c
	}
	return string(b)
}
