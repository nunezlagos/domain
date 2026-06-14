# Tasks: HU-28.4-pgerrcode-error-codes

- [x] **pc-001**: Migrar `service/observation/service.go` (UniqueViolation + ForeignKeyViolation con ConstraintName)
- [x] **pc-002**: Migrar `service/cron/service.go` (UniqueViolation)
- [x] **pc-003**: Migrar `service/agent/service.go` (UniqueViolation)
- [x] **pc-004**: Migrar `service/flow/service.go` (UniqueViolation — 1 lugar real, spec asumió 2)
- [x] **pc-005**: Migrar `service/webhook/service.go` (UniqueViolation)
- [x] **pc-006**: Migrar `service/role/service.go` (helper isUniqueViolation)
- [x] **pc-007**: Migrar `service/spec/service.go` (helper isUniqueViolation)
- [x] **pc-008**: Migrar `service/requirement/service.go` (helper isUniqueViolation)
- [x] **pc-009**: Migrar `service/issue/service.go` (helper isUniqueViolation)
- [x] **pc-010**: Refactor `llm/retry/retry.go` — HTTP status codes via `strconv.Itoa` sobre listas centralizadas.
- [x] **pc-011**: Test de regresión actualizado en `requirement/service_test.go` y `role/service_test.go` inyectando `*pgconn.PgError{Code: pgerrcode.UniqueViolation}`.
- [x] **pc-012**: Suite completa verde — `go test ./internal/service/... ./internal/llm/retry/...` pasa (249 tests).

## Extras dentro del scope `internal/service/**`

- [x] Migrado `service/skill/service.go` (UniqueViolation + CheckViolation con ConstraintName)
- [x] Migrado `service/invite/service.go` (UniqueViolation + ConstraintName)
- [x] Migrado `service/project/service.go` (UniqueViolation)
