// Command: go run ./cmd/domain-admin/runners_usage/main.go (helper para
// regenerar el reporte con fake data). NO se usa en producción; es
// helper de demo del issue-35.4 para generar el report inicial.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"nunezlagos/domain/internal/admin/runners_usage"
)

func main() {
	days := flag.Int("days", 30, "ventana de análisis (default 30)")
	out := flag.String("out", "reports", "directorio de salida")
	flag.Parse()

	// Fake data plausible para pre-launch / issue-35.4 demo.
	// Ver docs/audit/2026-06-runners-coverage.md para el racional
	// de cada número.
	agent := runners_usage.RunnerUsage{Total: 245, Succeeded: 213, Failed: 32, SuccessRate: 0.87, AvgDurationSec: 12.3}
	flow := runners_usage.RunnerUsage{Total: 1023, Succeeded: 941, Failed: 82, SuccessRate: 0.92, AvgDurationSec: 45.6}
	skill := runners_usage.RunnerUsage{Total: 0} // nunca usado server-side (issue-35.2)

	sources := map[string]int{
		"mcp":     800,
		"cron":    350,
		"webhook": 150,
	}
	orgs := []runners_usage.TopEntity{
		{ID: "11111111-1111-1111-1111-111111111111", Num: 500},
		{ID: "22222222-2222-2222-2222-222222222222", Num: 320},
		{ID: "33333333-3333-3333-3333-333333333333", Num: 280},
	}
	costs := map[string]float64{
		"agent_runner": 12.5,
		"flow_runner":  45.6,
		"skill_runner": 0.0,
	}

	r := runners_usage.BuildReportFromData(*days, agent, flow, skill, sources, orgs, costs)

	// Top agents de ejemplo (los 5 más usados).
	r.AgentRunner.TopEntities = []runners_usage.TopEntity{
		{ID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", Num: 89},
		{ID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", Num: 67},
		{ID: "cccccccc-cccc-cccc-cccc-cccccccccccc", Num: 45},
		{ID: "dddddddd-dddd-dddd-dddd-dddddddddddd", Num: 30},
		{ID: "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee", Num: 14},
	}
	r.FlowRunner.TopEntities = []runners_usage.TopEntity{
		{ID: "ffffffff-ffff-ffff-ffff-ffffffffffff", Num: 234},
		{ID: "99999999-9999-9999-9999-999999999999", Num: 187},
		{ID: "88888888-8888-8888-8888-888888888888", Num: 145},
		{ID: "77777777-7777-7777-7777-777777777777", Num: 100},
		{ID: "66666666-6666-6666-6666-666666666666", Num: 89},
	}
	r.AgentRunner.HighFailure = []runners_usage.HighFailRate{
		{ID: "12121212-1212-1212-1212-121212121212", Num: 12, Failed: 9, FailRate: 0.75},
		{ID: "13131313-1313-1313-1313-131313131313", Num: 8, Failed: 5, FailRate: 0.63},
	}

	abs, err := filepath.Abs(*out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "abs: %v\n", err)
		os.Exit(1)
	}
	path, err := runners_usage.WriteReport(r, abs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "write: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(runners_usage.FormatTable(r))
	fmt.Printf("report written: %s\n", path)
}
