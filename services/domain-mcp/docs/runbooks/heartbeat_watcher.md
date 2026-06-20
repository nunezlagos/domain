# Runbook: Heartbeat Watcher

## ¿Qué es?

El Heartbeat Watcher es un system cron que detecta `flow_run_steps` stuck (status='running' + sin heartbeat por más de 5 minutos) y los marca como `failed` con razón `heartbeat_timeout`.

## ¿Cómo funciona?

1. Corre cada 60 segundos (configurable via `DOMAIN_HEARTBEAT_WATCHER_TICK_SECONDS`)
2. Consulta `flow_run_steps` con `FOR UPDATE SKIP LOCKED` para evitar race conditions
3. Marca steps como `failed` con `failure_reason = 'heartbeat_timeout'`
4. Inserta un evento en `saga_compensation_log` según la `retry_policy` del agent_template
5. Si todos los steps del flow_run están terminales, actualiza `flow_runs.status = 'failed'`

## Threshold

- Default: 5 minutos (`DOMAIN_HEARTBEAT_WATCHER_TIMEOUT_MINUTES`)
- NO marca steps con `last_heartbeat_at IS NULL AND started_at IS NULL` (nunca arrancaron)
- NO marca steps con `last_heartbeat_at IS NULL AND started_at < NOW() - timeout` (recién arrancados)

## Saga Compensation por retry_policy

| Policy | Saga Event | flow_runs afectado |
|--------|-----------|-------------------|
| `idempotent` | `auto_retry_eligible` | NO marca failed |
| `re-emit` | `reemit_eligible` | NO marca failed |
| `require-cleanup` | `cleanup_required` | Marca failed inmediato |

## Métricas

- `domain_heartbeat_watcher_stuck_total{org_id, phase, reason}` — steps detectados
- `domain_heartbeat_watcher_ticks_total{result}` — ticks del cron (ok|leader_skip|error)

## Cómo investigar stuck flows

1. Verificar métrica `domain_heartbeat_watcher_stuck_total`
2. Consultar `flow_run_steps WHERE failure_reason = 'heartbeat_timeout'`
3. Revisar `saga_compensation_log` para el flow_run_id
4. Decidir si re-ejecutar o investigar causa raíz del stuck
