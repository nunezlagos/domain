# Tasks: HU-28.5-fix-ignored-errors

- [ ] **ie-001**: Fix `handler/api.go` writeJSON — log error en vez de `_ =`
- [ ] **ie-002**: Fix `middleware/idempotency.go` — log errores de w.Write
- [ ] **ie-003**: Fix `auth/apikey/middleware.go` — log errores de w.Write
- [ ] **ie-004**: Fix audit errors en `service/observation/` (3 lugares)
- [ ] **ie-005**: Fix audit errors en `service/session/` (3 lugares)
- [ ] **ie-006**: Fix audit errors en `service/agent/` (3 lugares)
- [ ] **ie-007**: Fix audit errors en `service/lifecycle/` (3 lugares)
- [ ] **ie-008**: Fix audit errors en `service/cron/` (2 lugares)
- [ ] **ie-009**: Fix audit errors en `service/webhook/`, `runner/agent/` (2 lugares)
- [ ] **ie-010**: Fix `llm/google/provider.go` — marshal error propagado
- [ ] **ie-011**: Fix `llm/ollama/provider.go` (4 lugares) — marshal error propagado
- [ ] **ie-012**: Fix `llm/openai/provider.go` (2 lugares) — marshal error propagado
- [ ] **ie-013**: Fix `webui/admin_memories.go` (4 lugares)
- [ ] **ie-014**: Fix `mcp/server/memory_tools.go` (rollback log)
- [ ] **ie-015**: Tests: inyectar error en encoder → log verificado con test logger
- [ ] **ie-016**: Suite completa verde
