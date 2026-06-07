# Design: HU-01.3-fts5-search

## Decisión arquitectónica

La funcionalidad de búsqueda vive en el paquete `store/sqlite/` como un archivo separado `fts5.go`, manteniendo separación de concerns con el CRUD base. No se añade una dependency externa: FTS5 viene integrado en SQLite desde 3.9.0 y Go's `mattn/go-sqlite3` lo soporta out of the box con `SQLITE_ENABLE_FTS5`.

```
store/
├── sqlite/
│   ├── store.go          # conexión, init, helpers
│   ├── crud.go           # Create/Read/Update/Delete observations
│   ├── fts5.go           # SearchObservations, SearchPrompts, sanitizeFTS5
│   ├── triggers.go       # FTS5 index maintenance triggers
│   └── store_test.go     # tests
├── interface.go          # Store interface con Search* methods
└── models.go             # Observation, Prompt, SearchResult
```

### Store Interface (añadir a `interface.go`)

```go
type SearchFilter struct {
    Type    string // "" means all
    Project string // "" means all
    Scope   string // "" means all
}

type SearchPagination struct {
    Limit  int // default 20
    Offset int // default 0
}

type SearchOpts struct {
    WithSnippets bool // compute snippet() for each result
    Filter       SearchFilter
    Pagination   SearchPagination
}

type SearchResult struct {
    Observation Observation
    Relevance   float64   `json:"relevance"`  // bm25 score
    Snippet     string    `json:"snippet,omitempty"`
}

type Store interface {
    // ... existing methods

    SearchObservations(ctx context.Context, query string, opts SearchOpts) ([]SearchResult, int, error)
    // returns (results, total_count, error)

    SearchPrompts(ctx context.Context, query string, pagination SearchPagination) ([]PromptSearchResult, int, error)
}
```

## FTS5 MATCH query construction

### SQL Pattern

```sql
SELECT o.id, o.title, o.content, o.type, o.project, o.scope, o.created_at,
       bm25(observations_fts) AS relevance
       {snippet_col}
FROM observations_fts fts
JOIN observations o ON o.id = fts.rowid
WHERE observations_fts MATCH ?
  AND o.deleted_at IS NULL
  {type_filter}
  {project_filter}
  {scope_filter}
ORDER BY relevance
LIMIT ? OFFSET ?
```

- `{snippet_col}`: si `WithSnippets=true`, añade `, snippet(observations_fts, 1, '<mark>', '</mark>', '...', 32) AS snippet`
- Cada filter se añade condicionalmente: si `Filter.Type != ""`, se incluye `AND o.type = ?` (con parámetro)
- `ORDER BY relevance` usa el ranking bm25 por defecto (lower = better match, se multiplica por -1 o se ordena ASC)

### Query Parameters

Los placeholders `?` se llenan en orden:
1. Query sanitizada (string MATCH)
2. Parámetros de filtro (type, project, scope) si presentes
3. Limit, Offset (int64)

## Sanitization: escape special FTS5 chars, wrap tokens in quotes

### Algoritmo `sanitizeFTS5`

FTS5 tiene caracteres especiales que causan syntax errors si aparecen sin quoting: `^ " * : ~ ( ) + -`

Pasos:

1. **Trim** whitespace. Si queda vacío, retorna error.
2. **Split** en tokens por espacios (strings.Fields).
3. Por cada token:
   a. Escape backslashes: `\` → `\\`
   b. Escape doble quote: `"` → `\"`
   c. Wrap en `"..."` → `"<token_escapado>"`
4. **Join** tokens con espacio.

```
Input:  "don't stop ^NEAR"
Tokens: ["don't", "stop", "^NEAR"]
Output: "\"don't\" \"stop\" \"^NEAR\""
```

Si el query raw contiene comillas dobles del usuario, se escapan internamente para que FTS5 las trate como parte del token y no como delimitador de frase.

### Manejo de errores

Si después de sanitizar la query queda vacía, retorna `ErrEmptyQuery`. El caller nunca ejecuta un MATCH con string vacío.

## Column filters via SQL WHERE clause on rowid join

Los filtros se aplican como condiciones `AND` en el `WHERE`, sobre columnas de la tabla `observations` (no de la FTS virtual). Esto es posible porque el JOIN es `fts.rowid = o.id`:

```go
func buildFilters(opts SearchOpts) (clause string, args []any) {
    var conditions []string
    if opts.Filter.Type != "" {
        conditions = append(conditions, "o.type = ?")
        args = append(args, opts.Filter.Type)
    }
    if opts.Filter.Project != "" {
        conditions = append(conditions, "o.project = ?")
        args = append(args, opts.Filter.Project)
    }
    if opts.Filter.Scope != "" {
        conditions = append(conditions, "o.scope = ?")
        args = append(args, opts.Filter.Scope)
    }
    if len(conditions) > 0 {
        clause = "AND " + strings.Join(conditions, " AND ")
    }
    return
}
```

Esto mantiene la query paramétrica (SQL injection safe) y delega el filtrado al planner de SQLite que usa el rowid index.

## Pagination with LIMIT/OFFSET

`LIMIT` es obligatorio (default 20, max 100). `OFFSET` default 0.

Se ejecuta una segunda query para `total_count`:

```sql
SELECT COUNT(*) FROM observations_fts fts
JOIN observations o ON o.id = fts.rowid
WHERE observations_fts MATCH ?
  AND o.deleted_at IS NULL
  {filters}
```

Esto permite mostrar "resultados 1-20 de 147" sin cargar todo.

## Snippet/highlight support

Controlado por `SearchOpts.WithSnippets`. Cuando es `true`, se añade la columna:

```sql
snippet(observations_fts, 1, '<mark>', '</mark>', '...', 32) AS snippet
```

El índice `1` indica columna `content` (posición 0 = title, 1 = content). Los tags `<mark>` son HTML semántico que el frontend puede estilar con CSS. Longitud máxima del fragmento: 32 tokens alrededor del match.

Para prompts:

```sql
snippet(prompts_fts, 0, '<mark>', '</mark>', '...', 32) AS snippet
```

## Index maintenance via triggers on observations table

Se requiere en el schema (HU-01.1), pero se documenta aquí la política:

```sql
-- After INSERT on observations
CREATE TRIGGER IF NOT EXISTS observations_ai AFTER INSERT ON observations BEGIN
    INSERT INTO observations_fts(rowid, title, content)
    VALUES (NEW.id, NEW.title, NEW.content);
END;

-- After DELETE on observations
CREATE TRIGGER IF NOT EXISTS observations_ad AFTER DELETE ON observations BEGIN
    INSERT INTO observations_fts(observations_fts, rowid, title, content)
    VALUES ('delete', OLD.id, OLD.title, OLD.content);
END;

-- After UPDATE of title or content on observations
CREATE TRIGGER IF NOT EXISTS observations_au AFTER UPDATE OF title, content ON observations BEGIN
    INSERT INTO observations_fts(observations_fts, rowid, title, content)
    VALUES ('delete', OLD.id, OLD.title, OLD.content);
    INSERT INTO observations_fts(rowid, title, content)
    VALUES (NEW.id, NEW.title, NEW.content);
END;
```

Triggers análogos para `user_prompts` → `prompts_fts`.

Nota: El `DELETE` trigger usa `INSERT INTO ... VALUES('delete', ...)` en lugar de `DELETE FROM` porque FTS5 requiere este approach de "delete command" para mantener los segment indexes consistentes.

## Query performance considerations

| Aspecto | Consideración |
|---------|---------------|
| **Índice FTS5** | Se usa `unicode61` tokenizer que soporta UTF-8. Content es la columna más pesada; considerar external content FTS5 si el dataset excede 100k rows. |
| **JOIN con observations** | El rowid join es directo (sin hash ni lookup). FTS5 rowid = observations.id, es O(1). |
| **Filtros** | `o.type`, `o.project`, `o.scope` no tienen índice propio en observaciones. Si estos filtros son cuello de botella, añadir índice compuesto: `CREATE INDEX idx_obs_type_project_scope ON observations(type, project, scope)`. |
| **bm25** | El cálculo de ranking es over HEAD para resultados grandes. Se calcula después del LIMIT? No — FTS5 necesita calcular bm25 para ordenar, pero internamente optimiza con el límite. |
| **total_count** | La query COUNT(*) es pesada en tablas grandes. Si > 10k resultados, considerar estimación o paginación infinita. |
| **Snippet()** | Se computa por fila. Con `WithSnippets=false` (default) no hay overhead. |

### Tokenizer choice

```sql
CREATE VIRTUAL TABLE observations_fts USING fts5(
    title, content,
    tokenize='unicode61 remove_diacritics 2',
    content='observations',
    content_rowid='id'
);
```

- `unicode61`: soporte UTF-8 completo, separa tokens por espacios/puntuación
- `remove_diacritics 2`: normaliza acentos (café → cafe) para matches más amplios
- `content=...` y `content_rowid=...`: external content FTS5 — la tabla virtual no almacena datos, solo el índice. Los datos viven en `observations`. Esto reduce storage a la mitad.

## Alternativas descartadas

1. **LIKE `%term%`**: No escala, no tiene ranking, no soporta stems. Descartado.
2. **Bleve**: Dependencia externa pesada para un store embebido. No justifica el overhead.
3. **SQLite FTS4**: Obsoleto, menos features que FTS5 (no tiene bm25, ni snippet tan flexible).
4. **PostgreSQL trigram index**: No aplica — el store es SQLite.

## Diagrama

```
 User query "error handler db"
         │
         ▼
  ┌──────────────┐
  │ sanitizeFTS5  │
  │ → tokens      │
  │ → escape      │
  │ → wrap quotes │
  │ → join        │
  └──────┬───────┘
         │ sanitized: "\"error\" \"handler\" \"db\""
         ▼
  ┌──────────────┐
  │ Build SQL     │
  │ MATCH ?       │
  │ + filters     │
  │ + pagination  │
  └──────┬───────┘
         ▼
  ┌──────────────┐
  │  SQLite FTS5  │
  │  + JOIN obs   │
  │  + bm25 rank  │
  └──────┬───────┘
         ▼
  ┌──────────────┐
  │ Parse rows → │
  │ SearchResult │
  └──────────────┘
```

## TDD plan

| Step | Test | Qué valida |
|------|------|------------|
| 1 | `TestSanitizeFTS5_Empty` | ErrEmptyQuery |
| 2 | `TestSanitizeFTS5_Normal` | `"hello" "world"` |
| 3 | `TestSanitizeFTS5_SpecialChars` | `"don't" "stop"` |
| 4 | `TestSanitizeFTS5_Quotes` | `"he said \"hi\""` |
| 5 | `TestSearchObservations_Basic` | keyword match title + content |
| 6 | `TestSearchObservations_ExcludesSoftDeleted` | deleted IS NULL |
| 7 | `TestSearchObservations_FilterByType` | solo type matching |
| 8 | `TestSearchObservations_FilterByProject` | solo project matching |
| 9 | `TestSearchObservations_FilterByScope` | solo scope matching |
| 10 | `TestSearchObservations_CombinedFilters` | type + project + scope |
| 11 | `TestSearchObservations_Pagination` | limit/offset funciona |
| 12 | `TestSearchObservations_Snippets` | snippet no vacío |
| 13 | `TestSearchObservations_EmptyQuery` | error |
| 14 | `TestSearchPrompts_Basic` | search en prompts |
| 15 | `TestTriggers_InsertUpdatesFTS` | trigger tras INSERT |
| 16 | `TestTriggers_UpdateTitleUpdatesFTS` | trigger tras UPDATE title |
| 17 | `TestTriggers_DeleteRemovesFromFTS` | trigger tras DELETE |
| 18 | `TestSearchObservations_RelevanceOrdered` | bm27 order |

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|-----------|
| FTS5 syntax error por special chars | sanitizeFTS5 wrap total todos los tokens |
| FTS5 trigger no sincronizado con UPDATE de title | Trigger `AFTER UPDATE OF title, content` |
| Soft-delete leak | `WHERE o.deleted_at IS NULL` en la query, no confiar solo en trigger |
| bm25 ranking no óptimo para el dominio | Default FTS5 bm25; se puede ajustar pesos por columna después |
| External content FTS5 no rebuild automático | Trigger cubre INSERT/UPDATE/DELETE; si hay migración manual, rebuild necesario |
