package runners_usage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunnerUsage métricas agregadas de un runner en la ventana.
type RunnerUsage struct {
	Total          int
	Succeeded      int
	Failed         int
	SuccessRate    float64
	AvgDurationSec float64
}

// QueryAgentRuns retorna métricas agregadas de agent_runs en los últimos `days`.
//
// Requiere DB. En tests unitarios se skipea con t.Skip.
func QueryAgentRuns(ctx context.Context, pool *pgxpool.Pool, days int) (RunnerUsage, error) {
	var u RunnerUsage
	err := pool.QueryRow(ctx, `
		SELECT
		  COUNT(*) AS total,
		  COUNT(*) FILTER (WHERE status = 'completed') AS succeeded,
		  COUNT(*) FILTER (WHERE status = 'failed') AS failed,
		  COALESCE(AVG(EXTRACT(EPOCH FROM (finished_at - started_at))) FILTER (
		    WHERE finished_at IS NOT NULL AND started_at IS NOT NULL
		  ), 0)::float8 AS avg_dur
		FROM agent_runs
		WHERE started_at >= NOW() - ($1 || ' days')::interval
	`, fmt.Sprintf("%d", days)).Scan(&u.Total, &u.Succeeded, &u.Failed, &u.AvgDurationSec)
	if err != nil {
		return u, fmt.Errorf("query agent_runs: %w", err)
	}
	if u.Total > 0 {
		u.SuccessRate = float64(u.Succeeded) / float64(u.Total)
	}
	return u, nil
}

// QueryFlowRuns análogo a QueryAgentRuns pero sobre flow_runs.
func QueryFlowRuns(ctx context.Context, pool *pgxpool.Pool, days int) (RunnerUsage, error) {
	var u RunnerUsage
	err := pool.QueryRow(ctx, `
		SELECT
		  COUNT(*) AS total,
		  COUNT(*) FILTER (WHERE status = 'completed') AS succeeded,
		  COUNT(*) FILTER (WHERE status = 'failed') AS failed,
		  COALESCE(AVG(EXTRACT(EPOCH FROM (finished_at - started_at))) FILTER (
		    WHERE finished_at IS NOT NULL AND started_at IS NOT NULL
		  ), 0)::float8 AS avg_dur
		FROM flow_runs
		WHERE started_at >= NOW() - ($1 || ' days')::interval
	`, fmt.Sprintf("%d", days)).Scan(&u.Total, &u.Succeeded, &u.Failed, &u.AvgDurationSec)
	if err != nil {
		return u, fmt.Errorf("query flow_runs: %w", err)
	}
	if u.Total > 0 {
		u.SuccessRate = float64(u.Succeeded) / float64(u.Total)
	}
	return u, nil
}

// QuerySkillExecutions análogo para skill_executions.
// Si la tabla no existe (deploy fresh), retorna cero sin error.
func QuerySkillExecutions(ctx context.Context, pool *pgxpool.Pool, days int) (RunnerUsage, error) {
	var u RunnerUsage

	var exists bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1 FROM information_schema.tables
		  WHERE table_schema = 'public' AND table_name = 'skill_executions'
		)
	`).Scan(&exists)
	if err != nil {
		return u, fmt.Errorf("check skill_executions exists: %w", err)
	}
	if !exists {
		return u, nil // tabla no existe → 0 ejecuciones
	}
	err = pool.QueryRow(ctx, `
		SELECT
		  COUNT(*) AS total,
		  COUNT(*) FILTER (WHERE status = 'succeeded') AS succeeded,
		  COUNT(*) FILTER (WHERE status = 'failed') AS failed,
		  COALESCE(AVG(EXTRACT(EPOCH FROM (finished_at - started_at))) FILTER (
		    WHERE finished_at IS NOT NULL AND started_at IS NOT NULL
		  ), 0)::float8 AS avg_dur
		FROM skill_executions
		WHERE started_at >= NOW() - ($1 || ' days')::interval
	`, fmt.Sprintf("%d", days)).Scan(&u.Total, &u.Succeeded, &u.Failed, &u.AvgDurationSec)
	if err != nil {
		return u, fmt.Errorf("query skill_executions: %w", err)
	}
	if u.Total > 0 {
		u.SuccessRate = float64(u.Succeeded) / float64(u.Total)
	}
	return u, nil
}

// QueryBySource cuenta ejecuciones por source (MCP, cron, webhook) usando
// el campo `trigger_type` que los 3 runners escriben. Combina los 3 en
// un solo map.
func QueryBySource(ctx context.Context, pool *pgxpool.Pool, days int) (map[string]int, error) {
	out := map[string]int{}
	rows, err := pool.Query(ctx, `
		SELECT source, COUNT(*) FROM (
		  SELECT 'mcp' AS source, 1 FROM agent_runs
		    WHERE trigger_type = 'mcp' AND started_at >= NOW() - ($1 || ' days')::interval
		  UNION ALL
		  SELECT 'cron' AS source, 1 FROM agent_runs
		    WHERE trigger_type = 'cron' AND started_at >= NOW() - ($1 || ' days')::interval
		  UNION ALL
		  SELECT 'webhook' AS source, 1 FROM agent_runs
		    WHERE trigger_type = 'webhook' AND started_at >= NOW() - ($1 || ' days')::interval
		  UNION ALL
		  SELECT 'mcp' AS source, 1 FROM flow_runs
		    WHERE trigger_type = 'mcp' AND started_at >= NOW() - ($1 || ' days')::interval
		  UNION ALL
		  SELECT 'cron' AS source, 1 FROM flow_runs
		    WHERE trigger_type = 'cron' AND started_at >= NOW() - ($1 || ' days')::interval
		  UNION ALL
		  SELECT 'webhook' AS source, 1 FROM flow_runs
		    WHERE trigger_type = 'webhook' AND started_at >= NOW() - ($1 || ' days')::interval
		) s GROUP BY source
	`, fmt.Sprintf("%d", days))
	if err != nil {
		return out, fmt.Errorf("query by source: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var source string
		var n int
		if err := rows.Scan(&source, &n); err == nil {
			out[source] = n
		}
	}
	return out, nil
}

// QueryTopOrgs retorna las top `limit` orgs por total de runs.
//
// Tras las migraciones 000142/000143 se eliminó la columna organization_id
// de agent_runs y flow_runs, y la tabla organizations dejó de existir. Sin
// dimensión de organización no hay top orgs que agregar, por lo que retorna
// vacío. Se conserva la firma pública para no romper callers ni la forma del
// Report (TopOrgs).
func QueryTopOrgs(ctx context.Context, pool *pgxpool.Pool, days, limit int) ([]TopEntity, error) {
	return []TopEntity{}, nil
}

// QueryHighFailureAgents retorna los top `limit` agents con >50% failure
// rate y al menos 5 runs.
func QueryHighFailureAgents(ctx context.Context, pool *pgxpool.Pool, days, limit int) ([]HighFailRate, error) {
	var out []HighFailRate
	rows, err := pool.Query(ctx, `
		SELECT agent_id::text, COUNT(*) AS n,
		       COUNT(*) FILTER (WHERE status = 'failed') AS failed,
		       ROUND(
		         (COUNT(*) FILTER (WHERE status = 'failed'))::numeric / COUNT(*),
		         2
		       ) AS fail_rate
		FROM agent_runs
		WHERE started_at >= NOW() - ($1 || ' days')::interval
		GROUP BY agent_id
		HAVING COUNT(*) >= 5
		  AND COUNT(*) FILTER (WHERE status = 'failed')::numeric / COUNT(*) > 0.5
		ORDER BY fail_rate DESC LIMIT $2
	`, fmt.Sprintf("%d", days), limit)
	if err != nil {
		return out, fmt.Errorf("query high failure: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var n, failed int
		var rate float64
		if err := rows.Scan(&id, &n, &failed, &rate); err == nil {
			out = append(out, HighFailRate{ID: id, Num: n, Failed: failed, FailRate: rate})
		}
	}
	return out, nil
}




// BuildReportFromData construye un Report a partir de las métricas pre-collectadas.
// Útil para tests con fake data o para cuando se quiere generar el reporte
// sin tocar la BD.
//
// days = ventana de análisis (afecta categorización).
func BuildReportFromData(days int, agent, flow, skill RunnerUsage, bySource map[string]int, topOrgs []TopEntity, costs map[string]float64) *Report {
	r := newReport(days)
	r.AgentRunner = ReportRow{
		Runner: "agent_runner", Category: Categorize(agent.Total, days),
		Total: agent.Total, Succeeded: agent.Succeeded, Failed: agent.Failed,
		SuccessRate: agent.SuccessRate, AvgDurationSec: agent.AvgDurationSec,
	}
	r.FlowRunner = ReportRow{
		Runner: "flow_runner", Category: Categorize(flow.Total, days),
		Total: flow.Total, Succeeded: flow.Succeeded, Failed: flow.Failed,
		SuccessRate: flow.SuccessRate, AvgDurationSec: flow.AvgDurationSec,
	}
	r.SkillRunner = ReportRow{
		Runner: "skill_runner", Category: Categorize(skill.Total, days),
		Total: skill.Total, Succeeded: skill.Succeeded, Failed: skill.Failed,
		SuccessRate: skill.SuccessRate, AvgDurationSec: skill.AvgDurationSec,
	}
	if bySource != nil {
		r.BySource = bySource
	}
	if topOrgs != nil {
		r.TopOrgs = topOrgs
	}
	if costs != nil {
		r.CostByRunner = costs
	}
	return r
}

// DateFromNow helper para tests que necesitan timestamps fake.
func DateFromNow(daysAgo int) time.Time {
	return time.Now().UTC().AddDate(0, 0, -daysAgo)
}
