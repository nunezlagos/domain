# Design: HU-05.4-http-prompts

## Decisión arquitectónica

### PromptRepo interface

```go
type Prompt struct {
    ID        int    `json:"id"`
    SessionID string `json:"session_id"`
    Content   string `json:"content"`
    Project   string `json:"project"`
    CreatedAt string `json:"created_at"`
}

type PromptRepo interface {
    Save(ctx context.Context, p Prompt) (Prompt, error)
    Recent(ctx context.Context, limit int) ([]Prompt, error)
    Search(ctx context.Context, query, project string) ([]Prompt, error)
    Delete(ctx context.Context, id int) error
}
```

### FTS5 search for prompts

La tabla `prompts_fts` se mantiene sincronizada vía triggers (HU-01.1). El search query es similar al de observations pero sobre prompts_fts:

```go
func (r *promptRepo) Search(ctx context.Context, query, project string) ([]Prompt, error) {
    safeQuery := sanitizeFTS5(query)
    q := `SELECT p.id, p.session_id, p.content, p.project, p.created_at
          FROM prompts_fts
          JOIN user_prompts p ON p.id = prompts_fts.rowid
          WHERE prompts_fts MATCH ?`
    args := []any{safeQuery}

    if project != "" {
        q += " AND p.project = ?"
        args = append(args, project)
    }

    q += " ORDER BY rank DESC LIMIT 20"
    // execute and scan
}
```

### Prompt vs Observation distinction

Los prompts son consultas de usuario, no observaciones de memoria. Se almacenan en tabla separada (`user_prompts`) con su propio FTS5 index. No tienen normalized_hash, conflict detection ni soft delete.

### Route registration

```go
func RegisterPromptRoutes(mux *http.ServeMux, repo PromptRepo) {
    mux.HandleFunc("POST /prompts", handleSavePrompt(repo))
    mux.HandleFunc("GET /prompts/recent", handleRecentPrompts(repo))
    mux.HandleFunc("GET /prompts/search", handleSearchPrompts(repo))
    mux.HandleFunc("DELETE /prompts/{id}", handleDeletePrompt(repo))
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Almacenar prompts en observations | Semántica diferente; prompts son consultas, no observaciones |
| Soft delete para prompts | Los prompts no tienen relaciones FK, DELETE físico es seguro |
| PATCH para prompts | No hay caso de uso identificado para editar prompts históricos |

## Diagrama

```
Client HTTP                          memoria server
    |                                       |
    | POST /prompts                          |
    |   +-- INSERT user_prompts -----------> store.Save()
    |   +-- trigger syncs prompts_fts        |
    |                                       |
    | GET  /prompts/recent                   |
    |   +-- SELECT ORDER BY created_at -----> store.Recent()
    |                                       |
    | GET  /prompts/search?q=               |
    |   +-- FTS5 MATCH prompts_fts --------> store.Search()
    |                                       |
    | DELETE /prompts/{id}                   |
    |   +-- DELETE FROM user_prompts -------> store.Delete()
    |                                       |
    +--------> api/prompts.go ------------> store/prompt.go
                                                |
                                            SQLite DB
```

## TDD plan

1. **Red:** Test POST /prompts → 201 → falla
2. **Green:** Save handler → pasa
3. **Red:** Test POST sin content → 400 → falla
4. **Green:** Validación → pasa
5. **Red:** Test GET /prompts/recent → array → falla
6. **Green:** Recent handler → pasa
7. **Red:** Test GET /prompts/search?q= → resultados → falla
8. **Green:** FTS5 search → pasa
9. **Red:** Test DELETE → 204 → falla
10. **Green:** Delete handler → pasa
11. **Sabotaje:** DELETE sin verificar existencia → DELETE 9999 da 204 igual → agregar check → test pasa

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| FTS5 prompts_fts desync | Depende de triggers de HU-01.1; si falla, rebuild manual |
| POST session_id inválido | Validar que session exista; si no, 400 con mensaje |
| Search sin índice | Same as above; triggers deben estar configurados |
