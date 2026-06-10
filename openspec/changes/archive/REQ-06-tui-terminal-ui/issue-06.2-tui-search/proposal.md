# Proposal: issue-06.2-tui-search

## Intención

Que el usuario pueda buscar observaciones con FTS5 desde el TUI, con un input accesible vía '/' desde cualquier vista, resultados con snippets navegables, y transición a detalle. Sin esto, el TUI sería solo un visor sin capacidad de búsqueda.

## Scope

**Incluye:**
- Overlay de input de búsqueda accesible con '/' global
- Consulta FTS5 vía store: `SearchFTS5(query)` con filtro por proyecto
- Lista de resultados con: título, snippet (match resaltado con color amarillo), tipo, proyecto, fecha
- Navegación j/k en resultados
- Enter → navega a detalle de observación (issue-06.3)
- ESC → cierra búsqueda
- Mensaje "No results" para queries sin match
- Búsqueda vacía es no-op

**No incluye:**
- Filtros avanzados (type, scope, project) — búsqueda básica inicial
- Historial de búsquedas
- Resultados paginados
- Búsqueda por topic_key o tags
- Exportar resultados

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Input | `tea.Model` con `tea.KeyMsg` handling; texto en `tea.TextInput` de bubbles o custom |
| Consulta | Store method `SearchFTS5(ctx, query, limit, offset)` → `[]SearchResult` |
| Resultados | Slice con struct `{Title, Snippet, Type, Project, CreatedAt}` |
| Snippet | FTS5 `snippet()` function con `...` truncado; match resaltado con lipgloss bold+yellow |
| Navegación | Cursor int + j/k/↑↓; wrap-around opcional |
| Transición | Al presionar Enter, envía `ObservationSelected{id}` al mainModel |

```go
type searchModel struct {
    input       string
    active      bool       // overlay visible?
    results     []SearchResult
    cursor      int
    loading     bool
    store       Store
}

type SearchResult struct {
    ID        int64
    Title     string
    Snippet   string
    Type      string
    Project   string
    CreatedAt string
    Score     float64
}
```

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| FTS5 query malformada causa crash | Media | Sanitizar input: escapar caracteres especiales FTS5, limitar longitud |
| Resultados muy lentos con DB grande | Baja | LIMIT 50 por defecto; indicador de loading async |
| Snippet muy largo rompe layout | Media | Truncar snippet a 120 chars con "..." |
| '/' conflict con otros keybindings | Media | '/' global capturado en mainModel antes de delegar a sub-modelos |

## Testing

- **Unitario:** Test de toggle de input con '/'
- **Unitario:** Test de navegación j/k en resultados
- **Unitario:** Test que búsqueda vacía es no-op
- **Integración:** Test con store mockeado que ejecuta SearchFTS5 y procesa resultados
- **Manual:** Escribir query, ver snippets, navegar, Enter a detalle, ESC volver
