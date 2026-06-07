# Proposal: HU-10.4-conflict-search-annotation

## Intención

Enriquecer los resultados de búsqueda (FTS5) con anotaciones de conflicto desde memory_relations, usando LEFT JOIN para evitar N+1 queries. Las anotaciones incluyen relación, confidence, status y target snippet.

## Scope

**Incluye:**
- Enriquecimiento de SearchResult con ConflictAnnotation
- Query FTS5 + LEFT JOIN memory_relations (N+1-safe)
- Agrupación de múltiples relations por observation
- Metadata: relation, confidence, judgment_status, target title snippet
- Campo `conflicts` en SearchResult (nil si no hay)

**No incluye:**
- Modificar el índice FTS5 (solo JOIN adicional)
- Conflict detection o judgment (HU-10.1, HU-10.2)
- CLI/HTTP changes (HU-10.3)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Annotation query | LEFT JOIN memory_relations ON (source_id = obs.id OR target_id = obs.id) después de la search query principal |
| Evitar N+1 | Una sola query adicional con los IDs de los resultados; o integrar en la query principal |
| Duplicación | GROUP BY obs.id con JSON aggregation de relations |
| Response | `conflicts []ConflictAnnotation` en SearchResult |

