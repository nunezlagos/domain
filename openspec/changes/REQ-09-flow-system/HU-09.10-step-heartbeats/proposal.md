# Proposal: HU-09.10-step-heartbeats

## Intención

API `ctx.Heartbeat(progress, message)` para steps largos, scheduler que detecta zombies y emisión SSE de progreso a UIs.

## Scope

**Incluye:**
- `ExecContext.Heartbeat(progress float, message string) error`
- Columnas `flow_run_steps.last_heartbeat_at, progress, progress_message`
- Scheduler zombie detector cada 30s
- SSE events `step.progress` (HU-11.3 channel reuse)
- Throttle heartbeat min 5s entre emisiones (batch in-memory)

**No incluye:**
- Heartbeats cross-process (only in-process; HU-09.6 handles crash recovery)

## Enfoque técnico

1. Heartbeat throttled in-memory antes de write DB (max 1/5s)
2. Zombie detector: `UPDATE WHERE last_heartbeat_at < now() - threshold AND status = running`
3. SSE publish via Postgres NOTIFY a frontend listeners

## Riesgos

- Flood writes: throttle estricto
- UI lag: batched updates client side

## Testing

- Heartbeat actualiza progress
- Throttle 5s respetado
- Zombie tras threshold → failed + retry
- Short steps (skill_call) exempt
- SSE event emitido
