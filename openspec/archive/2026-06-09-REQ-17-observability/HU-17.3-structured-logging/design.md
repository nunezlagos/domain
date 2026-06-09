# Design: HU-17.3-structured-logging

## Decisión arquitectónica

**Logger:** `log/slog` (stdlib Go 1.21+) — sin terceros como zap/zerolog
**Handlers:** JSONHandler (prod), TextHandler con colorización wrapper (dev)
**Correlación:** custom handler wrapper que enriquece con `trace_id`/`span_id` desde OTel + `request_id` desde context

## Alternativas descartadas

- **zap:** más rápido pero API duplicada; slog cubre 95% con stdlib y sin lock-in
- **zerolog:** similar; slog ganó por ser stdlib
- **logrus:** deprecado oficialmente

## Componentes

```
internal/observability/logging/
  setup.go         → Setup(cfg) *slog.Logger; SetDefault
  handler.go       → enrichedHandler wrappea base handler y agrega trace/request_id
  ctx.go           → FromContext(ctx) *slog.Logger; WithRequestID/UserID/ProjectID
  middleware.go    → HTTPMiddleware(next) http.Handler inyecta request_id
  admin.go         → HandlerLogLevel(w, r) cambia nivel dinámico
  linter_test.go   → recorre `internal/` buscando keys prohibidas en slog calls
```

## Variables de entorno

| var | default | descripción |
|-----|---------|-------------|
| DOMAIN_LOG_LEVEL | info | debug, info, warn, error |
| DOMAIN_LOG_FORMAT | text en dev, json en prod | text \| json |
| DOMAIN_LOG_OUTPUT | stdout | stdout \| stderr |
| DOMAIN_LOG_ADD_SOURCE | false | incluir file:line |

## TDD plan

1. JSON output incluye campos requeridos
2. text output legible y coloreado
3. trace_id presente cuando hay span activo
4. request_id propagado en HTTP middleware
5. POST /admin/log-level cambia nivel observable
6. Linter test: caso fixture con `slog.String("password", x)` → falla
