# Tasks: issue-33.4-quota-snapshot-dashboard-ready

## Backend

- [ ] **T1**: Crear migración `migrations/000095_usage_aggregates.sql`:
  - Materialized view `usage_daily_aggregates` con UNIQUE index.
  - GRANT SELECT a `app_user` (rol que usa el API).

- [ ] **T2**: Modificar `runUsageAlertEvaluator` en
  `cmd/domain/main.go` (job existente de issue-15.3) para agregar
  al final:
  ```go
  if _, err := pool.Exec(ctx,
    `REFRESH MATERIALIZED VIEW CONCURRENTLY usage_daily_aggregates`); err != nil {
    logger.Warn("usage aggregates refresh failed", slog.Any("err", err))
  }
  ```

- [ ] **T3**: Crear `internal/api/handler/usage.go`:
  - `UsageCurrentGET(w, r)`:
    - Auth required (middleware ya existente).
    - `orgID := principal.OrganizationID`.
    - Query: 7 sub-queries agregadas (observations, agents,
      agent_runs, flow_runs, cost_usd, tokens_in, tokens_out).
    - JOIN con `org_rate_limits` y `org_flow_config` para
      limits.
    - Formatea JSON `{organization, period, counters, limits}`.
  - `UsageHistoryGET(w, r)`:
    - Parse `?days=N` (default 7, max 365).
    - Query a `usage_daily_aggregates` filtrado por org y rango.
    - Formatea JSON `{organization, history: [{date, ...}]}`.

- [ ] **T4**: Wire en `cmd/domain/main.go` con auth (los 2
  endpoints requieren Bearer OR session).

- [ ] **T5**: OpenAPI annotations (issue 32.3):
  ```go
  // UsageCurrent godoc
  // @Summary Get current usage snapshot
  // @Tags Usage
  // @Produce json
  // @Success 200 {object} UsageCurrentResponse
  // @Router /usage/current [get]
  // @Security bearerAuth
  func UsageCurrentGET(w, r) {...}
  ```
  (similar para `UsageHistoryGET`).

- [ ] **T6**: Índices: verificar que existen:
  - `cost_logs(organization_id, created_at)`.
  - `observations(organization_id, created_at)`.
  - `agent_runs(organization_id, started_at)`.
  - `flow_runs(organization_id, started_at)`.
  Si no, agregar migration con CREATE INDEX CONCURRENTLY.

- [ ] **T7**: Estructura de tests: helper `setupUsageTestData(orgID,
  n int)` que inserta N observaciones, M cost_logs, etc.

## Tests

- [ ] **T-unit-1**: `TestUsageCurrent_FormatsCorrectly**` — setup
  con 100 observations + 50 cost_logs → response tiene
  `counters.observations = 100`, `counters.cost_usd_today =
  <sum>`, etc.
- [ ] **T-unit-2**: `TestUsageCurrent_FiltersByOrg**` — setup con
  2 orgs (cada una con 100 obs) → request como org A → response
  tiene `observations = 100` (no 200).
- [ ] **T-unit-3**: `TestUsageCurrent_EmptyOrg**` — org sin uso →
  response 200 con todos los counters en 0.
- [ ] **T-unit-4**: `TestUsageHistory_DefaultDays**` — sin
  param `?days=` → retorna 7 días.
- [ ] **T-unit-5**: `TestUsageHistory_DaysLimit**` — `?days=400` →
  400 Bad Request con "days must be <= 365".
- [ ] **T-unit-6**: `TestUsageHistory_OrderedDesc**` — request →
  history está ordenada por date DESC (más reciente primero).
- [ ] **T-e2e-1**: `TestE2E_UsageCurrent_Performance**` — org con
  1M cost_logs → request a /current → response en <500ms.
- [ ] **T-e2e-2**: `TestE2E_UsageHistory_Refresh**` — insertar data
  nueva → correr el job de refresh → query /history → la nueva
  data aparece.
- [ ] **T-e2e-3**: `TestE2E_UsageIsReadOnly**` — POST/PUT/DELETE
  a /usage/current → 405.
- [ ] **T-sabotaje**: Comentar el filtro
  `WHERE organization_id = $principal` en una de las queries
  (sabotaje: no filtra por org) → test unit-2 DEBE FALLAR (ve
  data de otras orgs) → restaurar filtro → test verde.
  Documentar en commit body.
