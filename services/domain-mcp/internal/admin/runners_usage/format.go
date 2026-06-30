package runners_usage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// FormatTable produce un reporte ASCII con los runners + categorización + sources + orgs.
// No falla con datos vacíos: si todo es NUNCA USADO, igual muestra.
func FormatTable(r *Report) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("=== Runner Usage Report (last %d days) ===\n", r.WindowDays))
	b.WriteString(fmt.Sprintf("generated: %s\n", r.GeneratedAt))
	if r.Insufficient {
		b.WriteString(fmt.Sprintf("WARNING: %s\n", r.Warning))
	}
	b.WriteString("\n")

	rows := []ReportRow{r.AgentRunner, r.FlowRunner, r.SkillRunner}
	for _, row := range rows {
		b.WriteString(formatRunnerRow(row))
		b.WriteString("\n")
	}

	b.WriteString("=== Source distribution ===\n")
	if len(r.BySource) == 0 {
		b.WriteString("  (no data)\n")
	} else {

		type kv struct {
			k string
			v int
		}
		pairs := make([]kv, 0, len(r.BySource))
		total := 0
		for k, v := range r.BySource {
			pairs = append(pairs, kv{k, v})
			total += v
		}
		sort.Slice(pairs, func(i, j int) bool { return pairs[i].v > pairs[j].v })
		for _, p := range pairs {
			pct := 0
			if total > 0 {
				pct = (p.v * 100) / total
			}
			b.WriteString(fmt.Sprintf("  %-10s %d (%d%%)\n", p.k, p.v, pct))
		}
	}
	b.WriteString("\n")

	b.WriteString("=== Top orgs by usage ===\n")
	if len(r.TopOrgs) == 0 {
		b.WriteString("  (no data)\n")
	} else {
		for i, o := range r.TopOrgs {
			if i >= 10 {
				break
			}
			b.WriteString(fmt.Sprintf("  %s: %d\n", o.ID, o.Num))
		}
	}
	b.WriteString("\n")

	b.WriteString("=== Cost (USD) by runner ===\n")
	for runner, cost := range r.CostByRunner {
		b.WriteString(fmt.Sprintf("  %-15s $%.2f\n", runner, cost))
	}
	b.WriteString("\n")


	if r.AgentRunner.Category == CategoryNeverUsed ||
		r.FlowRunner.Category == CategoryNeverUsed ||
		r.SkillRunner.Category == CategoryNeverUsed {
		b.WriteString("WARNING: at least one runner is NUNCA USADO. ")
		b.WriteString("Re-evaluate maintenance cost: maybe move to MCP-direct or mark as beta.\n")
	}
	return b.String()
}

func formatRunnerRow(row ReportRow) string {
	tag := ""
	if row.Category == CategoryNeverUsed {
		tag = " [WARNING]"
	}
	if row.Total == 0 {
		return fmt.Sprintf("%s: %s%s\n  0 ejecuciones en %d días\n",
			row.Runner, row.Category, tag, 0)
	}
	avgStr := ""
	if row.AvgDurationSec > 0 {
		avgStr = fmt.Sprintf(", avg %.1fs", row.AvgDurationSec)
	}
	return fmt.Sprintf("%s: %s (%d ejecuciones, %.0f%% success%s)%s\n",
		row.Runner, row.Category, row.Total, row.SuccessRate*100, avgStr, tag)
}

// FormatJSON serializa el report como JSON estructurado (con indentation).
// Apto para commit a git: solo UUIDs, counts, promedios. NO nombres ni
// emails.
func FormatJSON(r *Report) ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// WriteReport persiste el JSON a reports/runners-usage-<YYYYMMDD>.json.
// Crea el dir si no existe. Retorna el path absoluto escrito.
func WriteReport(r *Report, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir reports dir: %w", err)
	}
	body, err := FormatJSON(r)
	if err != nil {
		return "", err
	}
	date := time.Now().UTC().Format("20060102")
	path := filepath.Join(dir, fmt.Sprintf("runners-usage-%s.json", date))
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return "", fmt.Errorf("write report: %w", err)
	}
	return path, nil
}
