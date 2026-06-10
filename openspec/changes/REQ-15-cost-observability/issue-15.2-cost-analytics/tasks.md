# Tasks: issue-15.2-cost-analytics

## Backend

- [ ] Implementar queries de agregación temporal: daily/weekly/monthly spend
- [ ] Implementar queries de breakdown: project, agent, flow, model, provider
- [ ] Crear endpoint GET /api/v1/cost/spend/daily|weekly|monthly
- [ ] Crear endpoint GET /api/v1/cost/breakdown/{dimension}
- [ ] Implementar cost forecasting con SMA
- [ ] Crear endpoint GET /api/v1/cost/forecast
- [ ] Crear migración para tabla `budgets`
- [ ] Implementar CRUD de budgets
- [ ] Implementar cálculo de current_spend en tiempo real
- [ ] Implementar status detection (ok/warning/exceeded)
- [ ] Crear endpoint GET /api/v1/cost/budgets con current_spend
- [ ] Implementar export CSV para cualquier agregación
- [ ] Crear endpoint GET /api/v1/cost/export?type=&from=&to=

## Frontend

- [ ] N/A (API, UI dashboard se cubre en issue-16.1)

## Tests

- [ ] Test unitario: SMA forecast calculation
- [ ] Test unitario: budget status detection
- [ ] Test de integración: daily spend query con datos mock
- [ ] Test de integración: breakdown by model
- [ ] Test de integración: budget CRUD + current spend
- [ ] Test de integración: CSV export formato correcto
- [ ] Sabotaje: forecast sin datos → 0, no panic

## Cierre

- [ ] Verificación manual: consultar endpoints con curl
- [ ] Suite verde: `go test ./internal/cost/...`
- [ ] Export CSV verificado en Excel/LibreOffice
