# Design: issue-05.5-http-export-import

## Decisión arquitectónica

### ExportRepo interface

```go
type ExportRepo interface {
    ExportProject(ctx context.Context, project string) (ExportPayload, error)
    ImportProject(ctx context.Context, payload ExportPayload) (ImportResult, error)
}
```

### Atomic import transaction pattern

```go
func (r *exportRepo) ImportProject(ctx context.Context, payload ExportPayload) (ImportResult, error) {
    tx, err := r.db.BeginTx(ctx, nil)
    if err != nil { return ImportResult{}, err }
    defer tx.Rollback() // safe; no-op after Commit

    var result ImportResult

    // 1. Import sessions with INSERT OR IGNORE
    for _, s := range payload.Sessions {
        res, err := tx.ExecContext(ctx,
            `INSERT OR IGNORE INTO sessions (id, project, directory, started_at, ended_at, summary, status)
             VALUES (?, ?, ?, ?, ?, ?, ?)`,
            s.ID, s.Project, s.Directory, s.StartedAt, s.EndedAt, s.Summary, s.Status)
        if err != nil {
            return result, fmt.Errorf("session %s: %w", s.ID, err)
        }
        n, _ := res.RowsAffected()
        result.SessionsImported += int(n)
    }

    // 2. Import observations
    for _, o := range payload.Observations {
        _, err := tx.ExecContext(ctx,
            `INSERT INTO observations (session_id, type, title, content, tool_name, project, scope,
             topic_key, normalized_hash, revision_count, duplicate_count, created_at, updated_at)
             VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
            o.SessionID, o.Type, o.Title, o.Content, o.ToolName, o.Project, o.Scope,
            o.TopicKey, o.NormalizedHash, o.RevisionCount, o.DuplicateCount,
            o.CreatedAt, o.UpdatedAt)
        if err != nil {
            return result, fmt.Errorf("observation: %w", err)
        }
        result.ObservationsImported++
    }

    // 3. Import prompts
    for _, p := range payload.Prompts {
        _, err := tx.ExecContext(ctx,
            `INSERT INTO user_prompts (session_id, content, project, created_at)
             VALUES (?, ?, ?, ?)`,
            p.SessionID, p.Content, p.Project, p.CreatedAt)
        if err != nil {
            return result, fmt.Errorf("prompt: %w", err)
        }
        result.PromptsImported++
    }

    if err := tx.Commit(); err != nil {
        return result, fmt.Errorf("commit: %w", err)
    }

    return result, nil
}
```

### Export queries

```go
func (r *exportRepo) ExportProject(ctx context.Context, project string) (ExportPayload, error) {
    ep := ExportPayload{
        ExportedAt: time.Now().UTC().Format(time.RFC3339),
        Project:    project,
        Source:     "Domain",
        Version:    version.String(), // from build info
    }

    // Sessions
    rows, _ := r.db.QueryContext(ctx,
        `SELECT id, project, directory, started_at, ended_at, summary, status
         FROM sessions WHERE project = ?`, project)
    ep.Sessions = scanSessions(rows)

    // Observations
    obsRows, _ := r.db.QueryContext(ctx,
        `SELECT ... FROM observations WHERE project = ? AND deleted_at IS NULL`, project)
    ep.Observations = scanObservations(obsRows)

    // Prompts
    pRows, _ := r.db.QueryContext(ctx,
        `SELECT ... FROM user_prompts WHERE project = ?`, project)
    ep.Prompts = scanPrompts(pRows)

    return ep, nil
}
```

### Auth middleware (delegado a issue-05.9)

```go
// En api/middleware.go:
func RequireToken(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := os.Getenv("ENGRAM_HTTP_TOKEN")
        if token == "" {
            writeError(w, apiError{500, "ENGRAM_HTTP_TOKEN not configured"})
            return
        }
        auth := r.Header.Get("Authorization")
        if auth != "Bearer "+token {
            writeError(w, apiError{401, "invalid or missing token"})
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

### Route registration

```go
func RegisterExportRoutes(mux *http.ServeMux, repo ExportRepo, authMiddleware func(http.Handler) http.Handler) {
    mux.Handle("GET /export", authMiddleware(http.HandlerFunc(handleExport(repo))))
    mux.Handle("POST /import", authMiddleware(http.HandlerFunc(handleImport(repo))))
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Import con INSERT OR REPLACE | Sobreescribe sesiones existentes; OR IGNORE preserva datos existentes |
| Export en múltiples requests | Un solo GET es más simple; para proyectos gigantes se puede agregar paginación después |
| JSON streaming | No justificado para el volumen esperado (< 10MB típico) |
| Formato binario (gob, protobuf) | JSON es universal, depurable, compresible; import/export son operaciones manuales |

## Diagrama

```
Client HTTP                             memoria server
    |                                          |
    | GET /export?project=myapp                 |
    |   +-- SELECT sessions WHERE project -----> sessions
    |   +-- SELECT observations WHERE project --> observations
    |   +-- SELECT prompts WHERE project -------> user_prompts
    |   +-- JSON response                       |
    |                                          |
    | POST /import (JSON body)                  |
    |   +-- BEGIN TRANSACTION                   |
    |   +-- INSERT OR IGNORE sessions           |
    |   +-- INSERT observations                 |
    |   +-- INSERT prompts                      |
    |   +-- COMMIT / ROLLBACK                   |
    |   +-- ImportResult response               |
    |                                          |
    +--------> api/export.go ----------------> store/export.go
                                                    |
                                                SQLite DB
```

## TDD plan

1. **Red:** Test GET /export?project=test → 200 con estructura → falla
2. **Green:** Export handler con queries → pasa
3. **Red:** Test export sin project → 400 → falla
4. **Green:** Validación → pasa
5. **Red:** Test POST /import → 200 con metrics → falla
6. **Green:** Import handler con tx → pasa
7. **Red:** Test import falla → rollback, DB sin cambios → falla
8. **Green:** Si error en tx, ROLLBACK → pasa
9. **Red:** Test import sin token → 401 → falla
10. **Green:** Agregar auth middleware al route → pasa
11. **Red:** Import idempotente → segunda vez no duplica → falla
12. **Green:** INSERT OR IGNORE → pasa
13. **Sabotaje:** Cambiar INSERT OR IGNORE a INSERT → duplica sessions en segunda import → test cae → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Import atómico lento con 10k registros | Una sola tx; si es lento, hacer batches de 500 en futures |
| Token Bearer en variable de entorno | Standard seguro; doc recomienda no commitear token |
| Export incluye soft-deleted observations | WHERE deleted_at IS NULL explícito |
| Import con session_id que viola FK | INSERT OR IGNORE sessions primero garantiza que existan |
