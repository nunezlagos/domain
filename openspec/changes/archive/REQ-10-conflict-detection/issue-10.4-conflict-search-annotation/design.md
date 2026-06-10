# Design: issue-10.4-conflict-search-annotation

## Decisión arquitectónica

### ConflictAnnotation

```go
type ConflictAnnotation struct {
    Relation       string  `json:"relation"`        // supersedes, conflicts_with, duplicate, candidate
    Direction      string  `json:"direction"`       // "outgoing" (source), "incoming" (target)
    TargetID       int64   `json:"target_id"`
    TargetTitle    string  `json:"target_title,omitempty"`
    JudgmentStatus string  `json:"judgment_status"` // pending, judged, error
    Confidence     float64 `json:"confidence,omitempty"`
}
```

### SearchResult enrichment

```go
type SearchResult struct {
    Observation Observation          `json:"observation"`
    Rank        float64              `json:"rank"`
    Conflicts   []ConflictAnnotation `json:"conflicts,omitempty"`
}
```

### N+1-safe approach: batch query after search

```go
func enrichWithConflicts(ctx context.Context, db *sql.DB, results []SearchResult) error {
    if len(results) == 0 { return nil }

    // Collect all observation IDs
    ids := make([]int64, len(results))
    for i, r := range results {
        ids[i] = r.Observation.ID
    }

    // Single query for all relations involving these observations
    query := `SELECT mr.source_id, mr.target_id, mr.relation, mr.judgment_status,
                     mr.confidence, o.title as target_title
              FROM memory_relations mr
              LEFT JOIN observations o ON o.id = mr.target_id
              WHERE (mr.source_id IN (<placeholders>) OR mr.target_id IN (<placeholders>))
                AND mr.relation != 'candidate'  -- skip pure candidates unless pending
              ORDER BY mr.source_id`

    // Build query with placeholders
    placeholders := make([]string, len(ids))
    args := make([]interface{}, 0, len(ids)*2)
    for i, id := range ids {
        placeholders[i] = "?"
        args = append(args, id)
    }
    inClause := strings.Join(placeholders, ",")
    query = strings.Replace(query, "<placeholders>", inClause, 2)

    rows, _ := db.QueryContext(ctx, query, append(args, args...)...)
    defer rows.Close()

    // Build annotation map: observation_id → []ConflictAnnotation
    annotations := make(map[int64][]ConflictAnnotation)
    for rows.Next() {
        var sourceID, targetID int64
        var relation, judgmentStatus string
        var confidence float64
        var targetTitle sql.NullString

        rows.Scan(&sourceID, &targetID, &relation, &judgmentStatus, &confidence, &targetTitle)

        // Annotate both source and target if relevant
        for _, obsID := range []int64{sourceID, targetID} {
            direction := "outgoing"
            if obsID == targetID {
                direction = "incoming"
            }
            annotations[obsID] = append(annotations[obsID], ConflictAnnotation{
                Relation:       relation,
                Direction:      direction,
                TargetID:       targetID,
                TargetTitle:    targetTitle.String,
                JudgmentStatus: judgmentStatus,
                Confidence:     confidence,
            })
        }
    }

    // Assign to results
    for i, r := range results {
        if anns, ok := annotations[r.Observation.ID]; ok {
            results[i].Conflicts = anns
        }
    }
    return nil
}
```

### Alternative: integrated JOIN (more efficient but complex)

```go
// Alternative: single query with JSON aggregation
// Requires SQLite JSON1 extension (usually available)
query := `
    SELECT o.*, f.rank,
           json_group_array(
               json_object(
                   'relation', mr.relation,
                   'direction', CASE WHEN mr.source_id = o.id THEN 'outgoing' ELSE 'incoming' END,
                   'target_id', CASE WHEN mr.source_id = o.id THEN mr.target_id ELSE mr.source_id END,
                   'judgment_status', mr.judgment_status,
                   'confidence', mr.confidence
               )
           ) FILTER (WHERE mr.id IS NOT NULL) as conflicts
    FROM observations_fts f
    JOIN observations o ON o.id = f.rowid
    LEFT JOIN memory_relations mr ON (mr.source_id = o.id OR mr.target_id = o.id)
    WHERE observations_fts MATCH ?
      AND o.deleted_at IS NULL
    GROUP BY o.id
    ORDER BY rank DESC
    LIMIT ? OFFSET ?`
```

Se elige el enfoque batch query (separado) por claridad y menor riesgo de errores con JSON aggregation. Si performance es problema, migrar a integrated JOIN.

### Search function signature

```go
func SearchWithConflicts(ctx context.Context, db *sql.DB, query string, limit, offset int) ([]SearchResult, int, error) {
    // 1. Execute normal search (FTS5)
    // 2. Get total count
    // 3. Enrich with conflicts via batch query
    // 4. Return enriched results
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Subquery por cada resultado (N+1) | Ineficiente; batch query es una sola query adicional |
| JSON aggregation en SQL | Más complejo; requiere JSON1; batch query es más portable |
| Precargar todas las relations | Ineficiente si hay millones de relations; batch por página es mejor |

## TDD plan

1. **Red:** SearchWithConflicts retorna annotations en resultados → falla
2. **Green:** Implement batch enrichWithConflicts → pasa
3. **Red:** Observation sin conflicts tiene nil → falla
4. **Green:** Implement empty check → pasa
5. **Red:** Duplicación de resultados por múltiples relations → verificar sin dups
6. **Green:** Implement proper grouping → pasa
7. **Sabotaje:** No excluir candidate relations → annotations incluyen candidates → test falla → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| JSON1 no disponible en SQLite | Batch query approach no requiere JSON1; es compatible con cualquier build |
| LEFT JOIN + GROUP BY lento con muchas relations | Índice en (source_id, target_id); batch query con IN clause limitado a page size |
| Anotaciones inconsistentes (source vs target desincronizado) | Siempre mirror: si A relation B, aparece en annotations de A y B |
