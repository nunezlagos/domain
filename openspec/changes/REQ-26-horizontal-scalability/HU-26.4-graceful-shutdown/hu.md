# HU-26.4-graceful-shutdown

**Origen:** `REQ-26-horizontal-scalability`
**Persona:** platform-engineer
**Prioridad tentativa:** alta
**Tipo:** hardening

## Historia de usuario

**Como** operador
**Quiero** que SIGTERM (rolling deploy, HPA scale-down) drene gracefully HTTP + workers + pool en window configurable
**Para** no perder requests in-flight ni romper transacciones

## Criterios de aceptación

### Escenario 1: SIGTERM signal handling

```gherkin
Dado que pod recibe SIGTERM
Cuando se procesa
Entonces:
  1. Readiness probe pasa a unhealthy (ELB deja de rutear nuevos requests)
  2. Wait 5s grace para ELB drain
  3. HTTP server `Shutdown(ctx)` con timeout 25s — espera requests in-flight
  4. Workers reciben ctx.Done() y terminan iteration actual
  5. DB pool close (drena conns gracefully)
  6. Process exit 0
Y total window <30s (K8s default)
```

### Escenario 2: Worker mid-iteration

```gherkin
Dado que worker procesa flow_run step
Cuando llega SIGTERM
Entonces:
  - termina el step actual con timeout 20s
  - actualiza checkpoint (HU-09.6) si no termina a tiempo
  - libera DB row con worker_id=NULL para recovery scanner
  - exit gracefully
```

### Escenario 3: Read replica conn drain

```gherkin
Dado que pool replica tiene 10 conns activas
Cuando shutdown
Entonces Close() espera grace 5s para queries en curso
Y después fuerza close
Y log warn si forced
```

### Escenario 4: Timeout total respetado

```gherkin
Dado que K8s `terminationGracePeriodSeconds=30`
Cuando shutdown excede 30s
Entonces K8s SIGKILL (no SIGTERM)
Y el código de shutdown nuestro tiene budget 28s con buffer 2s
```

### Escenario 5: Health endpoints during shutdown

```gherkin
Dado que shutdown started
Cuando llega `/health/ready`
Entonces 503 Service Unavailable (no listo para nuevos)
Cuando llega `/health` liveness
Entonces 200 hasta que process exit (sigue vivo, drenando)
```

### Escenario 6: Métricas shutdown

```gherkin
Dado que termina shutdown
Cuando inspecciono métricas finales
Entonces se publican:
  - `domain_shutdown_duration_seconds` histogram
  - `domain_shutdown_in_flight_at_start{kind}` counter (http, workers, queries)
  - `domain_shutdown_forced_total{reason}` si timeout
```

## Análisis breve

- **Qué pide:** SIGTERM handler + readiness flip + sequenced drain + métricas + tests
- **Esfuerzo:** S
- **Riesgos:** órdenes incorrectos rompen drain; tests difíciles
