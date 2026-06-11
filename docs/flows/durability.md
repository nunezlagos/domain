# Durable Execution — Flows

> issue-09.6 — `internal/runner/flow/{claim,heartbeat,recovery,resume,durable}.go`

Los flow runs sobreviven crashes de worker: el estado se checkpointea por step
en Postgres y cualquier worker puede retomar un run huérfano desde su cursor.

## Modelo

- `flow_runs.worker_id` — worker que tiene ownership del run
- `flow_runs.last_heartbeat_at` — último latido del worker (cada 30s)
- `flow_runs.cursor` — JSONB con el step actual y posición en el DAG
- `flow_runs.recovery_count` — cuántas veces el run fue reclamado tras crash
- Migration 000076: `output_compressed BYTEA`, `output_s3_key VARCHAR(500)`

## Claim atómico (de-003)

`ClaimRunClaims.ClaimRun` toma ownership con `UPDATE ... RETURNING` sobre un
subselect `FOR UPDATE SKIP LOCKED`. Elegible:

1. Runs en `pending` (nunca ejecutados), o
2. Runs en `running` cuyo `last_heartbeat_at` superó `StaleAfter` (default 5m)

Dos workers compitiendo nunca obtienen el mismo run (`SKIP LOCKED`,
verificado por `TestRace_TwoWorkersClaim`). `IsRecovery=true` indica reclaim
de un run stale.

## Heartbeat (de-004)

Goroutine por run activo actualiza `last_heartbeat_at` cada 30s
(`internal/runner/flow/heartbeat.go`). Si el worker muere, el heartbeat se
detiene y el run queda elegible para reclaim pasado `StaleAfter`.

Métrica: `domain_flow_heartbeat_age_seconds` (gauge). Alertar si > 60s
sostenido.

## Recovery scanner (de-005)

`internal/runner/flow/recovery.go` corre cada 60s bajo advisory lock
(`LockKeyFlowRecovery`): libera `worker_id` de runs stale y detecta
crash-loops (`recovery_count` alto → el run se marca failed en lugar de
reintentar infinito).

## Resume desde cursor (de-006)

`ResumeRun` parte del cursor persistido: los steps con checkpoint completado
no se re-ejecutan; sus outputs se rehidratan desde el estado persistido.

## Replay safety (de-009)

Si un step declara `replay_safe: false` y el run es recovery, el engine NO lo
re-ejecuta automáticamente: el run pasa a `awaiting_human`. Default: replay
safe (`IsReplaySafe(nil) == true`).

## Outputs grandes (de-007)

- El output JSON se comprime gzip (`CompressOutput`, BestCompression)
- Si el blob comprimido supera 10MB (`S3SpillThreshold`), se sube a S3 y la
  fila guarda `output_s3_key`

## Idempotencia (de-008)

Cada step ejecuta con key `flow_run:<run_id>:step:<step_key>`
(`StepIDempotencyKey`). Side-effects externos deben usar esta key para
deduplicar en re-ejecuciones.

## Operación

| Síntoma | Causa probable | Acción |
|---------|----------------|--------|
| Run en `running` sin avanzar | Worker muerto | Esperar reclaim automático (≤5m + 60s scanner) |
| `recovery_count` creciendo | Crash-loop en un step | Revisar logs del step; el scanner lo marca failed |
| Run en `awaiting_human` | Step replay-unsafe tras recovery | Decidir manualmente reanudar o cancelar |
| `domain_flow_heartbeat_age_seconds` alto | Workers caídos o saturados | Revisar procesos del runner |
