# Proposal: issue-01.3-fts5-search

## Intención

Implementar búsqueda de texto completo (FTS5) sobre las tablas `observations` y `user_prompts` para que el usuario pueda recuperar observaciones previas mediante palabras clave, con filtros por tipo/proyecto/scope, snippets destacados y paginación. Es la diferencia entre tener datos y poder _encontrar_ lo que importa.

## Scope

### In Scope
- Función `SearchObservations(query, filter, pagination)` en la store interface
- Construcción de query FTS5 MATCH con sanitización de tokens
- Filtros por `type`, `project`, `scope` mediante JOIN con `observations.rowid`
- Exclusión automática de registros soft-deleteados (`deleted_at IS NOT NULL`)
- Devolución de metadatos: `id`, `type`, `project`, `scope`, `title`, `content`, `created_at`, `relevance` (bm25)
- Snippet/highlight via FTS5 `snippet()` function
- Paginación con `LIMIT` / `OFFSET`
- Sanitización de query: escape FTS5 special chars, wrap tokens en doble quote
- Función `SearchPrompts(query, pagination)` análoga sobre `prompts_fts`
- Mantenimiento del índice FTS5 vía triggers `INSERT`/`UPDATE`/`DELETE` en `observations` y `user_prompts`
- Tests unitarios para sanitización + queries reales sobre SQLite in-memory

### Out of Scope
- Búsqueda por vector embeddings / similitud semántica (futuro)
- Búsqueda con stemming o expansión de query (FTS5 default es tokenización simple)
- Ranking custom más allá de `bm25` (se puede afinar después)
- Interfaz CLI o API REST (lo expone REQ-03/REQ-04)
- Highlight en frontend (el dato raw viene del store)

## Enfoque técnico

- Paquete `store/sqlite/` → archivo `fts5.go` con toda la lógica de FTS5
- `sanitizeFTS5(query string) string`: parsea la query del usuario, escapa `^ " * : ~ ( ) + -`, envuelve cada token en `"..."`, la concatena con spaces
- `SearchObservations` construye: `SELECT ... FROM observations_fts WHERE observations_fts MATCH ?` con JOIN a `observations` para filtros + exclusión soft-delete + metadata
- `SearchPrompts` construye query similar sobre `prompts_fts`
- Snippets: `snippet(observations_fts, 1, '<b>', '</b>', '...', 32)` sobre content
- Triggers: `ai`, `ad`, `au` en observaciones mantienen `observations_fts` sincronizado
- Testing: tabla in-memory con datos seeded, validar resultados, filtros, paginación, sanitización

## Riesgos

| Riesgo | Impacto | Mitigación |
|--------|---------|------------|
| FTS5 syntax error por query maliciosa | Medio | Sanitización estricta: escapar + wrap quotes |
| Soft-delete leak por trigger mal configurado | Alto (privacidad) | `WHERE o.deleted_at IS NULL` explícito en search query, no solo triggers |
| Performance en índices grandes (>100k rows) | Medio | `LIMIT` siempre obligatorio, benchmark con datos reales |
| Snippet() muy lento en tablas grandes | Bajo | Solo se calcula si se pide explicitamente (flag `with_snippets`) |
| Tokenizer no maneja UTF-8 mixto | Bajo | FTS5 unicode61 tokenizer maneja UTF-8 out of the box |

## Testing

- **Unitarios:** `TestSanitizeFTS5` con tabla de casos (vacio, special chars, quotes, normal)
- **Integración:** SQLite in-memory, crear schema, insertar observations, ejecutar search queries reales
- **Cobertura Gherkin:** un test por scenario del hu.md
- **Sabotaje:** (1) romper sanitización → query con `^` falla → test detecta error FTS5. (2) no excluir soft-delete → test devuelve deleted → test falla. (3) FTS5 trigger no actualiza en UPDATE title → search devuelve stale data → test falla
