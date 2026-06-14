# Tasks: HU-28.4-pgerrcode-error-codes

- [ ] **pc-001**: Migrar `service/observation/service.go` (2 lugares)
- [ ] **pc-002**: Migrar `service/cron/service.go` (1 lugar)
- [ ] **pc-003**: Migrar `service/agent/service.go` (1 lugar)
- [ ] **pc-004**: Migrar `service/flow/service.go` (2 lugares)
- [ ] **pc-005**: Migrar `service/webhook/service.go` (1 lugar)
- [ ] **pc-006**: Migrar `service/role/service.go` (1 lugar)
- [ ] **pc-007**: Migrar `service/spec/service.go` (1 lugar)
- [ ] **pc-008**: Migrar `service/requirement/service.go` (1 lugar)
- [ ] **pc-009**: Migrar `service/issue/service.go` (1 lugar)
- [ ] **pc-010**: Migrar `llm/retry/retry.go` (IsTransient, raw HTTP status string → strconv)
- [ ] **pc-011**: Test de regresión: inyectar `&pgconn.PgError{Code: pgerrcode.UniqueViolation}` → mapeo correcto
- [ ] **pc-012**: Suite completa verde
