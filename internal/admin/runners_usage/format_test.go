package runners_usage

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewReport_PopulatesDefaults(t *testing.T) {
	r := newReport(30)
	require.Equal(t, 30, r.WindowDays)
	require.NotEmpty(t, r.GeneratedAt)
	require.False(t, r.Insufficient)
	require.Empty(t, r.Warning)
	// Zero value of Category es "". El caller lo setea explícitamente
	// cuando aplica Categorize() al Total. Acá verificamos el
	// "shape" del report, no la clasificación.
	require.Equal(t, "", string(r.AgentRunner.Category))
}

func TestNewReport_FlagsInsufficientData(t *testing.T) {
	r := newReport(3)
	require.True(t, r.Insufficient)
	require.Contains(t, r.Warning, "insufficient data")
}

func TestFormatTable_IncludesRunnerCategory(t *testing.T) {
	r := newReport(30)
	r.AgentRunner = ReportRow{Runner: "agent_runner", Category: CategoryUsed, Total: 245, Succeeded: 213, Failed: 32, SuccessRate: 0.87}
	r.FlowRunner = ReportRow{Runner: "flow_runner", Category: CategoryUsed, Total: 1023}
	r.SkillRunner = ReportRow{Runner: "skill_runner", Category: CategoryNeverUsed, Total: 0}

	out := FormatTable(r)
	require.Contains(t, out, "agent_runner")
	require.Contains(t, out, "USADO")
	require.Contains(t, out, "flow_runner")
	require.Contains(t, out, "skill_runner")
	require.Contains(t, out, "NUNCA USADO")
}

func TestFormatTable_WarnsNeverUsed(t *testing.T) {
	r := newReport(30)
	r.SkillRunner = ReportRow{Runner: "skill_runner", Category: CategoryNeverUsed}
	out := FormatTable(r)
	require.Contains(t, out, "WARNING")
	require.Contains(t, out, "NUNCA")
	// El bloque final con "at least one runner is NUNCA USADO" es la
	// acción decisional: el operador lo lee y decide si matar el runner.
	// Sin ese bloque el reporte miente sobre el impacto.
	require.Contains(t, out, "at least one runner is NUNCA USADO")
}

func TestFormatTable_ShowsSourceDistribution(t *testing.T) {
	r := newReport(30)
	r.BySource = map[string]int{"mcp": 800, "cron": 350, "webhook": 150}
	out := FormatTable(r)
	require.Contains(t, out, "mcp")
	require.Contains(t, out, "800")
	require.Contains(t, out, "cron")
	require.Contains(t, out, "350")
}

// TestFormatJSON_NoPII assserta que el JSON no contiene emails ni campos
// típicos de PII. Esto es lo que hace al reporte commiteable a git.
func TestFormatJSON_NoPII(t *testing.T) {
	r := newReport(30)
	r.AgentRunner = ReportRow{
		Runner: "agent_runner", Category: CategoryUsed, Total: 50, Succeeded: 40, Failed: 10,
		SuccessRate: 0.8, TopEntities: []TopEntity{{ID: "11111111-1111-1111-1111-111111111111", Num: 50}},
	}
	r.FlowRunner.TopEntities = []TopEntity{{ID: "22222222-2222-2222-2222-222222222222", Num: 200}}
	r.TopOrgs = []TopEntity{{ID: "33333333-3333-3333-3333-333333333333", Num: 500}}
	r.BySource = map[string]int{"mcp": 800}
	r.CostByRunner = map[string]float64{"agent_runner": 12.5, "flow_runner": 45.6}

	out, err := FormatJSON(r)
	require.NoError(t, err)
	body := string(out)

	// PII forbidden patterns.
	require.NotContains(t, body, "@", "JSON contains @ — possible email")
	require.NotRegexp(t, regexp.MustCompile(`(?i)"name"`), body, "JSON contains name field")
	require.NotRegexp(t, regexp.MustCompile(`(?i)"email"`), body, "JSON contains email field")
	require.NotRegexp(t, regexp.MustCompile(`(?i)"user"`), body, "JSON contains user field")

	// Solo UUIDs como ID: hex + dashes.
	uuidRe := regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	matches := uuidRe.FindAllString(body, -1)
	require.GreaterOrEqual(t, len(matches), 2, "expected at least 2 UUIDs in JSON output")

	// Métricas sí están.
	require.Contains(t, body, `"total": 50`)
	require.Contains(t, body, `"category": "USADO"`)
}

func TestWriteReport_CreatesFile(t *testing.T) {
	tmp := t.TempDir()
	r := newReport(30)
	r.AgentRunner = ReportRow{Runner: "agent_runner", Category: CategoryUsed, Total: 50}

	path, err := WriteReport(r, tmp)
	require.NoError(t, err)
	require.FileExists(t, path)
	require.Contains(t, path, "runners-usage-")
	require.Contains(t, path, ".json")
}
