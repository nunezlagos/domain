# HU-09.6-durable-execution

**Origen:** `REQ-09-flow-system`
**Persona:** dx-engineer, integrator
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** operador
**Quiero** que un flow que estaba corriendo cuando el server cayó se resume automáticamente al volver el server
**Para** ejecutar flows largos (minutos/horas) sin perder progreso por restart o crash

## Modelo

- Cada `flow_runs` tiene checkpoints por step
- Step `completed` queda persistido con su output
- Al arrancar el server: scanner busca flows en estado `running` cuyo `last_heartbeat_at < now() - 60s` → resume desde último step completado
- Steps `code_exec` y `llm_call` deben ser **idempotentes** o usar idempotency keys (HU-13.4)
- Steps con side-effects externos NO-idempotentes deben declarar `replay_safe: false` → manual intervention

## Criterios de aceptación

### Escenario 1: Checkpoint por step

```gherkin
Dado que un flow tiene steps [A, B, C, D]
Cuando A y B completan
Entonces `flow_runs.cursor = {step:"B", status:"completed", output:{...}}`
Y `flow_run_steps` tiene 2 filas (A, B) con outputs persistidos
```

### Escenario 2: Resume tras restart

```gherkin
Dado que flow está running en step C cuando server cae
Y al volver el server, hay otro instance corriendo
Cuando scanner detecta `last_heartbeat_at < now() - 60s` y `status = running`
Entonces marca el flow_run como `recoverable`
Y lo asigna al worker pool del nuevo instance
Y reanuda desde step C (re-ejecuta C, no salta a D)
Y se logea audit "flow_run.resumed_after_crash"
```

### Escenario 3: Heartbeat continuo

```gherkin
Dado que un flow está ejecutando step largo
Cuando pasa 30s
Entonces el worker actualiza `flow_runs.last_heartbeat_at = now()`
Y se publica métrica `domain_flow_heartbeat_age_seconds`
```

### Escenario 4: Replay-unsafe step

```gherkin
Dado que step "send_email" tiene `replay_safe: false`
Cuando el flow se resumea después de crash en ese step
Entonces el step NO se re-ejecuta automáticamente
Y status del flow queda `paused_awaiting_human`
Y se notifica admin con CTA "ejecutar manualmente o skip"
```

### Escenario 5: Idempotency key auto-generada

```gherkin
Dado que step "create_resource" en flow_run X corre por primera vez
Cuando se invoca endpoint downstream con idempotency_key
Entonces se usa `flow_run_id + step_id` como idempotency key estable
Y si crash y resume → segunda ejecución con misma key → downstream devuelve mismo result
```

### Escenario 6: Output persistido binario

```gherkin
Dado que step completa con output 5MB
Cuando se checkpointea
Entonces output se almacena comprimido (gzip) en `flow_run_steps.output_compressed BYTEA`
Y si > 10MB se sube a S3 con ref en `output_s3_key`
```

## Análisis breve

- **Qué pide:** checkpoints + heartbeat + scanner + replay_safe flag + idempotency hints + output overflow S3
- **Esfuerzo:** L
- **Riesgos:** lost updates en crash mid-checkpoint; dos workers procesando el mismo run; replay con side-effects
