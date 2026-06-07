# HU-11.3-execution-streaming

**Origen:** `REQ-11-runner-system`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** usuario ejecutando un flow o skill
**Quiero** ver los logs en vivo, el progreso de cada step y las salidas del LLM en tiempo real
**Para** diagnosticar problemas rápidamente y tener visibilidad del avance sin esperar a que termine

## Criterios de aceptación

### Escenario 1: Cliente se suscribe a live logs de un run

```gherkin
Dado un domain_flow_run con id "run_abc123" que está ejecutándose
Cuando el cliente abre una conexión WebSocket a `/ws/runs/run_abc123`
Entonces recibe eventos en tiempo real mientras el run progresa
Y el primer evento contiene el estado inicial del run
```

### Escenario 2: Eventos de step completion

```gherkin
Dado que estoy suscrito a "run_abc123" vía WebSocket
Cuando un step del flow se completa
Entonces recibo un evento:
  | type       | "step_complete"          |
  | step_id    | "step_1"                 |
  | step_name  | "generate_code"          |
  | status     | "success"                |
  | duration   | 2.345                    |
  | output     | "Generated 150 lines..." |
```

### Escenario 3: Streaming de texto LLM

```gherkin
Dado que estoy suscrito vía SSE a `/api/v1/runs/run_abc123/events`
Cuando el LLM está generando texto para un step
Entonces recibo eventos SSE con:
  | event | data                          |
  |-------|-------------------------------|
  | token | {"text":"The","step_id":"s1"} |
  | token | {"text":" answer","step_id":"s1"} |
  | token | {"text":" is","step_id":"s1"} |
  | token | {"text":" 42","step_id":"s1"} |
  | done  | {"step_id":"s1","tokens":42} |
Y los tokens se reciben en orden y sin pérdida
```

### Escenario 4: Errores en tiempo real

```gherkin
Dado que estoy suscrito a un run vía WebSocket
Cuando un step falla con error
Entonces recibo un evento:
  | type      | "step_error"       |
  | step_id   | "step_2"           |
  | error     | "Connection timeout|
  |           | to OpenAI API"     |
  | timestamp | "2026-06-07T..."  |
Y el run se marca como `failed`
```

### Escenario 5: Múltiples clientes reciben los mismos eventos

```gherkin
Dado que 3 clientes están suscritos al mismo run_id
Cuando ocurre un evento de step completion
Entonces los 3 clientes reciben el mismo evento
En el mismo orden
Sin diferencias en el contenido
```

### Escenario 6: Cliente se desconecta y reconecta

```gherkin
Dado un cliente que estaba suscrito y perdió conexión
Cuando reconecta al WebSocket con `?last_event_id=42`
Entonces recibe todos los eventos desde el id 43 en adelante
Sin duplicar eventos previos
```

### Escenario 7: SSE para progreso general

```gherkin
Dado que estoy suscrito vía SSE a `/api/v1/runs/run_abc123/progress`
Cuando recibo eventos de progreso
Entonces recibo:
  | event    | data                                        |
  |----------|---------------------------------------------|
  | progress | {"step_current":1,"step_total":5,"pct":20} |
  | progress | {"step_current":2,"step_total":5,"pct":40} |
  | progress | {"step_current":5,"step_total":5,"pct":100}|
  | complete | {"run_id":"run_abc123","status":"success"} |
```

## Análisis breve

- **Qué pide realmente:** Sistema de streaming bidireccional para ejecuciones: WebSocket para logs detallados y SSE para progreso ligero. Los clientes se suscriben por run_id y reciben eventos en tiempo real.
- **Módulos sospechados:** `internal/api/ws/`, `internal/api/sse/`, `internal/runner/stream/`, `internal/models/event.go`
- **Riesgos / dependencias:** El servidor debe manejar N conexiones concurrentes. La memoria de eventos para reconexión puede crecer. Compatibilidad con proxies reversos (SSE).
- **Esfuerzo tentativo:** M

## Verificación previa

- [ ] Revisar codebase (grep)
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
