# issue-17.2-tracing-otel

**Origen:** `REQ-17-observability`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** operador y desarrollador
**Quiero** tracing distribuido OpenTelemetry exportable vía OTLP
**Para** debuggear latencias y errores cruzando MCP/HTTP/CLI con vista unificada en Jaeger/Tempo/Honeycomb

## Criterios de aceptación

### Escenario 1: Spans HTTP y DB se exportan

```gherkin
Dado que `DOMAIN_OTEL_ENABLED=true` y `DOMAIN_OTEL_EXPORTER_OTLP_ENDPOINT=http://jaeger:4317`
Cuando llega un GET /api/v1/observations
Entonces se crea un span raíz `HTTP GET /api/v1/observations` con duración y status
Y se crean spans hijos por query pgx (`SELECT observations...`)
Y trace_id y span_id propagan via header W3C traceparent
```

### Escenario 2: Context propagation entre MCP y HTTP

```gherkin
Dado que un cliente MCP invoca `domain_mem_save` (vía stdio)
Y el handler llama internamente al HTTP API
Cuando inspecciono el trace
Entonces el span MCP es padre del span HTTP
Y comparten el mismo trace_id
```

### Escenario 3: Sampling configurable

```gherkin
Dado que `DOMAIN_OTEL_SAMPLE_RATIO=0.1`
Cuando hago 1000 requests
Entonces aproximadamente 100 traces se exportan (10%)
Y los errores siempre se samplean al 100% (tail-based-like fallback con ParentBased + AlwaysOnError)
```

## Análisis breve

- **Qué pide:** OTel SDK Go + OTLP exporter + instrumentación HTTP/pgx/MCP
- **Módulos sospechados:** `internal/observability/tracing/`, hooks en HTTP, pgx, mcp
- **Riesgos:** Overhead, exporter caído bloqueando, leaks de PII en attributes
- **Esfuerzo tentativo:** M
