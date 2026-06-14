# Tasks: issue-17.2-tracing-otel

## Backend

- [ ] **otel-001**: Agregar deps `go.opentelemetry.io/otel`, `otlptracegrpc`, `otelhttp`, `otelpgx`
- [ ] **otel-002**: `tracing/provider.go` con Setup(ctx, cfg) y shutdown ordered
- [ ] **otel-003**: Middleware HTTP server con `otelhttp.NewHandler`
- [ ] **otel-004**: HTTP client (LLM calls) con `otelhttp.NewTransport`
- [ ] **otel-005**: Pool pgx instrumentado con `otelpgx.NewTracer()`
- [ ] **otel-006**: Helper MCP `ExtractTraceparent` / `InjectTraceparent` para handlers stdio
- [ ] **otel-007**: Always-on sampler para errores (custom span processor)
- [ ] **otel-008**: Whitelist `SafeAttrs()` con linter test

## Config

- [ ] **cfg-001**: Variables `DOMAIN_OTEL_*` en issue-01.2 config system

## Tests

- [ ] **test-001**: Unit span HTTP attrs
- [ ] **test-002**: Integration con collector mock + pgx + HTTP
- [ ] **test-003**: Sampler ratio statistical test
- [ ] **test-004**: PII linter — fallar si attr key matches whitelist negativo
- [ ] **sabotaje-001**: Tirar exporter → request no debe colgar (timeout 5s)

## Docs

- [ ] **docs-001**: `docs/observability/tracing.md` — cómo correr Jaeger local con compose dev, ejemplos de queries

## Cierre

- [ ] Smoke: levantar Jaeger via dev-up extra, hacer request, ver span en UI
