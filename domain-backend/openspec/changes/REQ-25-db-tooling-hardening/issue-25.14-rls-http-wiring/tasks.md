# Tasks: issue-25.14-rls-http-wiring

- [x] **rls-100**: HU spec completo (issue.md, design.md, proposal.md, tasks.md, state.yaml) — 2026-06-11
- [x] **rls-101**: Migration 000085 RLS en observations + sessions (issue-25.5 cierre) — 2026-06-11
- [x] **rls-102**: `txctx.WithTxContext` + `TxFromContext` + unit tests — 2026-06-11 (5/5 verde)
- [x] **rls-103**: Test E2E RED — httptest con testcontainers, 2 orgs, GET obs con key de A → 0 rows sin wireup — 2026-06-11
- [x] **rls-104**: `apikey.Middleware` modificado para abrir tx + SET LOCAL post-auth — 2026-06-11
- [x] **rls-105**: Refactor `service/observation` para usar `txctx.TxFromContext` con fallback — 2026-06-11
- [x] **rls-106**: Refactor `service/session` idem + End() reusa tx del middleware — 2026-06-11
- [x] **rls-107**: Refactor `service/timeline`, `service/search`, `service/lifecycle` (idempotente con fallback) — 2026-06-11
- [x] **rls-108**: Refactor `context/stitcher` (usa observations) — 2026-06-11
- [x] **rls-109**: Wireup MCP server — `mcp/server/wireup.go` con `withOrgCtx` — 2026-06-11
- [x] **rls-110**: Refactor `mcp/server/memory_tools.go` para usar wireup (handleMemDelete, handleMemCapturePassive) — 2026-06-11
- [x] **rls-111**: Refactor `webui/admin_memories.go` (usa observations) — 2026-06-11
- [x] **rls-112**: Tests E2E GREEN — GET con key A ve 1 obs, id de B → 404, cross-org bloqueado — 2026-06-11
- [x] **rls-113**: Sabotaje — handler monkey-patch ignora tx → RLS bloquea (0 rows) — 2026-06-11
- [ ] **rls-114**: Suite completa short tests verde (0 regresiones) — pendiente de corrida en CI con Docker
- [ ] **rls-115**: Suite integration tests verde — pendiente (requiere Docker)
- [x] **rls-116**: Commit por intención + state.yaml → implemented — 2026-06-11
