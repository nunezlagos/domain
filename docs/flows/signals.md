# External Signals — Flows

> issue-09.8 — `internal/service/flow/signals.go` + step `wait_signal`

Permite que un sistema externo resuma un flow pausado enviando una señal con
payload (approvals, callbacks, eventos asíncronos) sin polling.

## Step `wait_signal`

```json
{
  "id": "wait_approval",
  "type": "wait_signal",
  "config": {
    "signal_name": "approval_received",
    "timeout_seconds": 86400
  }
}
```

Comportamiento:

1. Al llegar al step, el run registra expectativa en
   `flow_run_signals_pending` y pasa a `paused_awaiting_signal`.
2. Espera sin CPU vía `LISTEN flow_signals` (pg NOTIFY); fallback a polling
   500ms si la conexión dedicada falla.
3. Al recibir la señal, el run vuelve a `running` y el output del step es
   `{"signal": <name>, "payload": <payload>, "delivered_at": ...}`.
4. Timeout sin señal → error `signal timeout`, integrado con la retry policy
   del step (issue-09.4): el timeout es transient y reintenta si
   `retries > 0`.

## Entregar una señal

```
POST /api/v1/runs/{run_id}/signals
{"name": "approval_received", "step_key": "wait_approval", "payload": {"approved": true, "by": "alice"}}
```

- `202 Accepted` — señal persistida y NOTIFY emitido
- `404` — run inexistente o de otra org (anti-enumeration)
- `409 no_pending_signal` — el run no espera esa señal (o la expectativa expiró)
- `422` — falta `name`

Audit log: `flow.signal_delivered`.

## Broadcast

`SignalStore.BroadcastSignal(name, payload)` entrega a TODOS los runs con
expectativa pendiente de ese nombre (ej. `global_pause`). Retorna la cantidad
de runs notificados.

## Semántica de entrega

- `Consume` es atómico (`FOR UPDATE SKIP LOCKED`) — una señal se entrega a
  un solo waiter y se marca `delivered_at`.
- Early signals: si la señal llega antes de que el step ejecute, queda
  buffereada en `flow_signals` y se consume al instante cuando el step inicia.
- Idempotencia del lado del emisor: re-enviar la misma señal crea otra fila;
  el step consume la más antigua no-delivered.

## Tests

`internal/runner/flow/signal_integration_test.go`: delivered resumes (E2E),
timeout, broadcast 2 runs, early signal, expectativa expirada (base del 409).
