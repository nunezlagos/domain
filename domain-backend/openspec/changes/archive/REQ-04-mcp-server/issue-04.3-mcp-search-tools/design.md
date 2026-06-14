# Design: issue-04.3-mcp-search-tools

## Decisión arquitectónica

| Decisión | Opción elegida | Alternativas |
|----------|---------------|--------------|
| Search engine | SQLite FTS5 | Bleve, Bluge, meilisearch |
| Content truncation | 500 chars en search, full en get_observation | Siempre truncado (pierde info) |
| Timeline ordering | Por id (monotónico) | Por created_at (permite futuro backfill) |
| Stats cache | No cache inicial, caché de 30s si lento | Sin caché (siempre fresco) |

SQLite FTS5 se elige porque el motor de almacenamiento ya es SQLite — no añade dependencias externas. FTS5 tiene ranking BM25 nativo. Bleve/Bluge añadirían 20MB+ al binary.

## Alternativas descartadas

- **Bleve:** Proyecto semi-abandonado, integration pesada con el store existente.
- **Meilisearch:** Requiere server aparte, demasiado para un CLI tool.
- **SQL LIKE '%term%':** No escala, no soporta ranking, no soporta stemming.

## Diagrama

```
domain_mem_search(request)
  │
  ├─► Parse & validate filters (query, type, project, scope, limit)
  ├─► Build FTS5 query: "SELECT ... FROM observations_fts WHERE ..."
  ├─► Execute query with ranking (bm25)
  ├─► For each result: truncate content to 500 chars
  ├─► For each result: check conflict table → annotate
  └─► Return { results: [...], total: int, limit: int }

domain_mem_context(request)
  │
  ├─► Resolve project (issue-04.6)
  ├─► Query: "SELECT ... FROM obs WHERE project=? ORDER BY created_at DESC LIMIT 10"
  ├─► Query active session if exists
  └─► Return { observations: [...], session: {...}, project: "..." }

domain_mem_timeline(request)
  │
  ├─► Validate observation_id exists
  ├─► Query before: "WHERE id < ? ORDER BY id DESC LIMIT before"
  ├─► Query after: "WHERE id > ? ORDER BY id ASC LIMIT after"
  ├─► Merge: reverse before slice, append after
  └─► Return { before: [...], after: [...], center: {...} }

domain_mem_get_observation(request)
  │
  ├─► Validate id
  ├─► "SELECT * FROM observations WHERE id=?"
  │     ├─► Not found → return error
  │     └─► Found → return full object (no truncation)
  └─► Return { observation }

domain_mem_stats(request)
  │
  ├─► Run aggregation queries
  ├─► Calculate storage estimate (SUM(length(content)))
  └─► Return { total_obs, total_sessions, total_prompts, ... }
```

## TDD plan

**Red:**
1. `TestMemSearchBasic`: insertar 3 obs, buscar por término → encuentra 1
2. `TestMemSearchEmpty`: buscar termino inexistente → 0 resultados
3. `TestMemSearchFilterType`: filtrar por type → solo ese type
4. `TestMemSearchFilterProject`: filtrar por project
5. `TestMemSearchAllProjects`: all_projects=true → múltiples projects
6. `TestMemSearchLimit`: limit=2 → max 2 resultados
7. `TestMemSearchConflictAnnotation`: obs con conflicto → tiene campo conflicts
8. `TestMemContext`: última observación aparece primera
9. `TestMemTimeline`: 7 obs, timeline con before=3,after=3 → 6 vecinos + centro
10. `TestMemTimelineEdge`: obs 1 con before=5 → <5 before
11. `TestMemGetObservation`: full content sin truncar
12. `TestMemGetObservationNotFound`: 404
13. `TestMemStats`: todos los campos presentes

**Green:** Store in-memory con FTS5, handlers.

**Refactor:** Extraer buildQuery, truncation, conflict annotation como funciones separadas.

**Sabotaje:** Romper FTS5 query builder → search devuelve 0 results → restaurar.

## Riesgos y mitigación

- **FTS5 syntax injection:** Sanitizar query input. Rechazar queries con caracteres de control. Escapar `*`, `"`, `(` adecuadamente.
- **Rendimiento en timeline con IDs grandes:** Índice en `id`. `WHERE id < ? ORDER BY id DESC LIMIT N` es O(log N).
