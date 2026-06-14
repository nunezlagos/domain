# Tasks: HU-28.5-fix-ignored-errors

- [x] **ie-001**: Fix `handler/api.go` writeJSON — log error en vez de `_ =`
- [x] **ie-002**: Fix `middleware/idempotency.go` — log errores de w.Write (también `middleware/ratelimit.go` y `handler/flow.go` export)
- [ ] **ie-003**: Fix `auth/apikey/middleware.go` — log errores de w.Write — fuera de scope (no está en `internal/api/middleware/`); pendiente para HU follow-up
- [x] **ie-004**: Fix audit errors en `service/observation/` (3 lugares)
- [x] **ie-005**: Fix audit errors en `service/session/` (3 lugares)
- [x] **ie-006**: Fix audit errors en `service/agent/` (3 lugares)
- [x] **ie-007**: Fix audit errors en `service/lifecycle/` (3 lugares — service.go + erasure.go)
- [x] **ie-008**: Fix audit errors en `service/cron/` (2 lugares)
- [x] **ie-009**: Fix audit errors en `service/webhook/` (service.go + management.go); `runner/agent/` no contiene `_ = s.Audit.Record` en la rama actual
- [ ] **ie-010**: Fix `llm/google/provider.go` — fuera de scope (no en handler/service/middleware)
- [ ] **ie-011**: Fix `llm/ollama/provider.go` (4 lugares) — fuera de scope
- [ ] **ie-012**: Fix `llm/openai/provider.go` (2 lugares) — fuera de scope
- [ ] **ie-013**: Fix `webui/admin_memories.go` (4 lugares) — fuera de scope (no en handler/service/middleware)
- [ ] **ie-014**: Fix `mcp/server/memory_tools.go` (rollback log) — fuera de scope
- [x] **ie-015**: Tests: inyectar error en encoder → log verificado con test logger (`writejson_test.go`, `recordorlog_test.go`)
- [x] **ie-016**: Suite completa verde (663 tests pass en audit + api + service)

Adicional fuera de tasks.md pero dentro del scope autorizado:
- [x] Migrados además: `service/prompt`, `service/requirement`, `service/invite`, `service/org`, `service/issue`, `service/projectmerge`, `service/knowledge`, `service/issuebuilder`, `service/skill`, `service/spec`, `service/enrollment`, `service/role`, `service/task`, `service/project`, `service/intake`, `service/flow`, `handler/flow.go` (1 call) — total 63 callers `_ = s.Audit.Record` reemplazados por `audit.RecordOrLog`.

Abstracción strangler-fig agregada: `audit.RecordOrLog(ctx, recorder, event)` en `internal/audit/audit.go`. Nil-safe; loggea con `slog.Warn` `action`, `entity_type`, `entity_id`.
