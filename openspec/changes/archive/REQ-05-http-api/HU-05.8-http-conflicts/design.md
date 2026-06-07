# Design: HU-05.8-http-conflicts

## Decisión arquitectónica

### ConflictRepo interface

```go
type ConflictRepo interface {
    List(ctx context.Context) ([]ConflictGroup, error)
    Judge(ctx context.Context, ids []int, model string) ([]Judgment, error)
    Compare(ctx context.Context, idA, idB int) (Comparison, error)
    GetByID(ctx context.Context, id int) (Conflict, error)
    Stats(ctx context.Context) (ConflictStats, error)
    Scan(ctx context.Context, project string) (ScanResult, error)
    ListDeferred(ctx context.Context) ([]DeferredItem, error)
    ReplayDeferred(ctx context.Context) (ReplayResult, error)
}
```

### Conflict group structure

```go
type ConflictGroup struct {
    NormalizedHash string        `json:"normalized_hash"`
    Count          int           `json:"count"`
    Observations   []Observation `json:"observations"`
    Status         string        `json:"status"` // "pending", "judged", "resolved"
}
```

### Delegation to REQ-10

Los handlers son thin wrappers que delegan en los módulos de REQ-10:

```go
func handleScan(repo ConflictRepo) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var req struct {
            Project string `json:"project,omitempty"`
        }
        json.NewDecoder(r.Body).Decode(&req)

        result, err := repo.Scan(r.Context(), req.Project)
        if err != nil {
            writeError(w, apiError{500, err.Error()})
            return
        }
        writeJSON(w, 200, result)
    }
}
```

### Route registration

```go
func RegisterConflictRoutes(mux *http.ServeMux, repo ConflictRepo) {
    mux.HandleFunc("GET /conflicts", handleListConflicts(repo))
    mux.HandleFunc("POST /conflicts/judge", handleJudgeConflict(repo))
    mux.HandleFunc("POST /conflicts/compare", handleCompare(repo))
    mux.HandleFunc("GET /conflicts/{id}", handleGetConflict(repo))
    mux.HandleFunc("GET /conflicts/stats", handleConflictStats(repo))
    mux.HandleFunc("POST /conflicts/scan", handleScan(repo))
    mux.HandleFunc("GET /conflicts/deferred", handleListDeferred(repo))
    mux.HandleFunc("POST /conflicts/deferred/replay", handleReplayDeferred(repo))
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Judge automático en POST /observations | Haría el POST lento; el judge es explícito y asíncrono |
| Merge automático en conflictos | Peligroso; la decisión final debe ser del usuario o un judge explícito |
| WebSocket para scan progresivo | HTTP polling es suficiente para el caso de uso actual |

## Diagrama

```
Client HTTP                          memoria server (localhost:7437)
    |                                          |
    | GET  /conflicts                           |
    | POST /conflicts/judge                    |
    | POST /conflicts/compare                   |
    | GET  /conflicts/{id}                      |
    | GET  /conflicts/stats                     |
    | POST /conflicts/scan                      |
    | GET  /conflicts/deferred                  |
    | POST /conflicts/deferred/replay           |
    |                                          |
    +--------> api/conflicts.go -------------> store/conflict.go
                                                |
                                            SQLite DB
                                                |
                                    +-----------+
                                    |
                            conflict/ (REQ-10)
                            ├── lexical.go
                            ├── semantic.go
                            └── deferred.go
```

## TDD plan

1. **Red:** GET /conflicts → array (empty) → falla
2. **Green:** List handler → pasa
3. **Red:** POST /conflicts/scan → conflicts_found → falla
4. **Green:** Scan handler delegando a lexical scan → pasa
5. **Red:** POST /conflicts/compare → similarity → falla
6. **Green:** Compare handler → pasa
7. **Red:** GET /conflicts/stats → métricas → falla
8. **Green:** Stats handler → pasa
9. **Red:** GET /conflicts/deferred → array → falla
10. **Green:** Deferred handler → pasa
11. **Sabotaje:** Scanner sin límite → escanea toda la DB sin control → agregar límite → test pasa

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| REQ-10 modules not implemented yet | Los handlers definen la interfaz; implementation vendrá después |
| Judge sin modelo configurado | Retornar error 400 si no hay modelo disponible |
| Scan sin límite puede saturar | Default limit de 1000 observaciones por scan |
