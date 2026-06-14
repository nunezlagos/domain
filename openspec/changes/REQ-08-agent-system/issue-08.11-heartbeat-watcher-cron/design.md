# Design: issue-08.11-heartbeat-watcher-cron

## Contexto

`flow_run_steps.last_heartbeat_at TIMESTAMPTZ` ya existe (migration 000063). Hoy no hay watcher que detecte stuck. Si un MCP externo cuelga o el cliente IDE pierde contexto, el step queda `status='running'` para siempre, bloqueando el `flow_run` parent.

Sin este watcher, issue-08.10 puede dejar flows zombis en BD que ocupan slots de concurrency, leakean memoria y bloquean cleanup.

## ADR-1 — Diferenciación system cron vs user cron

Hay 2 tipos de crons en Domain:

| Tipo | Tabla | Visibilidad user | Propósito |
|---|---|---|---|
| User-defined | `crons` | ✅ visible vía API | Tareas programadas del project (`weekly-audit`, etc.) |
| System | `internal/scheduler/cron/system/*.go` (registrados al boot) | ❌ NO visible | Salud operacional interna |

Decisión: `heartbeat-watcher` es **system**, NO va en tabla `crons` (los users no deben configurarlo ni verlo). Se hardcodea en código + se enable por config flag.

## ADR-2 — Por qué `FOR UPDATE SKIP LOCKED`

Race condition posible: el cliente IDE actualiza `last_heartbeat_at` justo cuando el cron está leyendo. Con `FOR UPDATE SKIP LOCKED`:

```sql
SELECT id FROM flow_run_steps
WHERE status='running' AND last_heartbeat_at < NOW() - INTERVAL '5 minutes'
FOR UPDATE SKIP LOCKED
LIMIT 100;
```

Si el cliente tiene un lock implícito por UPDATE concurrente, el cron lo skip y procesa otros. La próxima tick, si el cliente terminó, lo vuelve a evaluar.

Trade-off: tick puede saltarse algunos stuck en cada pasada. Mitigación: tick cada 60s → en 2-3 ticks lo atrapa.

## ADR-3 — Threshold default 5min

Justificación:
- Phases típicas de orquestador (sdd-design, sdd-tasks) toman 5-60s
- sdd-apply largo puede tomar 2-3min (test runs)
- Algunos workflows pueden tomar 5min legítimamente
- Threshold de 5min cubre los casos legítimos sin atrapar false positives

Configurable via env `DOMAIN_HEARTBEAT_WATCHER_TIMEOUT_MINUTES` para escenarios especiales (CI con tests muy lentos puede subir a 10min).

## ADR-4 — Saga compensation policy

Por cada step marcado `failed`, el watcher consulta `agent_templates.metadata.retry_policy`:

| Policy | Acción del watcher |
|---|---|
| `idempotent` | Marca failed + saga event `'auto_retry_eligible'`. flow_runs no se marca failed automáticamente; espera resume. |
| `re-emit` | Marca failed + saga event `'reemit_eligible'`. Mismo: espera resume. |
| `require-cleanup` | Marca failed + saga event `'cleanup_required'`. flow_runs.status='failed' inmediato; resume requiere rollback manual previo. |

El watcher **NO auto-reanuda**. Esa decisión queda al orquestador (issue-08.10) o al humano.

## ADR-5 — Métricas

```
domain_heartbeat_watcher_stuck_total{org_id, phase, reason}
  reason = "heartbeat_timeout" | "agent_template_missing" | "saga_failed"
```

Cardinalidad acotada: org_id (asumiendo <10k orgs), phase (10 valores), reason (3 valores). Aceptable.

`org_id` cumple con regla observability.md: aceptable si <10k orgs esperados.

## ADR-6 — Cuándo NO disparar

Casos donde el cron NO debe marcar failed:
- `status != 'running'` (ya fue procesado por otra vía)
- `last_heartbeat_at IS NULL` AND `started_at IS NULL` (step nunca arrancó)
- `last_heartbeat_at IS NULL` AND `started_at > NOW() - timeout` (recién arrancó, no tuvo chance de enviar heartbeat)

Query base:
```sql
SELECT id FROM flow_run_steps
WHERE status='running'
  AND (
    last_heartbeat_at < NOW() - $1::interval
    OR (last_heartbeat_at IS NULL AND started_at < NOW() - $1::interval)
  )
FOR UPDATE SKIP LOCKED
LIMIT 100;
```

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|---|---|---|
| False positive: client IDE legítimo tarda > 5min | Media | Cliente IDE debe heartbeat cada 60s; threshold configurable |
| Race con cliente actualizando estado | Media | FOR UPDATE SKIP LOCKED |
| Leader split-brain (2 nodos creen ser leader) | Baja | Leader election existente con TTL + heartbeat de leader |
| Cron no corre por crash | Baja | Restart automático del proceso (k8s liveness probe) |
| Métrica explota cardinality si bug genera muchos reasons | Media | Lista cerrada de reasons en `internal/metrics/heartbeat.go` |

## Observabilidad

- Counter: `domain_heartbeat_watcher_stuck_total` (mencionada arriba)
- Counter: `domain_heartbeat_watcher_ticks_total{result="ok|leader_skip|error"}` — visibility del propio cron
- Log Info por tick con count de stuck detectados
- OTel span por tick (corto, pero útil para debug)

## Plan de implementación

1. Implementar Watcher struct + Tick()
2. Tests unit con mock de pool
3. Tests integration con testcontainers
4. Wire-up en `cmd/domain/main.go::runServer`
5. Métricas + alerts en `deploy/prometheus/`
