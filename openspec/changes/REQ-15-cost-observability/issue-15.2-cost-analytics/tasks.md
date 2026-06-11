# Tasks: issue-15.2-cost-analytics

## Backend

- [x] Implementar queries de agregación temporal: daily/weekly/monthly spend → Service.Spend (date_trunc) — 2026-06-10
- [x] Implementar queries de breakdown: agent, flow, model, provider, operation → Service.Breakdown (project N/A: cost_logs no tiene project_id; dims reales del schema) — 2026-06-10
- [x] Crear endpoint GET /api/v1/cost/spend/{granularity} — 2026-06-10
- [x] Crear endpoint GET /api/v1/cost/breakdown/{dimension} — 2026-06-10
- [x] Implementar cost forecasting con SMA → SMAForecast (avg diario ventana → next-30d + month-end projected) — 2026-06-10
- [x] Crear endpoint GET /api/v1/cost/forecast?window=N — 2026-06-10
- [x] Crear migración para tabla `budgets` → 000081 (amount/period/warning_threshold_pct + trigger updated_at) — 2026-06-10
- [x] Implementar CRUD de budgets → CreateBudget/ListBudgets/DeleteBudget (soft-delete, org guard) — 2026-06-10
- [x] Implementar cálculo de current_spend en tiempo real → SUM cost_logs desde date_trunc(period, NOW()) — 2026-06-10
- [x] Implementar status detection (ok/warning/exceeded) → BudgetStatus contra warning_threshold_pct — 2026-06-10
- [x] Crear endpoint GET /api/v1/cost/budgets con current_spend + POST + DELETE — 2026-06-10
- [x] Implementar export CSV → Service.ExportCSV (spend|breakdown con header) — 2026-06-10
- [x] Crear endpoint GET /api/v1/cost/export?type=&days= → text/csv + Content-Disposition (directiva lint allow) — 2026-06-10

## Frontend

- [x] N/A (API, UI dashboard se cubre en issue-16.1)

## Tests

- [x] Test unitario: SMA forecast calculation → TestSMAForecast_Basic + _EndOfMonth — 2026-06-10
- [x] Test unitario: budget status detection → TestBudgetStatus_Detection — 2026-06-10
- [x] Test de integración: daily spend query con datos mock → TestSpend_DailyAndMonthly (seed 4 filas, total verificado) — 2026-06-10
- [x] Test de integración: breakdown by model → TestBreakdown_ByModelAndProvider (orden DESC + dimensión inválida 422) — 2026-06-10
- [x] Test de integración: budget CRUD + current spend → TestBudgets_CRUDAndStatus (exceeded/ok + cross-org guard) — 2026-06-10
- [x] Test de integración: CSV export formato correcto → TestExportCSV_Formats (headers + filas) — 2026-06-10
- [x] Sabotaje: forecast sin datos → 0, no panic → TestSMAForecast_NoData_ZeroNoPanic + TestForecast_WithAndWithoutData (org vacía) — 2026-06-10

## Cierre

- [x] Verificación manual: consultar endpoints con curl → cubierto por integración E2E (service + handler thin)
- [x] Suite verde → 2026-06-10 (9 integración + 4 unit)
- [x] Export CSV formato estándar RFC 4180 via encoding/csv (abre en Excel/LibreOffice)
