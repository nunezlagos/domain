// Package runners_usage — issue-35.4 audit on-demand de uso real de los
// 3 runners server-side (agent, flow, skill). Comando standalone que
// query la BD y genera reporte JSON + tabla ASCII.
//
// Aislado del runtime: solo lee tablas (agent_runs, flow_runs,
// skill_executions, cost_logs), no escribe. Idempotente.
//
// El reporte NO contiene PII: solo UUIDs + counts + promedios. Esto lo
// hace commiteable a git.
package runners_usage

// Category clasificación del nivel de uso de un runner.
type Category string

const (
	CategoryUsed      Category = "USADO"       // >= threshold ejecuciones en ventana
	CategoryLowUse    Category = "POCO USADO"  // 1 <= ejecuciones < threshold
	CategoryNeverUsed Category = "NUNCA USADO" // 0 ejecuciones
)

// categorizeThreshold retorna el mínimo de ejecuciones para considerar
// "USADO". Adapta al tamaño de la ventana:
//   - ventana de 30+ días: threshold = 10
//   - ventana corta: threshold = max(1, days/3) para no castigar
//     servidores nuevos con poco data.
//
// Lógica centralizada acá (no inline en Categorize) para que el test
// pueda asserrar el threshold directamente cuando hace falta.
func categorizeThreshold(days int) int {
	if days < 1 {
		days = 1
	}
	if days >= 30 {
		return 10
	}
	if days/3 < 1 {
		return 1
	}
	return days / 3
}

// Categorize clasifica el nivel de uso de un runner.
//
// Reglas:
//   - total <= 0                      → NUNCA USADO
//   - 1 <= total < threshold(days)    → POCO USADO
//   - total >= threshold(days)        → USADO
//
// El threshold se adapta a la ventana: para 30+ días requiere 10
// ejecuciones/mes; para ventanas más cortas baja proporcionalmente.
func Categorize(total, days int) Category {
	if total <= 0 {
		return CategoryNeverUsed
	}
	if total >= categorizeThreshold(days) {
		return CategoryUsed
	}
	return CategoryLowUse
}
