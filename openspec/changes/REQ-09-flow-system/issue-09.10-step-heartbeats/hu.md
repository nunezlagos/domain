# issue-09.10-step-heartbeats

**Origen:** `REQ-09-flow-system`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** developer de steps que tardan mucho (minutos a horas)
**Quiero** emitir heartbeats con progreso parcial desde el step
**Para** que el scheduler no marque el step como zombie y para mostrar progreso al user

## Criterios de aceptación

### Escenario 1: Step emite heartbeat con progreso

```gherkin
Dado que step code_exec corre con context que expone `ctx.Heartbeat(progress, message)`
Cuando el step llama `ctx.Heartbeat(0.3, "downloaded 30%")`
Entonces se actualiza `flow_run_steps.last_heartbeat_at = NOW()`
Y `flow_run_steps.progress = 0.30`
Y `flow_run_steps.progress_message = "downloaded 30%"`
Y se publica evento via SSE para UIs subscritas
```

### Escenario 2: Step sin heartbeat por threshold

```gherkin
Dado que step tiene `heartbeat_threshold_seconds: 120`
Cuando pasan 120s sin heartbeat
Entonces el step se considera zombie
Y el scheduler lo marca failed con error `HeartbeatMissed`
Y aplica retry policy
```

### Escenario 3: Steps cortos exempt

```gherkin
Dado que step tipo `llm_call` o `skill_call` típicamente <30s
Cuando no emite heartbeat
Entonces NO es marcado zombie hasta `step.timeout_seconds`
Y heartbeat es opcional (solo para steps >timeout default)
```

### Escenario 4: Progress visible en API

```gherkin
Dado que step está corriendo con progress=0.5
Cuando GET /api/v1/runs/:id
Entonces response incluye `steps[].progress` y `steps[].progress_message`
Y SSE stream emite update cada heartbeat
```

## Análisis breve

- **Qué pide:** ctx.Heartbeat API + scheduler zombie detection + SSE progress events
- **Esfuerzo:** S
- **Riesgos:** flood heartbeats; UI render lag
