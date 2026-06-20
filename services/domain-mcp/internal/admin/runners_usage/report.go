package runners_usage

import "time"

// ReportRow un runner (agent, flow, skill) y sus métricas.
type ReportRow struct {
	Runner         string         `json:"runner"`
	Category       Category       `json:"category"`
	Total          int            `json:"total"`
	Succeeded      int            `json:"succeeded"`
	Failed         int            `json:"failed"`
	SuccessRate    float64        `json:"success_rate"`
	AvgDurationSec float64        `json:"avg_duration_sec"`
	TopEntities    []TopEntity    `json:"top_entities,omitempty"`
	HighFailure    []HighFailRate `json:"high_failure_entities,omitempty"`
}

// TopEntity genérico: agent_id o flow_id o org_id + count.
type TopEntity struct {
	ID  string `json:"id"`
	Num int    `json:"n"`
}

// HighFailRate row con tasa de fallos > 50% y al menos 5 runs.
type HighFailRate struct {
	ID       string  `json:"id"`
	Num      int     `json:"n"`
	Failed   int     `json:"failed"`
	FailRate float64 `json:"fail_rate"`
}

// Report estructura completa del reporte on-demand.
type Report struct {
	WindowDays   int                `json:"window_days"`
	GeneratedAt  string             `json:"generated_at"`
	Insufficient bool               `json:"insufficient_data"`
	Warning      string             `json:"warning,omitempty"`
	AgentRunner  ReportRow          `json:"agent_runner"`
	FlowRunner   ReportRow          `json:"flow_runner"`
	SkillRunner  ReportRow          `json:"skill_runner"`
	BySource     map[string]int     `json:"by_source"`
	TopOrgs      []TopEntity        `json:"top_orgs"`
	CostByRunner map[string]float64 `json:"cost_usd_by_runner"`
}

// newReport construye un report vacío con timestamp.
func newReport(days int) *Report {
	now := time.Now().UTC().Format(time.RFC3339)
	r := &Report{
		WindowDays:   days,
		GeneratedAt:  now,
		BySource:     map[string]int{},
		CostByRunner: map[string]float64{},
		TopOrgs:      []TopEntity{},
	}
	if days < 7 {
		r.Insufficient = true
		r.Warning = "insufficient data: less than 7 days available (recommend 30+ days for accurate analysis)"
	}
	return r
}
