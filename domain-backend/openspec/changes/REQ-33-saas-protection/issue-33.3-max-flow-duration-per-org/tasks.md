# Tasks: issue-33.3-max-flow-duration-per-org

## Backend

- [ ] **T1**: Crear migración `migrations/000094_org_flow_config.sql`:
  - Tabla `org_flow_config` con constraint de rango 10-86400.
  - Índice en `organization_id` (PK ya cubre).

- [ ] **T2**: Seeder `internal/seeds/org_flow_config_seeder.go`:
  INSERT 300 para cada org sin entry (idempotente).

- [ ] **T3**: `internal/service/flow/budget.go`:
  - `GetMaxDuration(ctx, pool, orgID) (time.Duration, error)`.
  - Cache in-memory 30s con mutex.

- [ ] **T4**: Modificar `flowrunner.RunRecovery` en
  `cmd/domain/main.go`:
  - Cambiar la query para hacer JOIN con `org_flow_config`.
  - Para cada flow_run excedido: cancelar context (via el
    `activeFlowCancels` map) Y marcar `failed` con
    `error_code: "max_duration_exceeded"`.

- [ ] **T5**: Métrica `metrics.FlowRunCancelledByMaxDuration`:
  - Counter con label `org_id`.
  - Incrementar en RunRecovery post-cancel.

- [ ] **T6**: Admin endpoint `PUT /api/v1/admin/flow-config/{orgID}`:
  - Body: `{max_flow_duration_seconds: 60}`.
  - Validar rango 10-86400.
  - UPSERT en `org_flow_config`.
  - Audit log.

- [ ] **T7**: Admin endpoint `GET /api/v1/admin/flow-config/{orgID}`
  para ver el config actual.

- [ ] **T8**: Logging estructurado post-cancel:
  ```go
  slog.Warn("flow_run cancelled by max_duration",
    "flow_run_id", id, "org_id", orgID,
    "duration_seconds", actualDuration,
    "budget_seconds", maxDuration)
  ```

## Tests

- [ ] **T-unit-1**: `TestGetMaxDuration_Default**` — org sin
  entry → 300s.
- [ ] **T-unit-2**: `TestGetMaxDuration_Custom**` — org con 60s →
  retorna 60s.
- [ ] **T-unit-3**: `TestRunRecovery_CancelsExceded**` — flow_run
  con started_at = NOW() - 400s, org budget 300s → flow_run se
  marca failed + context cancel llamado.
- [ ] **T-unit-4**: `TestRunRecovery_DoesNotCancelWithin**` —
  flow_run con started_at = NOW() - 100s, budget 300s → NO se
  cancela.
- [ ] **T-unit-5**: `TestRunRecovery_PerOrgBudget**` — 2 flow_runs
  de distintas orgs con distintos budgets → cada uno se cancela
  según SU budget.
- [ ] **T-e2e-1**: `TestE2E_FlowCancelledAndContextPropagates**` —
  flow_run con sub-flows anidados → cancel del top propaga a
  los sub-flows (verificar con un flag en los handlers de
  test).
- [ ] **T-e2e-2**: `TestE2E_AdminCanUpdateBudget**` — admin PUT
  /flow-config/{org} → tabla actualizada → próximo RunRecovery
  usa el nuevo budget.
- [ ] **T-sabotaje**: Comentar el JOIN con `org_flow_config` en
  RunRecovery (sabotaje: siempre usa 300s hardcoded) → test
  unit-5 DEBE FALLAR (flow con budget custom 60s no se cancela
  a los 100s) → restaurar JOIN → test verde. Documentar en
  commit body.
