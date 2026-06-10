# Design: issue-05.3-http-search-context

## Decisión arquitectónica

### SearchRepo interface

```go
type SearchRepo interface {
    Search(ctx context.Context, query string, filter SearchFilter) ([]SearchResult, error)
    Timeline(ctx context.Context, obsID int, before, after int) (TimelineResponse, error)
    Context(ctx context.Context, project, scope string) ([]Observation, error)
}

type SearchFilter struct {
    Type    string
    Project string
    Scope   string
    Limit   int
}
```

### FTS5 search query pattern

```go
func (r *searchRepo) Search(ctx context.Context, query string, filter SearchFilter) ([]SearchResult, error) {
    // Sanitize FTS5 query
    safeQuery := sanitizeFTS5(query)

    // Build query with optional filters
    q := `SELECT o.id, o.title, o.content, o.type, o.project, o.scope, o.tool_name, o.created_at,
                 rank
          FROM observations_fts
          JOIN observations o ON o.id = observations_fts.rowid
          WHERE observations_fts MATCH ? AND o.deleted_at IS NULL`

    args := []any{safeQuery}

    if filter.Type != "" {
        q += " AND o.type = ?"
        args = append(args, filter.Type)
    }
    if filter.Project != "" {
        q += " AND o.project = ?"
        args = append(args, filter.Project)
    }
    if filter.Scope != "" {
        q += " AND o.scope = ?"
        args = append(args, filter.Scope)
    }

    q += " ORDER BY rank DESC"

    if filter.Limit > 0 {
        q += " LIMIT ?"
        args = append(args, filter.Limit)
    } else {
        q += " LIMIT 20"
    }

    rows, _ := r.db.QueryContext(ctx, q, args...)
    defer rows.Close()
    // scan rows into []SearchResult
}
```

### FTS5 sanitizer

```go
func sanitizeFTS5(q string) string {
    // Remove/replace FTS5 special characters: ^ * " ( )
    re := regexp.MustCompile(`[^\w\sáéíóúüñ]`)
    q = re.ReplaceAllString(q, " ")
    // Collapse multiple spaces
    q = strings.Join(strings.Fields(q), " ")
    if q == "" { return "" }
    // Wrap each word for prefix matching
    terms := strings.Split(q, " ")
    for i, t := range terms {
        terms[i] = t + "*"
    }
    return strings.Join(terms, " ")
}
```

### Timeline query

```go
func (r *searchRepo) Timeline(ctx context.Context, obsID int, before, after int) (TimelineResponse, error) {
    // Verify center exists
    center, err := r.GetByID(ctx, obsID)
    if err != nil { return TimelineResponse{}, err }

    var t TimelineResponse
    t.Center = center

    // Before: observations with id < center, ordered DESC
    if before > 0 {
        rows, _ := r.db.QueryContext(ctx,
            `SELECT ... FROM observations
             WHERE id < ? AND deleted_at IS NULL
             ORDER BY id DESC LIMIT ?`, obsID, before)
        t.Before = scanObservations(rows)
    }

    // After: observations with id > center, ordered ASC
    if after > 0 {
        rows, _ := r.db.QueryContext(ctx,
            `SELECT ... FROM observations
             WHERE id > ? AND deleted_at IS NULL
             ORDER BY id ASC LIMIT ?`, obsID, after)
        t.After = scanObservations(rows)
    }

    return t, nil
}
```

### Route registration

```go
func RegisterSearchRoutes(mux *http.ServeMux, repo SearchRepo) {
    mux.HandleFunc("GET /search", handleSearch(repo))
    mux.HandleFunc("GET /timeline", handleTimeline(repo))
    mux.HandleFunc("GET /context", handleContext(repo))
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| LIKE search en vez de FTS5 | LIKE no usa índices, performance O(n) en DB grande; FTS5 es O(log n) con ranking |
| ElasticSearch externo | Overkill para DB local embebida; FTS5 de SQLite es suficiente |
| Timeline vía cursor pagination | IDs secuenciales hacen offset más simple; paginación por cursor se puede agregar después |

## Diagrama

```
Client HTTP                             memoria server (localhost:7437)
    |                                          |
    | GET /search?q=&type=&project=&scope=      |
    |   +-- FTS5 MATCH query -----------------> observations_fts
    |   +-- JOIN observations for filters -----> observations
    |   +-- WHERE deleted_at IS NULL            |
    |                                          |
    | GET /timeline?observation_id=&before=     |
    |   +-- center observation ---------------> GetByID
    |   +-- before/after queries -------------> observations (id < / id >)
    |                                          |
    | GET /context?project=&scope=              |
    |   +-- SELECT with project filter -------> observations
    |                                          |
    +--------> api/search.go ---------------> store/search.go
                                                  |
                                              SQLite DB
```

## TDD plan

1. **Red:** Test GET /search?q=hello → 200 → falla
2. **Green:** Handler con MATCH query → pasa
3. **Red:** Test GET /search → 400 (sin q) → falla
4. **Green:** Validar query presente → pasa
5. **Red:** Test search con filtro type → solo type=decision → falla
6. **Green:** WHERE dinámico con args → pasa
7. **Red:** Test GET /timeline?observation_id=1&before=2 → center + 2 before → falla
8. **Green:** Timeline query con id < / id > → pasa
9. **Red:** Test timeline 404 → falla
10. **Green:** GetByID check → pasa
11. **Red:** Test GET /context?project=myapp → array → falla
12. **Green:** Context query simple → pasa
13. **Sabotaje:** Sacar deleted_at filter de search → muestra borradas → test cae → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| FTS5 query malformed por input user | Sanitizer remueve caracteres especiales; si query queda vacía → 400 |
| Timeline con IDs no secuenciales (hard delete gaps) | Usar ORDER BY id + LIMIT; gaps no afectan la semántica "N anteriores" |
| Context sin scope devuelve demasiados | Scope default "project"; si no se pasa scope incluir todos |
| Search sin resultados | Array vacío, no error |
