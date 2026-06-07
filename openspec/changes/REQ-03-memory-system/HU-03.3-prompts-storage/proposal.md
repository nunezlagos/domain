# Proposal: HU-03.3-prompts-storage

## Intención

Implementar almacenamiento de prompts con las mismas capacidades de FTS que observations, más un buffer process-local que permite captura inmediata sin bloqueo. El buffer emula el patrón de engram donde `domain_mem_save` con `capture_prompt` captura el prompt actual.

## Scope

**Incluye:**
- Tabla `prompts` con tsvector, GIN index
- Columnas: `id` (UUID), `session_id` (TEXT, FK a sessions), `content` (TEXT), `tsv` (TSVECTOR generated), `created_at`
- CRUD: Insert, GetByID, Search (tsvector), ListBySession, Delete
- Buffer process-local: buffered channel con size configurable, flush on shutdown, batch insert cada N segundos o N prompts
- Integración con `domain_mem_save`: si `capture_prompt` es true, capturar del contexto actual y encolar en buffer
- Filtros: session_id, limit, offset, total count

**Excluye:**
- Edición de prompts (inmutable por diseño)
- Asociación automática con la sesión activa (se pasa explícitamente)
- Prompt templating o versioning

## Enfoque técnico

1. **Migración**: tabla `prompts` similar a observations, con FK opcional a sessions
2. **Buffer**: `PromptBuffer` struct con `chan Prompt`, `ticker` para flush periódico, `sync.WaitGroup` para graceful shutdown
3. **Batch insert**: INSERT múltiple cada 500ms o cada 50 prompts (lo que ocurra primero)
4. **Integración**: `MemoryService.SavePrompt(content, sessionID)` → encola en buffer
5. **Search**: mismo approach que HU-03.1 pero solo sobre prompts

## Riesgos

| Riesgo | Impacto | Mitigación |
|--------|---------|------------|
| Pérdida de prompts en buffer si crash | Bajo | Buffer es best-effort; critical prompts deben ir directo a DB |
| Buffer overflow | Medio | Channel con buffer size configurable; si está lleno, bloquea o descarta con log |
| Race condition en shutdown | Medio | WaitGroup + close channel + flush final antes de salir |

## Testing

- **Unitarios**: buffer con channel mockeado, verificar flush
- **Integración**: insert → search → delete
- **Sabotaje**: llenar buffer y verificar bloqueo o descarte
