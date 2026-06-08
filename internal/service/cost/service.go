// Package cost — HU-15.1+15.2 analytics de cost por org + agent.
//
// Consulta las vistas materializadas (domain_cost_daily_by_org, _by_agent)
// para responder queries del cliente.
package cost

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DailyByOrg struct {
	Day           time.Time `json:"day"`
	Runs          int64     `json:"runs"`
	TokensInput   int64     `json:"tokens_input"`
	TokensOutput  int64     `json:"tokens_output"`
	CostUSD       float64   `json:"cost_usd"`
	AvgDurationS  float64   `json:"avg_duration_s"`
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

// DailyByOrg devuelve cost daily aggregated por org. Rango: últimos N días.
func (s *Service) DailyByOrg(ctx context.Context, orgID uuid.UUID, days int) ([]DailyByOrg, error) {
	if days <= 0 || days > 365 {
		days = 30
	}
	rows, err := s.Pool.Query(ctx,
		`SELECT day, runs, tokens_input, tokens_output, cost_usd, avg_duration_s
		 FROM domain_cost_daily_by_org
		 WHERE organization_id = $1
		   AND day >= CURRENT_DATE - $2::int
		 ORDER BY day DESC`,
		orgID, days)
	if err != nil {
		return nil, fmt.Errorf("query daily by org: %w", err)
	}
	defer rows.Close()
	var out []DailyByOrg
	for rows.Next() {
		var d DailyByOrg
		if err := rows.Scan(&d.Day, &d.Runs, &d.TokensInput, &d.TokensOutput,
			&d.CostUSD, &d.AvgDurationS); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// DailyByAgent devuelve cost daily aggregated por agent dentro de la org.
func (s *Service) DailyByAgent(ctx context.Context, orgID uuid.UUID, days int) ([]DailyByAgent, error) {
	if days <= 0 || days > 365 {
		days = 30
	}
	rows, err := s.Pool.Query(ctx,
		`SELECT day, agent_id, agent_slug, runs, tokens_input, tokens_output, cost_usd
		 FROM domain_cost_daily_by_agent
		 WHERE organization_id = $1
		   AND day >= CURRENT_DATE - $2::int
		 ORDER BY day DESC, cost_usd DESC`,
		orgID, days)
	if err != nil {
		return nil, fmt.Errorf("query daily by agent: %w", err)
	}
	defer rows.Close()
	var out []DailyByAgent
	for rows.Next() {
		var d DailyByAgent
		if err := rows.Scan(&d.Day, &d.AgentID, &d.AgentSlug, &d.Runs,
			&d.TokensInput, &d.TokensOutput, &d.CostUSD); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
