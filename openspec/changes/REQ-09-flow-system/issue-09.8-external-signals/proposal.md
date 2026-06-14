# Proposal: issue-09.8-external-signals

## Intención

Permitir que sistemas externos envíen signals con payload para resumir flows en estado `paused_awaiting_signal`, sin polling. Complementa `human_input` (que es interactivo) con eventos asíncronos arbitrarios.

## Scope

**Incluye:**
- Step type `await_signal` con `signal_name` + `timeout_seconds`
- Tabla `flow_run_signals_pending` y `flow_signals_delivered`
- Endpoints POST /runs/:id/signals (per-run) y POST /flows/:slug/signals (broadcast)
- RBAC permission `run.signal`
- Persistencia de signals "early" en ventana configurable
- Timeout con retry policy

**No incluye:**
- Signal scheduling (cron usa REQ-10)
- Signal acks/retry desde la app (signal es one-shot, el caller retry)

## Enfoque técnico

1. `await_signal` step inserta row en `flow_run_signals_pending` y libera worker
2. Endpoint signal: UPDATE pending → INSERT delivered + tx + wake worker via LISTEN/NOTIFY
3. Worker pool tiene listener para wake
4. Broadcast: query múltiples runs paused matching signal_name

## Riesgos

- Signal a flow cancelled: ignorar gracefully
- Replay: dedup por signal_id opcional
- Memory: cap signals pending por org

## Testing

- await + signal → resume
- timeout aplica retry
- broadcast a N runs
- signal a flow sin pending → 409
- RBAC enforce
