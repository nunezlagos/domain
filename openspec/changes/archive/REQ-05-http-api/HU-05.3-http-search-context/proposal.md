# Proposal: HU-05.3-http-search-context

## Intención

Exponer 3 endpoints de consulta: búsqueda full-text vía FTS5, timeline contextual alrededor de una observación, y retrieval de observaciones por proyecto/scope. Estos endpoints son críticos para que agentes de IA y herramientas externas puedan recuperar memoria relevante.

## Scope

**Incluye:**
- `GET /search?q=&type=&project=&scope=&limit=` — FTS5 search con filtros combinados
- `GET /timeline?observation_id=&before=&after=` — contexto cronológico alrededor de una observación
- `GET /context?project=&scope=` — todas las observaciones de un proyecto (filtro scope opcional)
- Respuestas JSON consistentes
- Exclusión de soft-deleted en todos los resultados

**No incluye:**
- CRUD de observaciones (HU-05.2)
- Merge de proyectos (HU-05.7)
- Autenticación (HU-05.9)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Search engine | FTS5 via `observations_fts` con `MATCH ?` |
| Query sanitization | Escape FTS5 special chars; si query vacía → 400 |
| Filtros combinados | WHERE clause adicional sobre JOIN con observations table |
| Timeline | Query con `id < center_id ORDER BY id DESC LIMIT before` + `id > center_id ORDER BY id ASC LIMIT after` |
| Context | Query simple `SELECT ... WHERE project = ? AND scope = ? AND deleted_at IS NULL` |

```go
type SearchResult struct {
    ID        int     `json:"id"`
    Title     string  `json:"title"`
    Content   string  `json:"content"`
    Type      string  `json:"type"`
    Project   string  `json:"project"`
    Scope     string  `json:"scope"`
    ToolName  string  `json:"tool_name"`
    CreatedAt string  `json:"created_at"`
    Rank      float64 `json:"rank"`
}

type TimelineResponse struct {
    Center Observation   `json:"center"`
    Before []Observation `json:"before"`
    After  []Observation `json:"after"`
}
```

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| FTS5 syntax error con caracteres especiales | Media | Sanitizar query: escapar " * - etc.; si falla, retornar error claro |
| Timeline sin before/after devuelve todo | Baja | Default before=0, after=0 → arrays vacíos |
| Search sin índice FTS5 | Baja | Dependencia de HU-01.1; el schema debe tener FTS5 configurado |

## Testing

- **Search:** GET /search?q=hello → 200, array con resultados
- **Search empty:** GET /search → 400
- **Search filters:** GET /search?q=hello&type=decision&project=myapp → filtrado
- **Search limit:** GET /search?q=hello&limit=3 → max 3
- **Timeline:** GET /timeline?observation_id=1&before=2&after=2 → center + 2+2
- **Timeline 404:** GET /timeline?observation_id=9999 → 404
- **Context:** GET /context?project=myapp → array de observaciones
- **Context 400:** GET /context → 400
- **Context no deleted:** soft-delete → no aparece en context
