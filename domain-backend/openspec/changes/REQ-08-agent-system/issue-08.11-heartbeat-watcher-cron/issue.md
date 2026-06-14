# issue-08.11-heartbeat-watcher-cron

**Origen:** `REQ-08-agent-system`
**Prioridad tentativa:** alta
**Tipo:** infrastructure
**Bloquea a:** `issue-08.10-sdd-pipeline-orchestrator`
**RFC:** `docs/rfc/0006-sdd-pipeline-orchestrator.md` (sección "Dependencias bloqueantes")

## Tarea técnica

**Como** plataforma Domain
**Quiero** un system cron que detecte `flow_run_steps` stuck (status='running' + last_heartbeat_at > 5min) y los marque como failed disparando saga compensation
**Para** que un MCP externo colgado o un cliente IDE que perdió contexto no deje flow_runs zombis en BD para siempre

## Modelo

- **System cron** (NO user-defined): vive en `internal/scheduler/cron/system/heartbeat_watcher.go`
- **Tick:** cada 60s, con leader election (consume `internal/scheduler/leader/`)
- **Detección:** `flow_run_steps` con `status='running'` AND `last_heartbeat_at < NOW() - INTERVAL '5min'`
- **Acción:** marca status='failed' con razón `'heartbeat_timeout'`, dispara saga_compensation_log según retry_policy del template
- **Métrica:** `domain_heartbeat_watcher_stuck_total{org_id, phase, reason}`
- **Cero schema nuevo** — `flow_run_steps.last_heartbeat_at` ya existe (migration 000063)

## Criterios de aceptación

### Escenario 1: Detección y mark-as-failed

```gherkin
Dado que existe un flow_run_step con status='running', last_heartbeat_at = NOW() - 6min
Cuando el cron heartbeat-watcher corre su tick
Entonces actualiza el step a status='failed', failure_reason='heartbeat_timeout'
Y persiste un row en saga_compensation_log con event='heartbeat_timeout_detected'
Y la métrica domain_heartbeat_watcher_stuck_total se incrementa
Y el flow_runs.status se actualiza a 'failed' si todos los steps están terminados
```

### Escenario 2: Heartbeat reciente NO se afecta

```gherkin
Dado que existe un flow_run_step con status='running', last_heartbeat_at = NOW() - 2min
Cuando el cron heartbeat-watcher corre su tick
Entonces el step NO se modifica
Y la métrica NO incrementa
```

### Escenario 3: Threshold configurable

```gherkin
Dado que la config domain.heartbeat_watcher.timeout_minutes = 10
Cuando un step tiene last_heartbeat_at = NOW() - 7min
Entonces el cron NO lo detecta como stuck (timeout = 10min)
```

### Escenario 4: Retry-policy require-cleanup dispara saga

```gherkin
Dado que un sdd-apply step (retry_policy='require-cleanup') queda stuck
Cuando heartbeat-watcher detecta y marca failed
Entonces saga_compensation_log registra event='cleanup_required'
Y el sistema NO auto-reanuda; espera flow_run.resume() explícito
```

### Escenario 5: Leader election — sólo 1 nodo procesa

```gherkin
Dado que 3 instancias de Domain server están corriendo (HA)
Cuando llega el tick del cron
Entonces SÓLO el leader actual ejecuta el UPDATE batch
Y los otros 2 NO ejecutan (leader election via internal/scheduler/leader/)
Y NO hay double-mark del mismo step
```

### Escenario 6: Sabotage — query sin lock concurrente

```gherkin
Dado que 2 clientes simultáneos heartbeat el mismo step (race condition)
Cuando heartbeat-watcher corre justo en el medio
Entonces el UPDATE usa FOR UPDATE SKIP LOCKED para evitar conflict
Y el step ganador conserva su status='running'
Y el cron NO lo marca failed por error
```

## Análisis breve

- **Qué pide:** cron service + watch + mark + saga trigger + leader election integration
- **Módulos:** `internal/scheduler/cron/system/heartbeat_watcher.go` (nuevo), `internal/scheduler/leader` (consume), `internal/metrics/heartbeat.go` (nuevo)
- **Esfuerzo:** S (1-2h con tests)
- **Riesgos:** false positives si el cliente IDE no envía heartbeat suficientemente seguido — mitigar con threshold de 5min default + heartbeat client-side cada 60s
- **Cero schema BD:** todo ya existe (last_heartbeat_at, saga_compensation_log)
