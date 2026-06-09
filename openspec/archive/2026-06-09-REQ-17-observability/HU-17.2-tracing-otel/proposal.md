# Proposal: HU-17.2-tracing-otel

## Intención

Tracing distribuido OpenTelemetry exportable vía OTLP gRPC/HTTP, con context propagation W3C entre todos los entrypoints (HTTP, MCP stdio, CLI commands, cron jobs).

## Scope

**Incluye:**
- SDK OTel Go (`go.opentelemetry.io/otel`) + exporter OTLP gRPC
- Instrumentación HTTP server (`otelhttp`) y client (outbound LLM calls)
- Instrumentación pgx v5 (`github.com/exaring/otelpgx`)
- Propagación W3C traceparent en MCP handlers
- Sampler `ParentBased(TraceIDRatioBased)` con ratio configurable y always-on para errores
- Resource attributes: service.name, service.version, deployment.environment

**No incluye:**
- Logs/metrics OTel (cubierto por HU-17.1 y HU-17.3 con backends nativos)
- Backend de tracing (Jaeger/Tempo) — infraestructura externa

## Enfoque técnico

1. Setup `tracerProvider` en `cmd/domain-mcp/main.go` con shutdown via context
2. Middleware `otelhttp.NewHandler(mux, "domain-http")` envolviendo router
3. Tracer pgx en pool config: `pool.Config().ConnConfig.Tracer = otelpgx.NewTracer()`
4. Helper `tracing.StartSpan(ctx, name, attrs...)` para spans de dominio
5. MCP handlers: extraer traceparent del request si está, sino crear nuevo

## Riesgos

- Exporter caído: usar `WithRetry` y batch processor con timeout, no bloquear request path
- PII en attributes: política explícita — no incluir `content` ni `email` en span attributes
- Overhead: sampler con ratio bajo en prod (0.05–0.1 default)

## Testing

- Test unit: span se crea y tiene attributes esperados
- Test integration con OTel Collector mock que recibe spans y verifica
- Test sampler: 1000 spans con ratio 0.1 → ~100 exportados, todos errores exportados
