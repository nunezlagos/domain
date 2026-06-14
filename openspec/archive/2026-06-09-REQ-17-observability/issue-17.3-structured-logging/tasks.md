# Tasks: issue-17.3-structured-logging

## Backend

- [ ] **log-001**: `logging/setup.go` con Setup() y SetDefault
- [ ] **log-002**: `logging/handler.go` enrichedHandler que agrega trace/request_id
- [ ] **log-003**: `logging/ctx.go` con FromContext, WithRequestID/UserID/ProjectID
- [ ] **log-004**: `logging/middleware.go` HTTPMiddleware injecting request_id (UUID v4) + header X-Request-ID
- [ ] **log-005**: `logging/admin.go` POST /admin/log-level con RBAC admin
- [ ] **log-006**: Reemplazar todos los `log.Printf` existentes por `slog`

## Config

- [ ] **cfg-001**: DOMAIN_LOG_* en issue-01.2

## Tests

- [ ] **test-001**: JSON output schema validation
- [ ] **test-002**: trace_id presente con span
- [ ] **test-003**: middleware HTTP — request_id en log y header
- [ ] **test-004**: admin endpoint cambia nivel y persiste audit
- [ ] **test-005**: linter PII keys

## Docs

- [ ] **docs-001**: `docs/observability/logging.md` con esquema de campos, ejemplos parseables en Loki/Datadog

## Cierre

- [ ] Smoke dev: comparar text vs json output
