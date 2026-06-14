# Tasks: issue-08.11-heartbeat-watcher-cron

## Schema

- [x] **mig-000**: NO migration nueva — `flow_run_steps.last_heartbeat_at` ya existe (000063)

## Config

- [x] **cfg-001**: `internal/config/config.go` — agregar `HeartbeatWatcherTimeoutMinutes int` (default 5), `HeartbeatWatcherEnabled bool` (default true)
- [x] **cfg-002**: `internal/config/env.go` — bind `DOMAIN_HEARTBEAT_WATCHER_TIMEOUT_MINUTES` y `DOMAIN_HEARTBEAT_WATCHER_ENABLED`

## Code

- [x] **svc-001**: `internal/scheduler/cron/system/heartbeat_watcher.go` (NUEVO):
  - `Watcher` struct (pool, leader, metrics, timeoutMin, logger)
  - `func (w *Watcher) Tick(ctx context.Context) error`
  - `func (w *Watcher) Start(ctx context.Context)` — loop cada 60s respetando leader
- [x] **svc-002**: Query SQL con FOR UPDATE SKIP LOCKED + JOIN agent_templates para retry_policy
- [x] **svc-003**: Acción por step:
  - UPDATE flow_run_steps SET status='failed', failure_reason='heartbeat_timeout'
  - INSERT saga_compensation_log según retry_policy
  - Si todos los steps del flow_run terminales: UPDATE flow_runs SET status='failed'
- [x] **svc-004**: Wire-up en `cmd/domain/main.go::runServer` — startup hook que llama `watcher.Start(ctx)` si Enabled

## Métricas

- [x] **obs-001**: `internal/metrics/heartbeat.go` (NUEVO):
  - `HeartbeatWatcherStuckTotal` counter (labels: org_id, phase, reason)
  - `HeartbeatWatcherTicksTotal` counter (labels: result)
- [x] **obs-002**: Registrar en `internal/metrics/registry.go`

## Tests

- [x] **test-001**: unit `heartbeat_watcher_test.go` — mock pool + leader + verify SQL ejecutado
- [x] **test-002**: integration `tests/e2e/heartbeat_watcher_test.go` escenario 1 (mark failed)
- [x] **test-003**: integration escenario 2 (heartbeat reciente no se toca)
- [x] **test-004**: integration escenario 3 (threshold configurable)
- [x] **test-005**: integration escenario 4 (retry-policy require-cleanup dispara saga)
- [x] **test-006**: integration escenario 5 (leader election — solo 1 nodo)
- [x] **test-007**: integration escenario 6 (race con FOR UPDATE SKIP LOCKED)
- [x] **sab-001**: SABOTAJE — desactivar `last_heartbeat_at` update en cliente IDE → cron debe atrapar steps stuck

## Docs

- [x] **doc-001**: `docs/runbooks/heartbeat_watcher.md` — qué pasa, cómo investigar stuck flows
- [x] **doc-002**: CHANGELOG.md Unreleased

## Estado

- [x] **state-001**: state.yaml → implemented post-merge
- [x] **state-002**: Mover de blocked_by del state.yaml de issue-08.10
