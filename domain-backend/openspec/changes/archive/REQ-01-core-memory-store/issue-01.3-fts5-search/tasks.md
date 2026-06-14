# Tasks: issue-01.3-fts5-search

## Backend

### Store Interface

- [ ] `interface.go`: Add `SearchFilter`, `SearchPagination`, `SearchOpts`, `SearchResult` types
- [ ] `interface.go`: Add `SearchObservations(ctx, query, opts) ([]SearchResult, int, error)` to Store interface
- [ ] `interface.go`: Add `SearchPrompts(ctx, query, pagination) ([]PromptSearchResult, int, error)` to Store interface

### SQLite implementation — `store/sqlite/fts5.go`

- [ ] Implement `sanitizeFTS5(query string) (string, error)`:
  - Trim whitespace, return `ErrEmptyQuery` if blank
  - Split tokens by spaces
  - Escape `\` and `"` inside each token
  - Wrap each token in double quotes
  - Join with spaces
  - Unit-test alongside
- [ ] Implement `buildSearchQuery(table, filters, withSnippets)` — SQL builder privado:
  - Base: `SELECT {cols} FROM {table}_fts fts JOIN {table} o ON o.id = fts.rowid WHERE {table}_fts MATCH ? AND o.deleted_at IS NULL`
  - Conditional filters: `o.type = ?`, `o.project = ?`, `o.scope = ?`
  - Conditional snippet column
  - `ORDER BY bm25({table}_fts)` + `LIMIT ? OFFSET ?`
- [ ] Implement `SearchObservations`:
  - Sanitize query
  - Build SQL with filters + pagination
  - Execute MATCH query con parámetros
  - Scan rows into `[]SearchResult`
  - Execute COUNT query for total (mismos filters, sin paginación)
  - Handle no-rows gracefully (empty slice, not nil)
- [ ] Implement `SearchPrompts`:
  - Similar a SearchObservations pero sobre `prompts_fts` y `user_prompts`
  - Sin filtros type/project/scope
  - `PromptSearchResult` incluye id, content, created_at, relevance, snippet
- [ ] Define `ErrEmptyQuery` como sentinel error en package

### FTS5 schema & triggers — `store/sqlite/triggers.go`

- [ ] Create `observations_fts` virtual table (external content) en init schema
- [ ] Create `prompts_fts` virtual table (external content) en init schema
- [ ] Trigger `observations_ai`: AFTER INSERT → insert into observations_fts
- [ ] Trigger `observations_ad`: AFTER DELETE → delete from observations_fts via 'delete' command
- [ ] Trigger `observations_au`: AFTER UPDATE OF title, content → delete old + insert new
- [ ] Triggers análogos para `user_prompts`:
  - `user_prompts_ai` (AFTER INSERT)
  - `user_prompts_ad` (AFTER DELETE)
  - `user_prompts_au` (AFTER UPDATE OF content)

### Init / migration

- [ ] Add `CREATE VIRTUAL TABLE IF NOT EXISTS observations_fts...` to schema init
- [ ] Add `CREATE VIRTUAL TABLE IF NOT EXISTS prompts_fts...` to schema init
- [ ] Add trigger creation statements to schema init
- [ ] Consider `INSERT INTO observations_fts(observations_fts) VALUES('rebuild')` for existing data on first migration

## Tests

### Unit tests — `store/sqlite/fts5_test.go`

- [ ] `TestSanitizeFTS5` — Table-driven:
  - `""` → `ErrEmptyQuery`
  - `"  "` → `ErrEmptyQuery`
  - `"hello"` → `"\"hello\""`
  - `"hello world"` → `"\"hello\" \"world\""`
  - `"don't stop"` → `"\"don't\" \"stop\""` (apostrophe preserved)
  - `"^NEAR"` → `"\"^NEAR\""` (special char escaped via quoting)
  - `"he said \"hi\""` → `"\"he said \\\"hi\\\"\""` (inner quotes escaped)

### Integration tests — `store/sqlite/store_test.go`

- [ ] `TestSearchObservations_EmptyDB` — search devuelve slice vacío, no error
- [ ] `TestSearchObservations_Basic` — insert observation with title "error handler", search "error" → match
- [ ] `TestSearchObservations_MatchTitle` — "handler config" matches title "handler config"
- [ ] `TestSearchObservations_MatchContent` — matches content text
- [ ] `TestSearchObservations_ExcludesSoftDeleted` — observation con `deleted_at` seteado no aparece
- [ ] `TestSearchObservations_FilterByType` — type "decision" solo devuelve decisiones
- [ ] `TestSearchObservations_FilterByProject` — project "Domain" solo devuelve ese proyecto
- [ ] `TestSearchObservations_FilterByScope` — scope "project" solo devuelve ese scope
- [ ] `TestSearchObservations_CombinedFilters` — type+project+scope combinados
- [ ] `TestSearchObservations_EmptyQuery` — devuelve error específico
- [ ] `TestSearchObservations_Pagination` — insert 25, limit 10 offset 0 → 10, offset 10 → 10, offset 20 → 5
- [ ] `TestSearchObservations_Snippets` — withSnippets=true, snippet contiene `<mark>` tags
- [ ] `TestSearchObservations_RelevanceRanking` — exact title match ranks higher than partial content match
- [ ] `TestSearchPrompts_Basic` — insert prompt, search keyword, match
- [ ] `TestSearchPrompts_EmptyDB` — empty results
- [ ] `TestTriggers_InsertFTS` — insert observation, search immediately, finds it
- [ ] `TestTriggers_UpdateTitleFTS` — update title, old title no longer matches, new title matches
- [ ] `TestTriggers_DeleteFTS` — delete observation, no longer in search results

### Sabotaje

- [ ] **Sabotaje sanitización**: comment out `wrap in quotes`, query with `^` → expect FTS5 syntax error → test catches it
- [ ] **Sabotaje soft-delete filter**: remove `AND o.deleted_at IS NULL` → search returns deleted → test catches it
- [ ] **Sabotaje trigger UPDATE**: comment out trigger `observations_au` → update title → search old title still returns → test catches it
- [ ] **Sabotaje trigger DELETE**: comment out trigger `observations_ad` → delete observation → search still returns → test catches it

## Cierre

- [ ] Verificación manual: crear observaciones con distintos tipos/proyectos/scopes, buscar, filtrar, paginar
- [ ] Suite verde: `go test ./store/sqlite/ -run TestFTS5 -v` pasa completo
- [ ] Suite completa: `go test ./...` pasa sin regresiones
- [ ] `go vet ./...` sin issues
- [ ] Revisar que no haya secretos ni paths hardcodeados
