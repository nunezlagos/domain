# Tasks: issue-10.4-conflict-search-annotation

## Backend

- [ ] **B1: Definir ConflictAnnotation struct**
      - `internal/conflict/annotation.go`

- [ ] **B2: Implementar enrichWithConflicts batch query**
      - Query memory_relations con IN clause usando IDs de resultados
      - LEFT JOIN observations para target_title
      - Mapear source_id y target_id a annotations
      - Direction: outgoing (source), incoming (target)

- [ ] **B3: Modificar Search() para incluir enrich**
      - `SearchWithConflicts(ctx, db, query, limit, offset)`
      - Ejecutar search normal
      - Enriquecer con batch query
      - Retornar SearchResult con Conflicts field

- [ ] **B4: Manejar paginación consistente**
      - Total count no se afecta por JOIN (COUNT aparte)
      - Annotations en cada página

- [ ] **B5: Filtrar candidate relations no juzgadas**
      - Incluir solo: supersedes, conflicts_with, duplicate
      - Incluir "candidate" solo si judgment_status = "pending"

## Tests

- [ ] **T1: Search results incluyen conflict annotations**
      ```go
      func TestSearchWithConflicts(t *testing.T) {
          db := setupTestDB(t)
          seedObservation(db, 1, "server error")
          seedObservation(db, 2, "server failure")
          insertRelation(db, 1, 2, "duplicate", "judged", 0.95)
          results, _, err := SearchWithConflicts(context.Background(), db, "server", 10, 0)
          assert.NoError(t, err)
          assert.Len(t, results[0].Conflicts, 1)
          assert.Equal(t, "duplicate", results[0].Conflicts[0].Relation)
      }
      ```

- [ ] **T2: Observation sin conflicts tiene Conflicts nil**
- [ ] **T3: Batch query es N+1-safe (1 query adicional, no N)**
- [ ] **T4: Annotations incluyen target_title snippet**
- [ ] **T5: Candidate pending se incluye como annotation**
- [ ] **T6: Candidate judged no aparece (solo supersedes/conflicts_with/duplicate)**
- [ ] **T7: Paginación mantiene annotations**
- [ ] **T8: Sabotaje — no excluir judged candidates → annotations incorrectas → test falla → restaurar**

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/conflict/... -v`
- [ ] `go test ./internal/store/... -v`
- [ ] Commit: `feat: conflict annotations in search results via N+1-safe batch query`
