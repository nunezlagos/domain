# Proposal: issue-15.2-cost-analytics

## Intención

Implementar un motor de analytics que consulta token_usage y produce agregaciones temporales (día/semana/mes) y dimensionales (proyecto/agente/flow/modelo/proveedor). Incluye forecast simple, budget tracking y export CSV.

## Scope

**Incluye:**
- API endpoints de analytics:
  - GET /api/v1/cost/spend/daily?from=&to=
  - GET /api/v1/cost/spend/weekly?from=&to=
  - GET /api/v1/cost/spend/monthly?from=&to=
  - GET /api/v1/cost/breakdown/project?from=&to=
  - GET /api/v1/cost/breakdown/agent?from=&to=
  - GET /api/v1/cost/breakdown/flow?from=&to=
  - GET /api/v1/cost/breakdown/model?from=&to=
  - GET /api/v1/cost/breakdown/provider?from=&to=
  - GET /api/v1/cost/forecast?days=30
  - GET /api/v1/cost/budget
  - GET /api/v1/cost/export?type=spend&format=csv
- Budget CRUD: POST/PUT/GET/DELETE /api/v1/cost/budgets
- Agregaciones SQL con date_trunc, GROUP BY
- Cost forecasting: promedio móvil simple (SMA) de últimos N días
- Budget tracking: comparar gasto actual vs budget configurado

**Excluye:**
- UI dashboard (issue-16.1)
- Alertas (issue-15.3)
- Machine learning forecasting (SMA es suficiente)

## Enfoque técnico

**SQL queries de agregación:**
```sql
-- Daily spend
SELECT DATE(created_at) AS date,
       SUM(total_tokens) AS total_tokens,
       SUM(cost) AS total_cost,
       COUNT(*) AS call_count
FROM token_usage
WHERE created_at BETWEEN $1 AND $2
GROUP BY DATE(created_at)
ORDER BY date;

-- Breakdown by model
SELECT model, provider,
       SUM(total_tokens) AS total_tokens,
       SUM(cost) AS total_cost,
       COUNT(*) AS call_count,
       ROUND(SUM(cost) * 100.0 / SUM(SUM(cost)) OVER (), 2) AS percentage
FROM token_usage
WHERE created_at BETWEEN $1 AND $2
GROUP BY model, provider
ORDER BY total_cost DESC;
```

**Forecast (SMA):**
```go
func Forecast(dailyCosts []float64, days int) float64 {
    if len(dailyCosts) == 0 {
        return 0
    }
    window := min(30, len(dailyCosts))
    sum := 0.0
    for i := len(dailyCosts) - window; i < len(dailyCosts); i++ {
        sum += dailyCosts[i]
    }
    avg := sum / float64(window)
    return avg * float64(days)
}
```

**Budget model:**
```go
type Budget struct {
    ID          string  `json:"id"`
    ProjectID   string  `json:"project_id,omitempty"`
    Limit       float64 `json:"limit"`       // USD
    Period      string  `json:"period"`      // "monthly", "weekly"
    WarnAt      float64 `json:"warn_at"`     // percentage 0-100
    CurrentSpend float64 `json:"current_spend"` // computed
    Remaining   float64 `json:"remaining"`
    Status      string  `json:"status"`      // "ok", "warning", "exceeded"
}
```

**CSV Export:**
```go
func ExportCSV(w io.Writer, headers []string, rows [][]string) error {
    writer := csv.NewWriter(w)
    writer.Write(headers)
    for _, row := range rows {
        writer.Write(row)
    }
    writer.Flush()
    return writer.Error()
}
```

## Riesgos

| Riesgo | Mitigación |
|--------|------------|
| Queries lentas con muchos datos | Indices en created_at, run_id, model. Materialized views para dashboards. |
| Forecast impreciso | Documentar que es estimación lineal simple. No decisions automáticas basadas en forecast. |
| Budget race condition | Calcular current_spend al momento de la consulta, no cacheado. |
| CSV con muchos datos | Streaming response, no cargar todo en memoria. |

## Testing

- Unit: forecast calculation
- Integration: aggregates queries con datos mock
- Integration: CSV export formato correcto
- Sabotaje: budget sin límite → status "unlimited"
