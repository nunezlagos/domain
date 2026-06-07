# Design: HU-15.2-cost-analytics

## Decisión arquitectónica

**Query-based analytics (no pre-agregados):** Calculamos agregaciones al momento de la consulta con SQL. Para dashboards frecuentes, podemos crear materialized views si la performance lo requiere.

**Budget tracking:**
- Budgets se guardan en tabla `budgets` con project_id opcional y período
- Current spend se calcula en tiempo real con SUM de token_usage del período actual
- Status se determina comparando current_spend vs limit:
  - < warn_at% → "ok"
  - >= warn_at% y < 100% → "warning"
  - >= 100% → "exceeded"

**Forecast:**
- Simple Moving Average de últimos 30 días
- Proyecta los próximos N días
- Intervalo de confianza: ± desvío estándar * 1.96 (95% confianza)

## Diagrama

```
API Endpoint
  │
  ▼
AnalyticsService
  ├── GetDailySpend(from, to) → SQL: date_trunc + SUM
  ├── GetBreakdownByModel(from, to) → SQL: GROUP BY model
  ├── GetForecast(days) → SQL: daily spend → SMA → projection
  ├── GetBudgets() → SQL: budgets + current spend
  └── ExportCSV(type, from, to) → SQL → csv.Writer

Budget flow:
POST /api/v1/cost/budgets → store budget
GET  /api/v1/cost/budgets → return budgets with current_spend computed
```

## TDD plan

1. **Red:** Test `TestGetDailySpend` con 14 días de datos mock
2. **Green:** Implementar query de daily spend
3. **Refactor:** Extraer DateTrunc a util, parametrizar granularidad
4. **Iterar:** Breakdowns, forecast, budget, CSV
5. **Sabotaje:**
