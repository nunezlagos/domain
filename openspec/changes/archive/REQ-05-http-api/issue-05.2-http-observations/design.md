# Design: issue-05.2-http-observations

## Decisión arquitectónica

### ObservationRepo interface

```go
type Observation struct {
    ID             int     `json:"id"`
    SessionID      string  `json:"session_id"`
    Type           string  `json:"type"`
    Title          string  `json:"title"`
    Content        string  `json:"content"`
    ToolName       string  `json:"tool_name"`
    Project        string  `json:"project"`
    Scope          string  `json:"scope"`
    TopicKey       *string `json:"topic_key,omitempty"`
    NormalizedHash *string `json:"normalized_hash,omitempty"`
    RevisionCount  int     `json:"revision_count"`
    DuplicateCount int     `json:"duplicate_count"`
    LastSeenAt     *string `json:"last_seen_at,omitempty"`
    CreatedAt      string  `json:"created_at"`
    UpdatedAt      string  `json:"updated_at"`
}

type ObservationRepo interface {
    Create(ctx context.Context, obs Observation) (Observation, *ConflictCandidate, error)
    CreatePassive(ctx context.Context, obs Observation) (Observation, error)
    Recent(ctx context.Context, filter ObservationFilter) ([]Observation, error)
    GetByID(ctx context.Context, id int) (Observation, error)
    Update(ctx context.Context, obs Observation) (Observation, error)
    SoftDelete(ctx context.Context, id int) error
    HardDelete(ctx context.Context, id int) error
}
```

### Conflict detection strategy

El conflict detection se implementa calculando el `normalized_hash` del nuevo contenido (usando `internal/dedup/` de issue-01.4) y buscando observaciones existentes con el mismo hash. Si se encuentra una coincidencia:

```go
type ConflictCandidate struct {
    ExistingID       int     `json:"existing_id"`
    ExistingTitle    string  `json:"existing_title"`
    SimilarityScore  float64 `json:"similarity_score"`
    ExistingContent  string  `json:"existing_content,omitempty"`
}
```

El `similarity_score` se calcula como:
- 1.0 si el `normalized_hash` coincide exactamente
- 0.0 a 1.0 si usamos un algoritmo más fino (Jaccard similarity sobre tokens) para el futuro

El POST nunca rechaza la creación; solo **advierte** con el campo `conflict_candidate` en la respuesta.

### Soft delete pattern

```sql
-- Soft delete marca el registro como eliminado
UPDATE observations SET deleted_at = datetime('now') WHERE id = ? AND deleted_at IS NULL

-- Todas las queries de lectura filtran deleted
SELECT * FROM observations WHERE deleted_at IS NULL AND id = ?

-- Hard delete es físico
DELETE FROM observations WHERE id = ?
```

### PATCH merge semantics

```go
func applyPatch(original Observation, patch map[string]any) Observation {
    if v, ok := patch["title"]; ok { original.Title = v.(string) }
    if v, ok := patch["content"]; ok { original.Content = v.(string) }
    if v, ok := patch["type"]; ok { original.Type = v.(string) }
    if v, ok := patch["project"]; ok { original.Project = v.(string) }
    if v, ok := patch["scope"]; ok { original.Scope = v.(string) }
    if v, ok := patch["topic_key"]; ok { original.TopicKey = strPtr(v.(string)) }
    if v, ok := patch["revision_count"]; ok { original.RevisionCount = int(v.(float64)) }
    original.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
    return original
}
```

El PATCH recibe un JSON plano, mergea solo los campos presentes. Los campos no enviados no se modifican.

### Route registration

```go
func RegisterObservationRoutes(mux *http.ServeMux, repo ObservationRepo) {
    mux.HandleFunc("POST /observations", handleCreateObservation(repo))
    mux.HandleFunc("POST /observations/passive", handleCreatePassive(repo))
    mux.HandleFunc("GET /observations/recent", handleRecentObservations(repo))
    mux.HandleFunc("GET /observations/{id}", handleGetObservation(repo))
    mux.HandleFunc("PATCH /observations/{id}", handleUpdateObservation(repo))
    mux.HandleFunc("DELETE /observations/{id}", handleDeleteObservation(repo))
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Rechazar POST con conflict (409) | El conflict es advertencia, no error; el usuario decidió crear la observación igual |
| Hard delete como default | Soft delete es más seguro para recoverability; hard requiere query param explícito |
| GraphQL mutations | Overkill para CRUD simple; REST es más directo y testeable |
| PUT en lugar de PATCH | PUT requiere el objeto completo; PATCH es semánticamente parcial |

## Diagrama

```
Client HTTP                          memoria server
    |                                       |
    | POST /observations                     |
    |   +-- conflict detection -------------> dedup.Normalize()
    |   +-- INSERT observation -------------> store.Create()
    |   +-- response with conflict_candidate |
    |                                       |
    | POST /observations/passive             |
    |   +-- INSERT (no conflict check) -----> store.CreatePassive()
    |                                       |
    | GET  /observations/recent              |
    | GET  /observations/{id}               |
    | PATCH /observations/{id}               |
    | DELETE /observations/{id}[?hard=true]  |
    |                                       |
    +--------> api/observations.go -------> store/observation.go
                                                |
                                            SQLite DB
                                                |
                                        observations table
```

## TDD plan

1. **Red:** Test POST /observations → 201 → falla
2. **Green:** Handler inserta en DB → pasa
3. **Red:** Test POST idéntico → conflict_candidate en response → falla
4. **Green:** Agregar normalized_hash lookup → pasa
5. **Red:** Test GET /observations/recent → array DESC → falla
6. **Green:** Handler con query SQL + filtros → pasa
7. **Red:** Test PATCH → 200, title cambiado → falla
8. **Green:** Implementar merge + UPDATE → pasa
9. **Red:** Test DELETE → 204, GET posterior → 404 → falla
10. **Green:** Soft delete con `deleted_at` → pasa
11. **Red:** Test `?hard=true` → DELETE físico → pasa
12. **Red:** Test POST /observations/passive → 201 sin conflict → falla
13. **Green:** Handler sin normalized_hash check → pasa
14. **Sabotaje:** Sacar `WHERE deleted_at IS NULL` de GET → muestra borradas → test cae → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Conflict detection lento en DB grande | normalized_hash tiene índice (idx_obs_hash); query es O(log n) |
| Soft-delete acumula registros huérfanos | Hard delete explícito + futura tarea de vacuum |
| PATCH actualiza updated_at aunque no cambie nada | Comparar campos antes de UPDATE; skip si no hay cambios |
| POST sin session_id da FK error feo | Validación temprana en handler → 400 con mensaje claro |
