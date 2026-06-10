# Design: issue-01.6-prompt-storage

## Arquitectura

Las operaciones de prompt viven en un archivo separado `internal/store/prompt.go` con receiver `*Store`. El DDL ya fue creado en issue-01.1, por lo que esta HU solo agrega la capa de acceso a datos.

### Store struct

```go
type Store struct {
    db *sql.DB
}
```

Se reutiliza el mismo `*Store` que el resto del paquete `store`. No se crea un store separado.

### Funciones

```go
// AddPrompt inserta un prompt en user_prompts.
// Retorna el ID del nuevo registro.
// Error si content está vacío o si session_id no existe (FK).
func (s *Store) AddPrompt(ctx context.Context, sessionID, content, project string) (int64, error)

// GetPrompt obtiene un prompt por su ID.
// Retorna error si no existe.
func (s *Store) GetPrompt(ctx context.Context, id int64) (*Prompt, error)

// ListPrompts lista prompts con filtro opcional de proyecto.
// Ordenados por created_at DESC. Paginación vía limit/offset.
func (s *Store) ListPrompts(ctx context.Context, project string, limit, offset int) ([]*Prompt, error)

// DeletePrompt elimina un prompt por ID.
// No es soft-delete; se borra físicamente.
func (s *Store) DeletePrompt(ctx context.Context, id int64) error

// SearchPrompts busca en prompts_fts con la query dada.
// Filtro opcional por project aplicado vía subquery o JOIN.
func (s *Store) SearchPrompts(ctx context.Context, query, project string, limit int) ([]*Prompt, error)

// capturePrompt guarda el texto del prompt en un buffer process-local.
func capturePrompt(content string)

// GetCapturedPrompt recupera y limpia el buffer.
func GetCapturedPrompt() string
```

### Prompt struct

```go
type Prompt struct {
    ID        int64  `json:"id"`
    SessionID string `json:"session_id"`
    Content   string `json:"content"`
    Project   string `json:"project"`
    CreatedAt string `json:"created_at"`
}
```

### SQL queries

```sql
-- AddPrompt
INSERT INTO user_prompts (session_id, content, project)
VALUES (?, ?, ?);

-- GetPrompt
SELECT id, session_id, content, project, created_at
FROM user_prompts WHERE id = ?;

-- ListPrompts
SELECT id, session_id, content, project, created_at
FROM user_prompts
WHERE (? = '' OR project = ?)
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- DeletePrompt
DELETE FROM user_prompts WHERE id = ?;

-- SearchPrompts (FTS5)
SELECT p.id, p.session_id, p.content, p.project, p.created_at
FROM user_prompts p
JOIN prompts_fts f ON p.id = f.rowid
WHERE prompts_fts MATCH ?
AND (? = '' OR p.project = ?)
ORDER BY rank
LIMIT ?;
```

### FTS5 query sanitizer

```go
// sanitizeFTS5Query escapa caracteres especiales de FTS5 para evitar
// syntax errors en la búsqueda. Los caracteres reservados son:
// ^ * " ( ) : + - ~ < >
func sanitizeFTS5Query(q string) string {
    re := regexp.MustCompile(`[*"():+\-~<>^]`)
    return re.ReplaceAllString(q, "")
}
```

### Captured prompt buffer

```go
package store

import "sync"

var (
    capturedPrompt string
    capturedMu     sync.Mutex
)

func capturePrompt(content string) {
    capturedMu.Lock()
    defer capturedMu.Unlock()
    capturedPrompt = content
}

func GetCapturedPrompt() string {
    capturedMu.Lock()
    defer capturedMu.Unlock()
    p := capturedPrompt
    capturedPrompt = "" // consume el buffer
    return p
}
```

El buffer es process-local, no persiste entre reinicios de la aplicación. Su propósito es permitir que el flujo "usuario escribe prompt → capturePrompt → usuario ejecuta domain_mem_save → GetCapturedPrompt inyecta el contexto" funcione sin que el prompt se pierda entre la escritura y la acción.

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Soft-delete para prompts | No hay necesidad; prompts son descartables, DELETE físico es suficiente |
| Canal Go en vez de mutex para buffer | Overkill para un solo valor; mutex es más simple y predecible |
| Persistir buffer en SQLite | Ya está persistido en user_prompts; el buffer es solo para contexto inmediato |
| Prompts en tabla separada con FK a observations | No hay relación semántica; prompts son entradas del usuario, no observaciones del sistema |

## TDD plan

1. **Red:** Test `AddPrompt` con content vacío espera error → falla sin validación
2. **Green:** Agregar validación de content vacío → pasa
3. **Red:** Test `AddPrompt` exitoso → inserta y retorna ID
4. **Green:** Implementar `AddPrompt` con INSERT → pasa
5. **Red:** Test `GetPrompt` → obtiene prompt por ID
6. **Green:** Implementar `GetPrompt` con SELECT → pasa
7. **Refactor:** Extraer scan de fila a helper `scanPrompt(scanner)`
8. **Red:** Test `ListPrompts` con filtro project
9. **Green:** Implementar `ListPrompts` con WHERE dinámico → pasa
10. **Red:** Test `DeletePrompt` → elimina y GetPrompt falla
11. **Green:** Implementar `DeletePrompt` con DELETE → pasa
12. **Red:** Test `SearchPrompts` en FTS5
13. **Green:** Implementar con JOIN a prompts_fts → pasa
14. **Red:** Test `capturePrompt` / `GetCapturedPrompt`
15. **Green:** Implementar buffer con mutex → pasa
16. **Sabotaje:** Comentar validación de content vacío → test cae → restaurar → pasa

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| FTS5 query malformada | Sanitizer de queries; test con caracteres especiales |
| FK violation silenciosa | SQLite retorna error; propagar al caller |
| Race en capturedPrompt | Mutex cubre ambas operaciones |
| Session ID no existe | FK constraint + error handling; test específico |
