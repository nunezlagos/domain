# Proposal: issue-08.11-heartbeat-watcher-cron

## Scope (in)

- System cron `heartbeat-watcher` que corre cada 60s
- Detecta `flow_run_steps` con `status='running' AND last_heartbeat_at < NOW() - timeout` (default 5min)
- Marca como `failed` con `failure_reason='heartbeat_timeout'`
- Dispara `saga_compensation_log` según `retry_policy` del template
- Actualiza `flow_runs.status` a `failed` si todos sus steps están terminados
- Métrica `domain_heartbeat_watcher_stuck_total{org_id, phase, reason}`
- Config: `domain.heartbeat_watcher.timeout_minutes` (default 5)
- Integration con `internal/scheduler/leader/` (sólo el leader ejecuta)
- Lock concurrente: `FOR UPDATE SKIP LOCKED` para race-safety
- 6 tests E2E (escenarios del issue.md) + 1 sabotaje race condition

## Scope (out)

- Schema BD nuevo — `last_heartbeat_at`, `saga_compensation_log`, `flow_run_steps.status` ya existen
- Heartbeat client-side — se asume que el cliente IDE actualiza `last_heartbeat_at` durante long-running phases (responsabilidad del orquestador en issue-08.10)
- Reanudación automática de flows fallados — espera flow_run.resume() explícito
- Alertas AlertManager — esas vienen en issue-17.1

## Cambios

### Schema BD

Cero migrations.

### Code Go

- `internal/scheduler/cron/system/heartbeat_watcher.go` (NUEVO):
  - `Watcher` struct con dependencies (pool, leader, metrics, timeout config)
  - `Tick(ctx)` ejecuta detección + UPDATE batch + emit metrics
  - Integration en `cmd/domain/main.go::runServer` para registrar al startup
- `internal/metrics/heartbeat.go` (NUEVO):
  - `HeartbeatWatcherStuckTotal` counter (org_id, phase, reason)
- `internal/config/config.go`:
  - Agregar `HeartbeatWatcherTimeoutMinutes int` (default 5)

### Tests

- `internal/scheduler/cron/system/heartbeat_watcher_test.go` — unit tests con mocks
- `tests/e2e/heartbeat_watcher_test.go` — 6 escenarios integration

## Dependencias

| Issue | Estado | Por qué |
|---|---|---|
| REQ-09 flows + flow_run_steps | implementado | Tabla + last_heartbeat_at column existe |
| REQ-10 cron + leader election | implementado | Scheduler base + leader pattern |
| issue-17.1 metrics-prometheus | implementado | Counter registration |

## Estado

`proposed` — listo para implementar. Es **bloqueante** de issue-08.10 (sin esto, los flows pueden quedar zombis).
