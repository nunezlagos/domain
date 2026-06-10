# Tasks: issue-04.2-mcp-save-tools

## Backend

- [ ] Crear `internal/mcp/tools/save.go` con los 5 tool handlers
- [ ] Implementar `domain_mem_save`: validar campos obligatorios (title, content), aplicar defaults (type=context, scope=personal)
- [ ] Implementar conflict detection: fuzzy match de title contra engine.Search, top 5 candidatos en response
- [ ] Implementar `mem_update`: validar id, merge parcial de campos, verificar existencia
- [ ] Implementar `domain_mem_delete`: soft (default) y hard, verificar existencia previa
- [ ] Implementar `domain_mem_suggest_topic_key`: slugify `type + "-" + title`, truncar a 80, dedup suffix
- [ ] Implementar `domain_mem_save_prompt`: guardar en engine, push a ring buffer
- [ ] Crear `internal/mcp/context.go`: ring buffer circular con capacidad 100, thread-safe
- [ ] Definir JSON Schema en cada tool definition dentro del registry
- [ ] Integrar con `internal/mcp/server.go`: registrar los 5 handlers

## Tests

- [ ] Test unitario: `TestMemSaveMinimal`, `TestMemSaveFull`, `TestMemSaveConflict`, `TestMemSaveInvalidType`
- [ ] Test unitario: `TestMemUpdatePartial`, `TestMemUpdateNotFound`
- [ ] Test unitario: `TestMemDeleteSoft`, `TestMemDeleteHard`
- [ ] Test unitario: `TestMemSuggestTopicKey` (ascii, multi-lang, dedup)
- [ ] Test unitario: `TestMemSavePrompt`, `TestRingBufferOverflow`
- [ ] Test integración: secuencia save → update → delete soft → search no muestra
- [ ] Sabotaje: domain_mem_save sin title → error. mem_update con id string → error de tipo

## Cierre

- [ ] Verificación manual: `mem mcp` + llamadas secuenciales a las 5 tools
- [ ] Suite verde: `go test ./internal/mcp/...`
