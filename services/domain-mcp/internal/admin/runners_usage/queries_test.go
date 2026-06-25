package runners_usage

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestQueryAgentRuns_RequiresDB(t *testing.T) {
	t.Skip("requires integration: needs real Postgres with agent_runs table")
	var pool *pgxpool.Pool
	_, _ = QueryAgentRuns(context.Background(), pool, 30)
}

func TestBuildReportFromData_Categorizes(t *testing.T) {
	agent := RunnerUsage{Total: 245, Succeeded: 213, Failed: 32, SuccessRate: 0.87, AvgDurationSec: 12.3}
	flow := RunnerUsage{Total: 1023, Succeeded: 941, Failed: 82, SuccessRate: 0.92, AvgDurationSec: 45.6}
	skill := RunnerUsage{Total: 0}
	sources := map[string]int{"mcp": 800, "cron": 350, "webhook": 150}
	orgs := []TopEntity{{ID: "11111111-1111-1111-1111-111111111111", Num: 500}}
	costs := map[string]float64{"agent_runner": 12.5, "flow_runner": 45.6}

	r := BuildReportFromData(30, agent, flow, skill, sources, orgs, costs)
	require.Equal(t, 30, r.WindowDays)
	require.Equal(t, CategoryUsed, r.AgentRunner.Category)
	require.Equal(t, CategoryUsed, r.FlowRunner.Category)
	require.Equal(t, CategoryNeverUsed, r.SkillRunner.Category)
	require.Equal(t, 245, r.AgentRunner.Total)
	require.Equal(t, 1023, r.FlowRunner.Total)
	require.Equal(t, 0, r.SkillRunner.Total)
	require.Equal(t, 800, r.BySource["mcp"])
	require.Equal(t, 500, r.TopOrgs[0].Num)
	require.InDelta(t, 12.5, r.CostByRunner["agent_runner"], 0.001)
}

func TestBuildReportFromData_LowUse(t *testing.T) {
	agent := RunnerUsage{Total: 3}
	r := BuildReportFromData(30, agent, RunnerUsage{}, RunnerUsage{}, nil, nil, nil)
	require.Equal(t, CategoryLowUse, r.AgentRunner.Category,
		"3 ejecuciones en 30 días debe ser POCO USADO (threshold=10)")
}

func TestBuildReportFromData_ShortWindowAdjusts(t *testing.T) {

	agent := RunnerUsage{Total: 1}
	r := BuildReportFromData(5, agent, RunnerUsage{}, RunnerUsage{}, nil, nil, nil)
	require.Equal(t, CategoryUsed, r.AgentRunner.Category)

	agent2 := RunnerUsage{Total: 0}
	r2 := BuildReportFromData(5, agent2, RunnerUsage{}, RunnerUsage{}, nil, nil, nil)
	require.Equal(t, CategoryNeverUsed, r2.AgentRunner.Category)
}
