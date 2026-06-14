# Proposal: issue-13.3-http-pagination-filters

## Intención

Implementar un sistema de paginación (offset y cursor), ordenamiento dinámico, filtros por campo y búsqueda full-text que se aplique a todos los endpoints de listado GET /api/v1/{entity}. El parseo de query params y construcción de SQL es genérico para reutilizarse entre entidades.

## Scope

**Incluye:**
- Query param parser: `?offset=`, `?limit=`, `?cursor=`, `?sort=`, `?q=`, filtros dinámicos `?campo=valor`
- Paginación offset-based: OFFSET + LIMIT, total count
- Paginación cursor-based: keyset pagination con cursor codificado en base64
- Ordenamiento dinámico: sort por uno o múltiples campos, ASC/DESC con prefijo `-`
- Filtros exactos: WHERE campo = valor, múltiples filtros con AND
- Filtros range: `?created_at[gte]=2024-01-01&created_at[lte]=2024-12-31`
- Búsqueda full-text: tsquery sobre índice GIN, con ranking y highlights
- Whitelist de campos permitidos para sort y filter por entidad
- Response envelope estándar: `{ data, pagination }`

**Excluye:**
- Filtros OR (complejidad extra, se puede lograr con ?q=)
- Aggregations / group by (futuro, otro endpoint)
- Pagination en mutaciones (POST/PUT/PATCH/DELETE no pagan)

## Enfoque técnico

**Query parameter parser:**
```go
type ListParams struct {
    Offset int                    `query:"offset"`
    Limit  int                    `query:"limit" default:"20" maximum:"100"`
    Cursor string                 `query:"cursor"`
    Sort   []SortField            `query:"sort"`  // ["-created_at", "type"]
    Filters map[string]string     `query:"-"`      // parsed from remaining params
    RangeFilters map[string]Range `query:"-"`      // campo[gte]=x&campo[lte]=y
    Search string                 `query:"q"`
}

type SortField struct {
    Field string
    Desc  bool
}
```

**SQL Builder genérico:**
```go
type QueryBuilder struct {
    table      string
    allowedSort  []string  // whitelist
    allowedFilter []string // whitelist
}

func (qb *QueryBuilder) BuildListQuery(params ListParams) (query string, args []any, err error) {
    // SELECT * FROM table
    // WHERE filters (AND)
    //   AND tsvector_col @@ plainto_tsquery('spanish', q)  (si q != "")
    // ORDER BY sort_fields
    // LIMIT limit OFFSET offset (offset-based)
    // OR WHERE id > cursor_last_id ORDER BY id LIMIT limit (cursor-based)
}
```

**Cursor encoding:**
```go
type Cursor struct {
    LastID    string            `json:"last_id"`
    LastValue map[string]any   `json:"last_value,omitempty"` // for multi-field sort
    Sort      []SortField      `json:"sort"`
    Filters   map[string]string `json:"filters"`
}

// Encode: base64(json(cursor))
// Decode: json(base64(cursor_string))
```

**Response:**
```go
type PaginationMeta struct {
    Offset   int    `json:"offset,omitempty"`
    Limit    int    `json:"limit"`
    Total    int    `json:"total"`
    Cursor   string `json:"cursor,omitempty"`
    HasMore  bool   `json:"has_more"`
}
```

## Riesgos

| Riesgo | Mitigación |
|--------|------------|
| SQL injection en sort/filter dinámicos | Whitelist estricta de campos permitidos, nunca concatenar raw input |
| Keyset pagination con sort no-único: resultados duplicados/saltados | Siempre incluir campo único (id) como tiebreaker en ORDER BY |
| Total count en tablas grandes es caro | Usar pg_stats_approx o estimación, o mostrar ">1000" |
| FTS en tablas sin índice GIN: full scan | Validar índice GIN existe en migration; query lento si no |

## Testing

- Unit: query builder genera SQL correcto
- Integration: 100 rows, testear offset y cursor pagination devuelven mismos resultados
- FTS: tsquery con stemming español
- Sabotaje: sort por campo no whitelisted → error
