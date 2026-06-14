# HU-28.8-timeafter-timertimer

**Origen:** `REQ-28-arquitectura-interna`
**Prioridad tentativa:** baja
**Tipo:** refactor

## Historia de usuario

**Como** operador de Domain
**Quiero** que los loops de retry usen `time.NewTimer` con `defer timer.Stop()` en vez de `time.After`
**Para** que los timers no se leak en memoria bajo alta carga de retrys

## Contexto

`time.After(d)` crea un `time.Timer` que no se libera hasta que el timer dispara. En loops de retry (backoff), si hay muchas operaciones concurrentes fallando, cada iteración del loop crea un timer que no se puede garbage collectear hasta que expire. Con `time.NewTimer` + `defer timer.Stop()`, el timer se libera inmediatamente al salir del loop, incluso si el context se cancela antes de que expire.

Afecta:
- `internal/llm/retry/retry.go:80` — retry loop de LLM providers
- `internal/llm/retry/retry.go:107` — segundo backoff
- `internal/mcp/server/resilience.go:200` — MCP resilience retry

## Criterios de aceptación

### Escenario 1: timer.Stop() en retry loop

```gherkin
Dado un retry loop con backoff
Cuando el context se cancela durante el backoff
Entonces `timer.Stop()` se llama (via defer)
Y el timer se libera inmediatamente
```

### Escenario 2: Misma semántica de timing

```gherkin
Dado un retry loop con backoff de 1s
Cuando uso time.NewTimer + timer.Stop() diferido
Entonces el comportamiento de espera es idéntico a time.After(1s)
```

## Análisis breve

- **Qué pide:** Reemplazar `time.After(d)` por `time.NewTimer(d)` + `defer timer.Stop()` + `<-timer.C`
- **Módulos afectados:** `internal/llm/retry/retry.go`, `internal/mcp/server/resilience.go`
- **Esfuerzo tentativo:** XS (2 horas)
- **Dependencias:** Ninguna
