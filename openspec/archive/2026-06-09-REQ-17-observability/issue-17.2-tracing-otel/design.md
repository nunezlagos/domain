# Design: issue-17.2-tracing-otel

## Decisión arquitectónica

**SDK:** `go.opentelemetry.io/otel` v1.x
**Exporter:** OTLP gRPC (default) con fallback OTLP HTTP
**Propagator:** W3C TraceContext (`propagation.TraceContext{}`) + Baggage
**Sampler:** `ParentBased(TraceIDRatioBased(ratio))` con override always-on para errores via span processor custom

## Alternativas descartadas

- **Jaeger native client:** OTel es estándar y vendor-neutral
- **Zipkin format:** OTLP soportado por todos los backends modernos
- **AlwaysOn sampler:** costos prohibitivos en prod

## Componentes

```
internal/observability/tracing/
  provider.go     → Setup(ctx, cfg) (*TracerProvider, error)
  http.go         → Middleware(handler) http.Handler
  pgx.go          → InstrumentPool(cfg *pgxpool.Config)
  mcp.go          → ExtractTraceparent(req), InjectTraceparent(resp)
  attrs.go        → SafeAttrs() — whitelist de attributes permitidos (sin PII)
```

## Variables de entorno

| var | default | descripción |
|-----|---------|-------------|
| DOMAIN_OTEL_ENABLED | false | Habilita tracing |
| DOMAIN_OTEL_EXPORTER_OTLP_ENDPOINT | http://localhost:4317 | Endpoint OTLP |
| DOMAIN_OTEL_EXPORTER_OTLP_PROTOCOL | grpc | grpc o http/protobuf |
| DOMAIN_OTEL_SAMPLE_RATIO | 0.1 | Sampling ratio [0..1] |
| DOMAIN_OTEL_SERVICE_NAME | domain-mcp | service.name resource attr |

## TDD plan

1. Span HTTP se crea con name+attrs esperados
2. Span pgx aparece como hijo del HTTP span
3. MCP request con traceparent header → span hijo del traceparent
4. Sampler ratio respetado en 1k requests
5. Errores siempre exportados aun con ratio bajo
6. Sabotaje: agregar `attr.String("email", user.Email)` → test PII detecta y falla
