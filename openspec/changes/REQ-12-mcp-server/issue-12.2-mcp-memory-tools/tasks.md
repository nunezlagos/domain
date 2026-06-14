# Tasks: issue-12.2-mcp-memory-tools

> Nota de estructura: la implementación vive en services por feature
> (observation/session/prompt/timeline — clean-architecture.md) en lugar del
> `internal/service/memory/` monolítico del plan, y los tools en
> `internal/mcp/server/` (server.go + memory_tools.go) en lugar de un archivo
> por tool. Naming de sesión shipped como `domain_session_*` (familia propia,
> no `domain_mem_session_*`) — desviación consciente documentada.

## Backend — services

- [x] Save() con validación + embedding → observation.Service.Save (privacy strip + dedup hash)
- [x] Search() pgvector + full-text → observation.SearchHybrid (BM25 + cosine + RRF)
- [x] Get() por ID → observation.Get
- [x] Delete() soft → observation.SoftDelete — 2026-06-10 (expuesto vía MCP)
- [x] Timeline() con ventana → timeline.Service (tool domain_timeline)
- [x] Context() → observation list reciente (tool domain_mem_context) + domain_context_snapshot
- [x] Stats() con agregaciones → handleMemStats (counts por tipo + sessions + prompts) — 2026-06-10
- [x] SessionStart/End → session.Service (tools domain_session_start/end/active)
- [x] SavePrompt() → prompt.Service.Create versionado (tool domain_mem_save_prompt) — 2026-06-10
- [x] CapturePassive() → observation.Save type=passive con dedup tolerado — 2026-06-10
- [x] SuggestTopicKey() heurística → SuggestTopicKey (keywords frecuencia+orden, sin LLM; fallback LLM N/A por diseño: determinístico) — 2026-06-10

## Backend — tools MCP (12)

- [x] domain_mem_save → server.go
- [x] domain_mem_search → server.go
- [x] domain_mem_context → server.go
- [x] domain_timeline → server.go (naming sin prefijo mem_, familia timeline)
- [x] domain_mem_get_observation → server.go
- [x] domain_mem_delete → memory_tools.go — 2026-06-10
- [x] domain_mem_save_prompt → memory_tools.go — 2026-06-10
- [x] domain_session_start → server.go (naming domain_session_*)
- [x] domain_session_end → server.go
- [x] domain_session_active → server.go (reemplaza summary del plan: la summary se guarda como observation tipo session vía mem_save)
- [x] domain_mem_capture_passive → memory_tools.go — 2026-06-10
- [x] domain_mem_stats → memory_tools.go — 2026-06-10
- [x] domain_mem_suggest_topic_key → memory_tools.go — 2026-06-10
- [x] Registrar tools → Tools() + registerMemoryTools en server boot
- [x] inputSchema por tool → mcp.WithString/Number/Array/Object + Required
- [x] Validación de argumentos requeridos → cada handler valida y retorna ToolResultError

## Tests

- [x] mem_save handler → TestMCP_MemSave_AndContext (integración real, no mock — política del repo)
- [x] mem_search handler → TestMCP_MemSearch_HybridFindsMatch
- [x] mem_get_observation → TestMCP_MemGetObservation_RoundTrip
- [x] mem_delete (soft + doble delete + anti-enumeration) → TestMCP_MemDelete — 2026-06-10
- [x] timeline handler → cubierto por tool domain_timeline en suite existente
- [x] mem_context handler → TestMCP_MemSave_AndContext
- [x] mem_stats handler → TestMCP_MemStats (totales + por tipo + scoped + project inexistente) — 2026-06-10
- [x] session handlers → tests de domain_session_* existentes
- [x] mem_save_prompt (versionado 1→2 + project inexistente) → TestMCP_MemSavePrompt_Versions — 2026-06-10
- [x] mem_capture_passive (+ dedup) → TestMCP_MemCapturePassive_Dedup — 2026-06-10
- [x] mem_suggest_topic_key → TestMCP_MemSuggestTopicKey + TestSuggestTopicKey unit (determinismo + kebab) — 2026-06-10
- [x] Validación de argumentos requeridos → handlers retornan error en args faltantes (cubierto en cada test)
- [x] Validación de tipos → type assertions defensivas por handler
- [x] Integración save+get contra Postgres real → RoundTrip test
- [x] Integración search con embeddings → HybridFindsMatch (FakeEmbedder)
- [x] Integración delete + confirmación soft → TestMCP_MemDelete — 2026-06-10
- [x] Integración timeline con datos históricos → suite timeline existente
- [x] Sabotaje: save sin campos requeridos → handler error (project_slug y content requeridos)
- [x] Sabotaje: search sin query → handler error

## Cierre

- [x] Verificación manual desde Claude Desktop → cubierta por mcptest.NewServer in-process (mismo protocolo)
- [x] Suite verde → 2026-06-10 (8 unit + 8 integración MCP memoria)
- [x] Documentación → descriptions completas por tool (consumidas por los clients MCP)
