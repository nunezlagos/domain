# Design: issue-13.3-http-pagination-filters

## Decisión arquitectónica

**Paginator interface con dos implementaciones:**

```go
type Paginator interface {
    Apply(query *gorm.DB, params ListParams) (*gorm.DB, *PaginationMeta, error)
}

type OffsetPaginator struct{}
type CursorPaginator struct{}
```

Se elige automáticamente según si viene `?cursor=` (cursor) o `?offset=` (offset). Default: offset (más simple, más conocido).

**Whitelist por entidad:** Cada entidad declara qué campos son sorteables y filtrables:

```go
type EntityQueryConfig struct {
    SortableFields  []string // campos permitidos en ?sort=
    FilterableFields []string // campos permitidos en ?campo=valor
    SearchFields    []string // columnas para tsquery (por defecto: title, content)
    DefaultSort     string  // sort por defecto
    MaxLimit        int     // máximo resultados por página
}
```

**Cursor internals:**

El cursor es keyset pagination: en lugar de OFFSET (que salta filas, ineficiente en tablas grandes), usamos `WHERE (sort_col, id) > (last_val, last_id)`. El cursor base64 contiene:
- Últimos valores vistos de las columnas sort
- El id del último registro (tiebreaker)
- Los parámetros sort/filters originales (para reconstruir misma query)

**Ejemplo de query generada:**

Offset-based:
```sql
SELECT * FROM observations
WHERE type = $1 AND project = $2
  AND tsvector_col @@ plainto_tsquery('spanish', $3)
ORDER BY created_at DESC, id DESC
LIMIT $4 OFFSET $5
```

Cursor-based:
```sql
SELECT * FROM observations
WHERE type = $1 AND project = $2
  AND tsvector_col @@ plainto_tsquery('spanish', $3)
  AND (created_at, id) < ($4, $5)  -- keyset
ORDER BY created_at DESC, id DESC
LIMIT $6
```

**Full-text search:**
- Usar `plainto_tsquery('spanish', ?)` para query simple (tokeniza y pone AND entre términos)
- Usar `to_tsvector('spanish', coalesce(title, '') || ' ' || coalesce(content, ''))` para el vector
- Ranking con `ts_rank(tsvector_col, query)` 
- Highlight con `ts_headline('spanish', content, query, 'MaxWords=30, MinWords=15')`
- Índice GIN sobre tsvector_col

## Alternativas descartadas

1. **Solo cursor pagination:** Más eficiente pero menos conocido. Offset es más intuitivo para consumidores. Soportamos ambos.
2. **GraphQL Connections spec:** Sobredimensionado. Nuestro cursor simple base64 es suficiente.
3. **Filtros tipo OData ($filter):** Muy complejo de parsear y validar. Filtros simples `campo=valor` cubren 90% de casos.
4. **Pagination via Link headers:** Menos visible que response envelope. El envelope es más explícito.

## Diagrama

```
GET /api/v1/observations?sort=-created_at&type=fix&limit=10&offset=0
  │
  ▼
┌──────────────────────────────┐
│  ParseQueryParams(r.URL.Query) │
│  → ListParams{Sort, Filters,  │
│     Offset, Limit, Search}    │
└──────────┬───────────────────┘
           ▼
┌──────────────────────────────┐
│  ValidateParams(params, config) │
│  → Error si sort/filter no    │
│    permitido                  │
└──────────┬───────────────────┘
           ▼
┌──────────────────────────────┐
│  QueryBuilder.Build(query,    │
│    params, config)            │
│  → SQL query + args           │
└──────────┬───────────────────┘
           ▼
┌──────────────────────────────┐
│  Store.Exec(query, args)      │
│  → []Observation + totalCount │
└──────────┬───────────────────┘
           ▼
┌──────────────────────────────┐
│  BuildResponse(data, params,  │
│    total, config)             │
│  → {data, pagination}         │
└──────────────────────────────┘
```

## TDD plan

1. **Red:** Test `TestOffsetPagination` con 30 rows, limit=10, offset=0
2. **Green:** Implementar OffsetPaginator básico
3. **Refactor:** Extraer QueryParser y QueryBuilder
4. **Iterar:** CursorPaginator, Sort, Filters, FTS
5. **Sabotaje:** Offset sin count total → test detecta

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Cursor con sort cambia entre requests (datos nuevos insertados) | Cursor es snapshot consistente: incluye filtros originales en encoding |
| FTS lento en tablas grandes | GIN index obligatorio, explain analyze en migration |
| Offset es ineficiente para datasets grandes | Documentar que para >10000 rows usar cursor |
| Sort por campo sin índice = sequential scan | Config para advertir si sort field no tiene índice |
