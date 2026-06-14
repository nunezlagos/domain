# issue-09.8-external-signals

**Origen:** `REQ-09-flow-system`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** developer de flows con waits externos
**Quiero** que un sistema externo pueda enviar un signal con payload para resumir un flow paused
**Para** integrar callbacks, webhooks de approval, eventos asíncronos sin polling

## Criterios de aceptación

### Escenario 1: Step await_signal

```gherkin
Dado que existe step type `await_signal` con config `{signal_name: "approval_received", timeout_seconds: 86400}`
Cuando el flow llega a ese step
Entonces el flow pasa a `paused_awaiting_signal`
Y se persiste expectativa en `flow_run_signals_pending`
Y NO consume CPU/budget mientras espera
```

### Escenario 2: Signal con payload resumea

```gherkin
Dado que existe flow_run paused awaiting "approval_received"
Cuando POST /api/v1/runs/:id/signals con body `{name:"approval_received", payload:{approved:true, by:"alice"}}`
Y la auth tiene permission "run.signal"
Entonces el step await_signal completa con `output = payload`
Y el flow continúa al next step
Y audit_log "flow.signal_delivered"
```

### Escenario 3: Signal a flow no esperando

```gherkin
Dado que flow_run NO tiene step await_signal activo
Cuando llega signal
Entonces 409 "no pending signal of that name"
Y opcionalmente persistir signal en `pending_signals` por window (config) para entrega posterior
```

### Escenario 4: Timeout

```gherkin
Dado que step await_signal tiene timeout_seconds=3600
Cuando pasan 3600s sin signal
Entonces el step falla con error `SignalTimeout`
Y el flow aplica retry policy del step (issue-09.4)
```

### Escenario 5: Signal por slug del flow (broadcast)

```gherkin
Dado que múltiples runs del mismo flow esperan signal "global_pause"
Cuando POST /api/v1/flows/:slug/signals con `{name:"global_pause"}`
Entonces TODOS los runs paused esperando ese signal lo reciben
Y se logean N entregas
```

## Análisis breve

- **Qué pide:** step type await_signal + tabla pending signals + endpoint + RBAC permission
- **Esfuerzo:** S
- **Riesgos:** signals huérfanos si flow cancelled; replay attack
