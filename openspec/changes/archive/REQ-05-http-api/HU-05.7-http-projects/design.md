# Design: HU-05.7-http-projects

## Decisión arquitectónica

### ProjectRepo interface

```go
type ProjectRepo interface {
    DetectCurrent(ctx context.Context, cwd string) (ProjectResult, error)
    Migrate(ctx context.Context, from, to string) (MigrationResult, error)
}
```

### Project resolution

```go
func (r *projectRepo) DetectCurrent(ctx context.Context, cwd string) (ProjectResult, error) {
    if cwd == "" {
        cwd, _ = os.Getwd()
    }
    // Use detection chain from HU-08.1
    result, err := project.Detect(cwd)
    if err != nil {
        return ProjectResult{
            Project:    "default",
            Source:     "fallback",
            Confidence: "low",
            Directory:  cwd,
        }, nil
    }
    result.Directory = cwd
    return result, nil
}
```

### Migration transaction

```go
func (r *projectRepo) Migrate(ctx context.Context, from, to string) (MigrationResult, error) {
    if from == to {
        return MigrationResult{}, ErrSameProject
    }

    tx, _ := r.db.BeginTx(ctx, nil)
    defer tx.Rollback()

    var result MigrationResult

    res, _ := tx.ExecContext(ctx,
        `UPDATE observations SET project = ? WHERE project = ? AND deleted_at IS NULL`, to, from)
    n, _ := res.RowsAffected()
    result.ObservationsMoved = int(n)

    res, _ = tx.ExecContext(ctx,
        `UPDATE sessions SET project = ? WHERE project = ?`, to, from)
    n, _ = res.RowsAffected()
    result.SessionsMoved = int(n)

    res, _ = tx.ExecContext(ctx,
        `UPDATE user_prompts SET project = ? WHERE project = ?`, to, from)
    n, _ = res.RowsAffected()
    result.PromptsMoved = int(n)

    if err := tx.Commit(); err != nil {
        return MigrationResult{}, err
    }

    return result, nil
}
```

### Route registration

```go
func RegisterProjectRoutes(mux *http.ServeMux, repo ProjectRepo, requireAuth func(http.Handler) http.Handler) {
    mux.HandleFunc("GET /project/current", handleCurrentProject(repo))
    mux.Handle("POST /projects/migrate", requireAuth(http.HandlerFunc(handleMigrate(repo))))
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Migrate con soft-delete check | Migrate solo mueve proyectos activos; si se necesita, se agrega después |
| GET /projects list | No hay tabla de proyectos; los proyectos son strings en la columna project |

## Diagrama

```
Client HTTP                          memoria server
    |                                       |
    | GET /project/current?cwd=/path        |
    |   +-- project.Detect(cwd) -----------> HU-08.1 chain
    |   +-- ProjectResult response           |
    |                                       |
    | POST /projects/migrate                 |
    |   +-- auth middleware                   |
    |   +-- BEGIN TRANSACTION                |
    |   +-- UPDATE observations SET project  |
    |   +-- UPDATE sessions SET project      |
    |   +-- UPDATE user_prompts SET project  |
    |   +-- COMMIT                           |
    |   +-- MigrationResult                  |
    |                                       |
    +--------> api/projects.go ----------> store/project.go
                                                |
                                            SQLite DB
```

## TDD plan

1. **Red:** Test GET /project/current → 200 project → falla
2. **Green:** Detect handler → pasa
3. **Red:** Test POST /projects/migrate → 200 → falla
4. **Green:** Migration handler → pasa
5. **Red:** Test migrate same → 400 → falla
6. **Green:** Validar from != to → pasa
7. **Sabotaje:** Sacar validación from==to → migrate same ejecuta UPDATE (0 rows) pero retorna 200 → test falla → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| project.Detect no implementado | Fallback a "default" con confidence "low" |
| Migration sin datos | Result con 0s, no error |
| UPDATE sessions cambia project de sesiones activas | Es intencional; la sesión sigue activa pero ahora pertenece al nuevo proyecto |
