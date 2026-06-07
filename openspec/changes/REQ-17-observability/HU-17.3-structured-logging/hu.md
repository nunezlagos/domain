# HU-17.3-structured-logging

**Origen:** `REQ-17-observability`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** operador
**Quiero** logs estructurados JSON con campos consistentes y correlación trace_id/request_id
**Para** poder agregar, filtrar y correlacionar con tracing en cualquier stack (Loki, Datadog, ELK)

## Criterios de aceptación

### Escenario 1: Logs en formato JSON estable

```gherkin
Dado que `DOMAIN_LOG_FORMAT=json` y `DOMAIN_LOG_LEVEL=info`
Cuando el servidor logea un evento
Entonces el output es una línea JSON con campos mínimos:
  | campo       | tipo    | requerido |
  | time        | string  | sí        |
  | level       | string  | sí        |
  | msg         | string  | sí        |
  | trace_id    | string  | si hay span activo |
  | span_id     | string  | si hay span activo |
  | request_id  | string  | si hay request HTTP |
  | user_id     | string  | si hay auth context |
  | project_id  | string  | si aplica |
  | error       | string  | si level >= error |
```

### Escenario 2: Niveles configurables y dynamic update

```gherkin
Dado que `DOMAIN_LOG_LEVEL=info`
Cuando POST /admin/log-level con body `{"level":"debug"}` con role admin
Entonces los siguientes logs se emiten con level debug
Y se persiste un audit_log del cambio
```

### Escenario 3: Texto legible en dev

```gherkin
Dado que `DOMAIN_LOG_FORMAT=text` (default en dev)
Cuando se emite un log
Entonces el output es texto coloreado human-friendly con `time level msg key=value`
```

### Escenario 4: Sin PII en mensajes

```gherkin
Dado que el sistema procesa payloads con email/tokens
Cuando se logea cualquier evento
Entonces los campos `content`, `email`, `api_key`, `password`, `secret` NO aparecen en el JSON
Y un linter en CI valida que ningún `slog.String("password"|"email"|...)` esté en el código
```

## Análisis breve

- **Qué pide:** slog con handler JSON y text, campos contextuales via `slog.With()`, integración con OTel para trace_id
- **Módulos sospechados:** `internal/observability/logging/`, middleware HTTP que inyecta request_id
- **Riesgos:** PII leak, log spam si level mal configurado
- **Esfuerzo tentativo:** S
