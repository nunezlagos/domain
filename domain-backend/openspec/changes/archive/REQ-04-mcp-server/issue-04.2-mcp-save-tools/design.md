# Design: issue-04.2-mcp-save-tools

## Decisión arquitectónica

| Decisión | Opción elegida | Alternativas |
|----------|---------------|--------------|
| Store subyacente | `engram.Engine` interface + in-memory default | SQLite directo, FS JSON |
| Conflict detection | Fuzzy title match >70% pre-insert | Embeddings/semántico (futuro REQ-10) |
| Partial update | Merge map → struct existente | SQL UPDATE SET field por field |
| Ring buffer | Array circular `[100]*PromptEntry` con mutex | Slice con append (crece sin límite) |

Se usa interfaz `engram.Engine` para mantener el MCP desacoplado del motor real. El ring buffer es array circular fijo para evitar GC pressure y garantizar O(1) inserción.

## Alternativas descartadas

- **Embeddings para conflict detection:** Muy pesado para stdio. El fuzzy match léxico es órdenes de magnitud más rápido y suficiente para detectar duplicados obvios.
- **SQLite directo en MCP:** El MCP server no debe tener conocimiento de la capa de persistencia. Delega a `engram.Engine`.

## Diagrama

```
domain_mem_save(request)
  │
  ├─► Validate input (JSON Schema)
  ├─► Conflict detection (fuzzy title match)
  │     ├─► No conflict: proceed
  │     └─► Conflict: attach candidates[] to response
  ├─► engine.CreateObservation(params)
  └─► Return { id, ...observation }

mem_update(request)
  │
  ├─► Validate: id required, at least one field
  ├─► engine.GetObservation(id)
  │     ├─► Not found → return error
  │     └─► Found → merge fields
  ├─► engine.UpdateObservation(id, merged)
  └─► Return { success: true }

domain_mem_delete(request)
  │
  ├─► Validate: id required, hard_delete default false
  ├─► engine.DeleteObservation(id, hard)
  └─► Return { success: true, deleted: true, hard: bool }

domain_mem_suggest_topic_key(request)
  │
  ├─► Normalize: type + title → slugify
  ├─► Dedup check: exists? → append -2, -3...
  └─► Return { topic_key: string }

domain_mem_save_prompt(request)
  │
  ├─► Validate: content required
  ├─► engine.SavePrompt(content, session_id, timestamp)
  ├─► ringBuffer.Push({ content, session_id, timestamp })
  └─► Return { success: true }
```

## TDD plan

**Red:**
1. `TestMemSaveMinimal`: guardar solo title+content → defaults aplicados
2. `TestMemSaveFull`: todos los campos → lectura exacta
3. `TestMemSaveConflict`: título similar → candidates[] no vacío
4. `TestMemSaveInvalidType`: error de validación
5. `TestMemUpdatePartial`: actualizar solo content → otros campos intactos
6. `TestMemUpdateNotFound`: id 99999 → error
7. `TestMemDeleteSoft`: soft delete → md != nil, search no lo encuentra
8. `TestMemDeleteHard`: hard delete → permanentemente ido
9. `TestMemSuggestTopicKey`: type+title → slug correcto
10. `TestMemSavePrompt`: guarda y aparece en ring buffer
11. `TestRingBufferOverflow`: 101 prompts → el primero se descarta

**Green:** Implementar cada handler+store mínimo.

**Refactor:** Extraer validators, normalizers, interfaz de store común.

**Sabotaje:** Romper fuzzy match → test conflict no detecta → restaurar.

## Riesgos y mitigación

- **Concurrencia en ring buffer:** `sync.RWMutex`, las lecturas son frecuentes (context tool), escrituras ocasionales. RLock para leer, Lock para escribir.
- **Fuzzy match sin dependencias:** Implementar Levenshtein simple con límite de 200 caracteres de título. Suficiente para detección de duplicados.
