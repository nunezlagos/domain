// Package cost — issue-15.1+15.2 analytics de cost por org + agent.
//
// Consulta las vistas materializadas (domain_cost_daily_by_org, _by_agent)
// para responder queries del cliente.
package cost

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/service/cost/costdb"
	"nunezlagos/domain/internal/store/txctx"
)

type DailyByOrg struct {
	Day           time.Time `json:"day"`
	Runs          int64     `json:"runs"`
	TokensInput   int64     `json:"tokens_input"`
	TokensOutput  int64     `json:"tokens_output"`
	CostUSD       float64   `json:"cost_usd"`
	AvgDurationS  float64   `json:"avg_duration_s"`
	PrevCostUSD   *float64  `json:"prev_cost_usd,omitempty"`   // LAG day before
	CostChangePCT *float64  `json:"cost_change_pct,omitempty"` // % vs prev
}

type DailyByAgent struct {
	Day          time.Time `json:"day"`
	AgentID      uuid.UUID `json:"agent_id"`
	AgentSlug    string    `json:"agent_slug"`
	Runs         int64     `json:"runs"`
	TokensInput  int64     `json:"tokens_input"`
	TokensOutput int64     `json:"tokens_output"`
	CostUSD      float64   `json:"cost_usd"`
}

type Service struct {
	Pool *pgxpool.Pool
}

// q devuelve las queries generadas atadas a la tx del contexto (si hay) o al
// pool. Seguro con o sin RLS: si el caller abrió una tx org-scopeada, las
// queries corren dentro de ella.
func (s *Service) q(ctx context.Context) *costdb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return costdb.New(tx)
	}
	return costdb.New(s.Pool)
}

// DailyByOrg devuelve cost daily aggregated por org. Rango: últimos N días.
func (s *Service) DailyByOrg(ctx context.Context, orgID uuid.UUID, days int) ([]DailyByOrg, error) {
	if days <= 0 || days > 365 {
		days = 30
	}
	rows, err := s.q(ctx).DailyByOrg(ctx, int32(days))
	if err != nil {
		return nil, fmt.Errorf("query daily by org: %w", err)
	}
	out := make([]DailyByOrg, 0, len(rows))
	for _, r := range rows {
		out = append(out, toDailyByOrg(r))
	}
	return out, nil
}

// DailyByAgent devuelve cost daily aggregated por agent dentro de la org.
func (s *Service) DailyByAgent(ctx context.Context, orgID uuid.UUID, days int) ([]DailyByAgent, error) {
	if days <= 0 || days > 365 {
		days = 30
	}
	rows, err := s.q(ctx).DailyByAgent(ctx, int32(days))
	if err != nil {
		return nil, fmt.Errorf("query daily by agent: %w", err)
	}
	out := make([]DailyByAgent, 0, len(rows))
	for _, r := range rows {
		out = append(out, toDailyByAgent(r))
	}
	return out, nil
}

func toDailyByOrg(r costdb.DailyByOrgRow) DailyByOrg {
	d := DailyByOrg{
		Day:          r.Day,
		Runs:         r.Runs,
		TokensInput:  r.TokensInput,
		TokensOutput: r.TokensOutput,
		CostUSD:      numericToFloat(r.CostUsd),
		AvgDurationS: numericToFloat(r.AvgDurationS),
		PrevCostUSD:  numericToFloatPtr(r.PrevCostUsd),
	}
	if d.PrevCostUSD != nil && *d.PrevCostUSD > 0 {
		pct := ((d.CostUSD - *d.PrevCostUSD) / *d.PrevCostUSD) * 100
		d.CostChangePCT = &pct
	}
	return d
}

func toDailyByAgent(r costdb.DailyByAgentRow) DailyByAgent {
	return DailyByAgent{
		Day:          r.Day,
		AgentID:      r.AgentID,
		AgentSlug:    r.AgentSlug,
		Runs:         r.Runs,
		TokensInput:  r.TokensInput,
		TokensOutput: r.TokensOutput,
		CostUSD:      numericToFloat(r.CostUsd),
	}
}

// numericToFloat convierte un pgtype.Numeric a float64; 0 si NULL/inválido.
func numericToFloat(n pgtype.Numeric) float64 {
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}

// numericToFloatPtr convierte un pgtype.Numeric nullable a *float64; nil si
// NULL/inválido. prev_cost_usd es NULL en la primera fila (LAG sin previo).
func numericToFloatPtr(n pgtype.Numeric) *float64 {
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return nil
	}
	v := f.Float64
	return &v
}
