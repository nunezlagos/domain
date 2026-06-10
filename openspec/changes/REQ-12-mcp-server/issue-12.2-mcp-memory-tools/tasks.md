# Tasks: issue-12.2-mcp-memory-tools

## Backend

- [ ] Implementar `MemoryService` en `internal/service/memory/service.go` con métodos para cada operación
- [ ] Implementar `internal/service/memory/save.go`: Save() con validación de campos, embedding generation
- [ ] Implementar `internal/service/memory/search.go`: Search() con pgvector cosine distance + fallback full-text
- [ ] Implementar `internal/service/memory/get.go`: Get() por ID
- [ ] Implementar `internal/service/memory/delete.go`: Delete() soft + hard
- [ ] Implementar `internal/service/memory/timeline.go`: Timeline() con ventana before/after
- [ ] Implementar `internal/service/memory/context.go`: Context() con resumen + stats
- [ ] Implementar `internal/service/memory/stats.go`: Stats() con agregaciones
- [ ] Implementar `internal/service/memory/session.go`: SessionStart/End/Summary
- [ ] Implementar `internal/service/memory/prompt.go`: SavePrompt()
- [ ] Implementar `internal/service/memory/passive.go`: CapturePassive()
- [ ] Implementar `internal/service/memory/topic.go`: SuggestTopicKey() con heurística + LLM fallback
- [ ] Crear `internal/mcp/tools/memory/tool_mem_save.go`: handler MCP
- [ ] Crear `internal/mcp/tools/memory/tool_mem_search.go`: handler MCP
- [ ] Crear `internal/mcp/tools/memory/tool_mem_context.go`: handler MCP
- [ ] Crear `internal/mcp/tools/memory/tool_mem_timeline.go`: handler MCP
- [ ] Crear `internal/mcp/tools/memory/tool_mem_get_observation.go`: handler MCP
- [ ] Crear `internal/mcp/tools/memory/tool_mem_delete.go`: handler MCP
- [ ] Crear `internal/mcp/tools/memory/tool_mem_save_prompt.go`: handler MCP
- [ ] Crear `internal/mcp/tools/memory/tool_mem_session_start.go`: handler MCP
- [ ] Crear `internal/mcp/tools/memory/tool_mem_session_end.go`: handler MCP
- [ ] Crear `internal/mcp/tools/memory/tool_mem_session_summary.go`: handler MCP
- [ ] Crear `internal/mcp/tools/memory/tool_mem_capture_passive.go`: handler MCP
- [ ] Crear `internal/mcp/tools/memory/tool_mem_stats.go`: handler MCP
- [ ] Crear `internal/mcp/tools/memory/tool_mem_suggest_topic_key.go`: handler MCP
- [ ] Registrar las 12 tools en `cmd/domain-mcp/main.go`
- [ ] Definir inputSchema como structs JSON Schema para cada tool
- [ ] Implementar validación de argumentos requeridos por tool

## Frontend

- [ ] (No aplica)

## Tests

- [ ] Test unitario: domain_mem_save handler con MemoryService mock
- [ ] Test unitario: domain_mem_search handler con resultados mock
- [ ] Test unitario: domain_mem_get_observation handler
- [ ] Test unitario: domain_mem_delete handler (soft delete)
- [ ] Test unitario: domain_mem_timeline handler
- [ ] Test unitario: domain_mem_context handler
- [ ] Test unitario: domain_mem_stats handler
- [ ] Test unitario: domain_mem_session_start/end/summary handlers
- [ ] Test unitario: domain_mem_save_prompt handler
- [ ] Test unitario: domain_mem_capture_passive handler
- [ ] Test unitario: domain_mem_suggest_topic_key handler
- [ ] Test unitario: validación de argumentos requeridos
- [ ] Test unitario: validación de tipos de argumentos
- [ ] Test integración: domain_mem_save + domain_mem_get_observation contra Postgres real
- [ ] Test integración: domain_mem_search con embeddings reales
- [ ] Test integración: domain_mem_delete + confirmación soft delete
- [ ] Test integración: domain_mem_timeline con datos históricos
- [ ] Sabotaje: domain_mem_save sin title → debe fallar con error de validación
- [ ] Sabotaje: domain_mem_search sin query → debe fallar

## Cierre

- [ ] Verificación manual: invocar cada tool desde Claude Desktop
- [ ] Suite verde: `go test ./internal/mcp/tools/memory/... ./internal/service/memory/...`
- [ ] Documentar cada tool MCP con ejemplos de uso
