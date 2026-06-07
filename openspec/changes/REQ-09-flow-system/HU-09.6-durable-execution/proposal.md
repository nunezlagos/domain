# Proposal: HU-09.6-durable-execution

## Intención

Hacer la ejecución de flows resilient a crashes/restarts mediante checkpointing por step, heartbeat continuo, scanner de recovery y manejo explícito de steps replay-unsafe.

## Scope

**Incluye:**
- Tabla `flow_run_steps` con output checkpointeado (compressed o S3)
- Columnas `flow_runs.last_heartbeat_at`, `worker_id`, `cursor JSONB`
- Worker heartbeat cada 30s
- Recovery scanner cada 60s busca runs huérfanos
- Flag `step.replay_safe BOOLEAN`
- Idempotency key auto = `flow_run_id:step_id` propagado a HU-13.4
- Output overflow a S3 con ref

**No incluye:**
- Cross-region failover (single region)
- Replay manual desde UI (futuro)

## Enfoque técnico

1. Worker claim run con `UPDATE ... SET worker_id = $me WHERE worker_id IS NULL AND status = pending RETURNING *`
2. Heartbeat goroutine cada 30s actualiza `last_heartbeat_at`
3. Scanner: `UPDATE ... SET worker_id = NULL WHERE last_heartbeat_at < now() - 60s AND status = running` luego workers regular polling
4. Checkpoint per step en tx + commit antes de llamar al next step
5. S3 spillover si output > 10MB

## Riesgos

- Dos workers procesando mismo run: UPDATE ... WHERE worker_id IS NULL es atómico, evita
- Replay con side-effect duplicado: idempotency key + replay_safe flag
- S3 timeout en spillover: retry con backoff + circuit breaker
- Checkpoint truncado en crash: tx all-or-nothing

## Testing

- Crash mid-step → recovery resumea
- Heartbeat actualiza
- Scanner detecta huérfano y reasigna
- Replay-unsafe pausa
- Idempotency key consistente entre re-runs
- Output > 10MB → S3 spillover
- Sabotaje: race 2 workers → solo 1 procesa
