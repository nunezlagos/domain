# Proposal: issue-08.12-orphan-runs-audit-cron

## Scope (in)

- System cron diario (4am UTC default) que detecta `agent_runs` orphan (sin `flow_run_id` y sin `metadata->>'standalone'`)
- Incrementa `domain_agent_runs_orphan_total{org_id, reason='bypass_service_layer'}` (counter ya registrado en issue-08.10)
- Persistencia de `last_ack_at` para idempotencia cross-tick (en tabla `system_state` o columna en `system_crons`)
- Alert `AgentRunsOrphanDetected` en `deploy/prometheus/alerts/orchestrator.yml`
- Leader election integration (sólo 1 nodo procesa)
- 6 tests E2E (escenarios del issue.md) + 1 sabotaje INSERT bypass

## Scope (out)

- Auto-borrar agent_runs orphans — el cron sólo audita, NO borra (cleanup es manual via dev process)
- Métrica nueva — la counter ya está definida en issue-08.10
- Endpoint API para listar orphans — issue futura si se necesita

## Cambios

### Schema BD

Cero migrations nuevas. Uso:
- `agent_runs.flow_run_id` (nullable existente)
- `agent_runs.metadata JSONB` (existente)
- `system_state` (o tabla equivalente para persistir `last_ack_at`) — verificar si existe, sino crear con 1 migration mínima

### Code Go

- `internal/scheduler/cron/system/orphan_runs_audit.go` (NUEVO):
  - `OrphanAuditor` struct (pool, leader, metrics, schedule, logger)
  - `Tick(ctx)` query + count + emit metrics + update last_ack_at
- `internal/state/system_state.go` (verificar si existe; sino crear): `GetLastAckAt(key)` / `SetLastAckAt(key, ts)`
- Wire-up en `cmd/domain/main.go::runServer`

### Tests

- `internal/scheduler/cron/system/orphan_runs_audit_test.go` — unit
- `tests/e2e/orphan_runs_audit_test.go` — 6 escenarios integration

### Alerts

- `deploy/prometheus/alerts/orchestrator.yml` — agregar regla:
  ```yaml
  - alert: AgentRunsOrphanDetected
    expr: increase(domain_agent_runs_orphan_total[24h]) > 0
    for: 0m
    labels: { severity: warning }
    annotations:
      summary: "Agent runs orphan detectados (bypass del service-layer)"
  ```

## Dependencias

| Issue | Estado | Por qué |
|---|---|---|
| issue-08.10 sdd-pipeline-orchestrator | proposed | Define la métrica `domain_agent_runs_orphan_total` y la flag WithStandalone |
| REQ-10 cron + leader | implementado | Scheduler base |
| issue-17.1 metrics | implementado | Counter registration |

⚠️ Dep circular con issue-08.10: la métrica se DEFINE en 08.10 pero el cron que la INCREMENTA está acá. Orden: implementar 08.12 primero (con métrica registrada acá; 08.10 la consume).

## Estado

`proposed` — bloqueante de issue-08.10 (sin esto el enforcement híbrido pierde la auditoría del bypass case).
