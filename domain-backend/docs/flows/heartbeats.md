# Step Heartbeats — Flows

> issue-09.10 — `internal/service/flow/heartbeats.go` + `internal/runner/flow/stepheartbeat.go`

Steps long-running (minutos a horas) reportan progreso parcial para que el
watchdog no los marque zombies y las UIs puedan mostrar avance.

## Desde un step

El runner inyecta un heartbeater en el context de cada step:

```go
hb := flowrunner.HeartbeaterFrom(ctx) // nunca nil (no-op si no hay)
_ = hb.Beat(ctx, 0.3, "downloaded 30%")
```

En el engine de registry (steptypes), el hook es `RunInput.Heartbeat`.

Efectos de un beat:

- `flow_run_steps.last_heartbeat_at = NOW()`
- `progress = 0.30`, `progress_message = "downloaded 30%"`
- `pg_notify(flow_step_progress, {flow_run_id, step_key, progress, message})`
  para el SSE stream (consumidor: endpoint issue-09.3)

## Throttle (hb-002)

Máximo un beat efectivo cada 5s (`DefaultHeartbeatMinInterval`); los beats
dentro de la ventana son no-op. Anti-flood verificado por
`TestSabotage_Heartbeat_FloodThrottled` (100 beats en 10s → 2 efectivos).

## Lifecycle de fila

El loop del runner crea la fila `flow_run_steps` en `running` al iniciar el
step (`beginStepRow`) — visible para el watchdog y para `GET /runs/:id` — y
la cierra en `completed`/`failed` con outputs comprimidos (`completeStepRow`).

## Zombie detection

`HeartbeatStore.FindStuck` (+ `FindStuckWithCustomThreshold` por step type)
detecta steps `running` sin heartbeat reciente; el Watchdog cron los marca
failed con `ErrHeartbeatMissed` y aplica la retry policy (issue-09.4).
Steps cortos quedan exentos hasta su `timeout_seconds` (escenario 3).

## Defaults de threshold

`flow_run_steps.heartbeat_threshold_seconds` (default 120). Steps que
declaran thresholds custom usan `FindStuckWithCustomThreshold`.
