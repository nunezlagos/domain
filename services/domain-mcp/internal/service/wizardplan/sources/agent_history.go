package sources

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/service/wizardplan"
	"nunezlagos/domain/internal/service/wizardplan/wizardplandb"
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

	q := wizardplandb.New(s.Pool)

	interval := pgtype.Interval{
		Days:  int32(days),
		Valid: true,
	}

	rows, err := q.ListAgentRunsSince(ctx, wizardplandb.ListAgentRunsSinceParams{
		IntervalDays: interval,
		UserID:       s.UserID,
		ResultLimit:  int32(limit * 3), // sobre-fetch para filtrar después
	})
	if err != nil {
		return err
	}

	related := []wizardplan.RelatedRun{}
	for _, row := range rows {
		summary := row.Summary
		summaryLower := lowerOnAscii(summary)
		if !matchesAnyKeyword(summaryLower, keywords) {
			continue
		}

		var startedAt time.Time
		if row.StartedAt.Valid {
			startedAt = row.StartedAt.Time
		}

		related = append(related, wizardplan.RelatedRun{
			AgentRunID: row.ID,
			AgentSlug:  row.AgentSlug,
			StartedAt:  startedAt,
			Summary:    summary,
		})
		if len(related) >= limit {
			break
		}
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
