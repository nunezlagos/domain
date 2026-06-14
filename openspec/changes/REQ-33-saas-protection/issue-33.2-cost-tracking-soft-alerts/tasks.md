# Tasks: issue-33.2-cost-tracking-soft-alerts

## Backend

- [ ] **T1**: Crear migración `migrations/000093_cost_alerts.sql`:
  - Tabla `org_cost_alert_thresholds`.
  - Tabla `cost_alerts_sent` con UNIQUE(organization_id, alert_date).
  - Índices en `cost_logs(organization_id, created_at)` para la
    agregación.

- [ ] **T2**: Crear paquete `internal/service/usagealerts/threshold/`:
  - `checker.go` — `CheckThresholds(ctx, pool) ([]Alert, error)`.
    Query: orgs con SUM(cost_usd) del día UTC >= threshold.
  - `sender.go` — `SendAlerts(ctx, alerts, emailSender)`.
    Para cada alert: INSERT ON CONFLICT, si affected=1, enviar.
  - `render.go` — `RenderEmail(alert) (subject, body string)`.
    Template plain text con breakdown.

- [ ] **T3**: Modificar `runUsageAlertEvaluator` en
  `cmd/domain/main.go` para invocar el threshold checker + sender
  después del flujo actual. Solo si SMTP configurado (o
  fallback log).

- [ ] **T4**: Admin endpoint `PUT /api/v1/admin/cost-threshold/{orgID}`:
  - Body: `{daily_usd: 50}`.
  - Auth: admin role.
  - UPSERT en `org_cost_alert_thresholds`.
  - Audit log con `actor=<admin>, action=update, resource=org/<id>`.

- [ ] **T5**: GET endpoint equivalente `GET /api/v1/admin/cost-threshold/{orgID}`
  para ver el threshold actual (útil para el dashboard admin).

- [ ] **T6**: Aggregation breakdown por provider/model: query
  adicional que corre solo para las orgs que crossed (no para
  todas). Output: `[{provider, model, cost_usd}]`.

- [ ] **T7**: Seeder para defaults: para cada org existente,
  INSERT en `org_cost_alert_thresholds` con $100 default (si no
  tiene ya). Idempotente.

## Tests

- [ ] **T-unit-1**: `TestCheckThresholds_DetectsCrossed**` —
  org con cost_logs que suman $50 + threshold $10 → retorna
  alert.
- [ ] **T-unit-2**: `TestCheckThresholds_NoAlertBelow**` — org con
  $5 + threshold $10 → NO retorna alert.
- [ ] **T-unit-3**: `TestSendAlerts_Dedup**` — enviar 2 veces el
  mismo alert → solo 1 email (segundo INSERT ON CONFLICT no
  afecta filas).
- [ ] **T-unit-4**: `TestRenderEmail_HasBreakdown**` — alert con
  cost por Anthropic + OpenAI → email body tiene líneas con
  cada provider.
- [ ] **T-e2e-1**: `TestE2E_CostAlertFlow**` — insertar cost_logs
  que cruzan threshold → correr `CheckThresholds + SendAlerts` →
  email mockeado fue llamado con subject correcto.
- [ ] **T-e2e-2**: `TestE2E_NoSMTPFallback**` — sin
  `DOMAIN_SMTP_HOST` → email no se envía pero log tiene warning
  con los detalles del alert.
- [ ] **T-e2e-3**: `TestE2E_AdminCanUpdateThreshold**` — admin PUT
  /cost-threshold/{org} → tabla actualizada → próximo job usa
  el nuevo threshold.
- [ ] **T-sabotaje**: Comentar la rama que hace `INSERT INTO
  cost_alerts_sent` (sabotaje: salta el check de dedup) → test
  e2e-1 con 2 invocaciones del job DEBE ver 2 emails enviados
  (duplicado spam) → restaurar INSERT → test verde. Documentar
  en commit body.
