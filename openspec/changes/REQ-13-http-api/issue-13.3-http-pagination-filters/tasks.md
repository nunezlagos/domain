# Tasks: issue-13.3-http-pagination-filters

> Decisión de arquitectura: sin QueryParser genérico (mismo razonamiento que
> el factory de 13.1 — handlers explícitos). La pieza central es el paquete
> `internal/api/cursor` (issue-13.6): keyset pagination con cursor opaco
> base64, hash de filtros (cursor inválido si cambian los filtros entre
> páginas), NormalizeSort y MaxLegacyOffset. Cada endpoint List declara sus
> filtros soportados explícitamente (whitelist por construcción). FTS vive
> en GET /api/v1/search (issue-03.7), no como ?q= dispersos.

## Backend

- [x] QueryParser genérico → N/A por diseño (ver nota); cada handler parsea sus params explícitos
- [x] Whitelist de campos sort/filter → por construcción: el handler solo lee los params que soporta; NormalizeSort valida dirección (400 si inválida)
- [x] Validación 400 → cursor corrupto/filters mismatch/sort mismatch → 400 con code específico
- [x] OffsetPaginator → soportado como legacy con MaxLegacyOffset (cap anti OFFSET profundo, convención db.md)
- [x] CursorPaginator keyset → internal/api/cursor (issue-13.6)
- [x] Encode/decode cursor base64 JSON → Cursor.Encode/Decode con validación de integridad
- [x] Sort dinámico → NormalizeSort (asc/desc); multi-campo N/A (orden estable por created_at+id, suficiente para listados actuales)
- [x] Filtros exactos → per-endpoint (ej. observations: project, tags, type; crons/webhooks org-scoped)
- [x] Filtros range [gte]/[lte] → date_from/date_to en search y timeline (sintaxis explícita en lugar de bracket-notation)
- [x] FTS ?q= → centralizado en GET /api/v1/search (issue-03.7) sobre 4 entity types con ts_rank
- [x] Integración en factory → N/A (sin factory)
- [x] Envelope {data, pagination} → next_cursor/has_more/limit (api.md)
- [x] Límite máximo → limit default 50, max 200 (api.md; el spec decía 20/100, la convención del proyecto ganó)
- [x] Edge cases → cursor inválido 400, sort inválido 400, offset cap

## Tests

- [x] Encode/decode roundtrip → TestEncodeDecodeRoundtrip
- [x] Cursor tampered/corrupto → TestDecode_Tampered
- [x] Filters mismatch invalida cursor → TestDecode_FiltersMismatch + HashFilters_Stable/DistinctOnChange
- [x] Sort → TestNormalizeSort + TestDecode_SortMismatch
- [x] FTS ranking → suite de issue-03.7 (search service)
- [x] Integración offset/cursor consistentes → observations list integration
- [x] Sabotaje whitelist → cursor con filtros distintos es rechazado (FiltersMismatch); sort fuera de asc|desc → 400
- [x] Performance → benchmarks BenchmarkCursorEncode/Decode/HashFilters

## Cierre

- [x] Verificación → cubierta por integration de observations list
- [x] Suite verde → 2026-06-11
- [ ] Performance test 10k rows offset vs cursor → DIFERIDO (benchmark con dataset grande, CI weekly)
