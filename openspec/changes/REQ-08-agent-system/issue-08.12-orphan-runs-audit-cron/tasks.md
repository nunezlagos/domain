# Tasks: issue-08.12-orphan-runs-audit-cron

## Schema

- [ ] **mig-001**: Verificar si tabla `system_state` existe (grep migrations). Si NO existe, crear migration 000074_create_system_state con CREATE TABLE IF NOT EXISTS system_state(key VARCHAR(100) PK, value JSONB, updated_at TIMESTAMPTZ)
- [ ] **mig-002**: down.sql correspondiente

## Code

- [ ] **svc-001**: `internal/scheduler/cron/system/orphan_runs_audit.go` (NUEVO):
  - `OrphanAuditor` struct (pool, leader, metrics, schedule, logger)
  - `func (a *OrphanAuditor) Tick(ctx context.Context) error`
  - `func (a *OrphanAuditor) Start(ctx context.Context)` — daily loop
- [ ] **svc-002**: Query SQL con WHERE flow_run_id IS NULL + standalone NULL + created_at > last_ack_at
- [ ] **svc-003**: `internal/state/system_state.go` (NUEVO si no existe):
  - GetValue(ctx, key string) (jsonb, error)
  - SetValue(ctx, key string, value jsonb) error
  - UPSERT atómico
- [ ] **svc-004**: Define la métrica `domain_agent_runs_orphan_total` en `internal/metrics/agent.go` (issue-08.10 la consume)
- [ ] **svc-005**: Define `domain_orphan_audit_ticks_total{result}` en `internal/metrics/orphan_audit.go`
- [ ] **svc-006**: Wire-up en `cmd/domain/main.go::runServer`

## Config

- [ ] **cfg-001**: `internal/config/config.go` — agregar `OrphanAuditEnabled bool` (default true), `OrphanAuditSchedule string` (default "0 4 * * *")
- [ ] **cfg-002**: Env bind `DOMAIN_ORPHAN_AUDIT_ENABLED`, `DOMAIN_ORPHAN_AUDIT_SCHEDULE`

## Alerts

- [ ] **alert-001**: `deploy/prometheus/alerts/orchestrator.yml` — agregar AgentRunsOrphanDetected con severity=warning

## Tests

- [ ] **test-001**: unit `orphan_runs_audit_test.go` con mock pool
- [ ] **test-002**: integration escenario 1 (detección bypass)
- [ ] **test-003**: integration escenario 2 (standalone=true NO counta)
- [ ] **test-004**: integration escenario 3 (idempotencia last_ack_at)
- [ ] **test-005**: integration escenario 4 (alert dispara) — verificar regla Prometheus parsea
- [ ] **test-006**: integration escenario 5 (leader election HA)
- [ ] **sab-001**: SABOTAJE — test inserta INSERT INTO agent_runs (flow_run_id=NULL, metadata='{}', ...) → cron lo cuenta dentro 24h

## Docs

- [ ] **doc-001**: `docs/runbooks/orphan_runs.md` — cómo investigar + manejar bypass detectados
- [ ] **doc-002**: CHANGELOG.md Unreleased

## Estado

- [ ] **state-001**: state.yaml → implemented post-merge
- [ ] **state-002**: Remove de blocked_by en state.yaml de issue-08.10
