# Design: issue-01.2-observation-crud

## Decisión arquitectónica

### Store API surface

Se exponen 5 funciones públicas en el package `internal/store`:

```go
// AddObservation crea una observación. Si candidatesOut no es nil, se llena con
// observaciones existentes que coinciden en normalized_hash (conflict detection).
// capturePrompt controla si se intenta guardar el prompt global actual.
func AddObservation(db *sql.DB, obs Observation, capturePrompt bool, candidatesOut *[]Candidate) (int64, error)

// GetObservation retorna una observación por ID. Si includeDeleted es false,
// excluye observaciones con deleted_at IS NOT NULL.
func GetObservation(db *sql.DB, id int64, includeDeleted bool) (Observation, error)

// UpdateObservation aplica actualizaciones parciales. Solo los campos no-nil en
// updates se modifican. Recalcula normalized_hash si title o content cambian.
func UpdateObservation(db *sql.DB, id int64, updates ObservationUpdate) error

// DeleteObservation elimina una observación. hard=true elimina físicamente la fila;
// hard=false setea deleted_at (soft delete). Error si soft-delete sobre ya eliminada.
func DeleteObservation(db *sql.DB, id int64, hard bool) error

// RecentObservations retorna observaciones recientes con filtros opcionales.
// Excluye soft-deleted a menos que filter.IncludeDeleted sea true.
func RecentObservations(db *sql.DB, filter ObservationFilter) ([]Observation, error)
```

### Tipos

```go
// Observation representa una fila completa de la tabla observations.
type Observation struct {
    ID              int64      `json:"id"`
    SessionID       string     `json:"session_id"`
    Type            string     `json:"type"`
    Title           string     `json:"title"`
    Content         string     `json:"content"`
    ToolName        string     `json:"tool_name"`
    Project         string     `json:"project"`
    Scope           string     `json:"scope"`
    TopicKey        *string    `json:"topic_key,omitempty"`
    NormalizedHash  *string    `json:"normalized_hash,omitempty"`
    RevisionCount   int        `json:"revision_count"`
    DuplicateCount  int        `json:"duplicate_count"`
    LastSeenAt      *string    `json:"last_seen_at,omitempty"`
    CreatedAt       string     `json:"created_at"`
    UpdatedAt       string     `json:"updated_at"`
    DeletedAt       *string    `json:"deleted_at,omitempty"`
}

// ObservationUpdate contiene campos opcionales para actualización parcial.
// Usamos punteros para distinguir "no enviado" de "enviado como vacío".
type ObservationUpdate struct {
    Title    *string
    Content  *string
    Type     *string
    Scope    *string
    TopicKey **string   // nil = no cambiar, pointer to nil = setear a NULL
}

// ObservationFilter para RecentObservations.
type ObservationFilter struct {
    Project        string
    Scope          string
    Type           string
    Limit          int      // default 50
    Offset         int
    IncludeDeleted bool
    SortDesc       bool     // default true (created_at DESC)
}

// Candidate representa una observación similar encontrada durante AddObservation.
type Candidate struct {
    ID     int64  `json:"id"`
    Title  string `json:"title"`
    Reason string `json:"reason"` // "exact_hash_match" | "high_similarity"
}
```

### Normalized hash para dedup

Se implementa `computeNormalizedHash(obs Observation) string`:

1. Normalizar content: `strings.ToLower` → `strings.TrimSpace` → collapse whitespace → strip basic punctuation (`.,;:!?`)
2. Concatenar: `project + "|" + scope + "|" + type + "|" + title + "|" + normalizedContent`
3. SHA-256 del resultado, hex-encoded

```go
func computeNormalizedHash(obs Observation) string {
    norm := normalizeContent(obs.Content)
    payload := strings.Join([]string{
        obs.Project, obs.Scope, obs.Type,
        strings.TrimSpace(strings.ToLower(obs.Title)),
        norm,
    }, "|")
    h := sha256.Sum256([]byte(payload))
    return hex.EncodeToString(h[:])
}
```

El hash se almacena en `observations.normalized_hash` y se indexa (`idx_obs_hash`). Sirve tanto para dedup como para detección rápida de conflictos.

### Conflict detection en AddObservation

Antes de insertar, se ejecuta `findCandidates(db, obs) ([]Candidate, error)`:

```sql
SELECT id, title FROM observations
WHERE normalized_hash = ?
  AND id != ?
  AND deleted_at IS NULL
LIMIT 5
```

Si hay resultados, se agregan a `candidatesOut` con reason `"exact_hash_match"`. Esto permite al caller decidir si crea igual o salta.

El flujo completo de AddObservation:

```
1. Validar title != "" && content != ""
2. Calcular normalized_hash
3. findCandidates(db, obs) → llenar candidatesOut (si hay)
4. Iniciar transacción
5. INSERT en observations
6. Si capturePrompt=true y CurrentPrompt != "":
     INSERT en user_prompts (session_id, content, project)
7. Commit tx
8. Retornar rowid del INSERT
```

### Soft delete vs hard delete

| Operación | SQL |
|-----------|-----|
| Soft delete | `UPDATE observations SET deleted_at = datetime('now') WHERE id = ? AND deleted_at IS NULL` |
| Hard delete | `DELETE FROM observations WHERE id = ?` |

Soft delete verifica `deleted_at IS NULL` en el WHERE para evitar doble eliminación. Si `RowsAffected == 0`, retorna error `"observation not found or already deleted"`.

Hard delete elimina físicamente; los triggers FTS5 (issue-01.1) se encargan de limpiar el índice automáticamente.

### FTS5 sync

No se toca FTS5 explícitamente. Los triggers definidos en issue-01.1 (`observations_ai`, `observations_ad`, `observations_au`) sincronizan automáticamente:
- INSERT → observations_ai agrega fila a observations_fts
- UPDATE → observations_au borra la vieja e inserta la nueva
- DELETE → observations_ad borra de observations_fts

### Prompt capture

Variable global a nivel package:

```go
var CurrentPrompt string
```

En `AddObservation`, si `capturePrompt == true` y `CurrentPrompt != ""`, dentro de la misma transacción se ejecuta:

```sql
INSERT INTO user_prompts (session_id, content, project)
VALUES (?, ?, ?)
```

Con los valores de la observación y el prompt actual. Es **best-effort**: si `CurrentPrompt` está vacío o fue consumido por otro hilo, simplemente no se guarda.

### Error handling patterns

Todos los errores se envuelven con contexto mediante `fmt.Errorf("AddObservation: %w", err)`.

| Condición | Error |
|-----------|-------|
| title vacío y content vacío | `ErrValidation` — "title and content are required" |
| GetObservation ID no existe | `ErrNotFound` — "observation not found" |
| Soft delete sobre ya eliminada | `ErrConflict` — "observation already deleted" |
| Violación FK (session_id inválido) | Error de SQLite propagado con contexto |
| Error de DB transitorio | Propagado sin wrap especial |

Se definen errores centinela:

```go
var (
    ErrNotFound    = errors.New("observation not found")
    ErrValidation  = errors.New("validation error")
    ErrConflict    = errors.New("conflict")
)
```

### RecentObservations query builder

```go
func buildRecentQuery(filter ObservationFilter) (string, []any) {
    clauses := []string{"FROM observations WHERE deleted_at IS NULL"}
    args := []any{}

    if filter.Project != "" {
        clauses = append(clauses, "AND project = ?")
        args = append(args, filter.Project)
    }
    if filter.Scope != "" {
        clauses = append(clauses, "AND scope = ?")
        args = append(args, filter.Scope)
    }
    // ... similar para type

    order := "DESC"
    if !filter.SortDesc {
        order = "ASC"
    }

    limit := filter.Limit
    if limit <= 0 {
        limit = 50
    }

    query := "SELECT id, session_id, type, title, content, tool_name, project, scope, " +
        "topic_key, normalized_hash, revision_count, duplicate_count, " +
        "last_seen_at, created_at, updated_at, deleted_at " +
        strings.Join(clauses, " ") +
        " ORDER BY created_at " + order +
        " LIMIT ? OFFSET ?"

    args = append(args, limit, filter.Offset)
    return query, args
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| ORM (gorm, sqlx) | La capa es delgada; `database/sql` directo da control total sobre queries y errores; sin magic de preloading |
| CASCADE delete en FK | Hard delete con CASCADE borraría memory_relations hijas — mejor control explícito desde la app |
| UUID como PK | `INTEGER PRIMARY KEY AUTOINCREMENT` da mejor performance en SQLite; es la convención natural |
| Conflict detection vía `ON CONFLICT` de SQLite | Los candidates requieren lógica app-level (qué tan similares, qué razones asignar); un constraint no basta |
| Captura de prompt vía trigger | La lógica de "si capture_prompt=true entonces inserta en user_prompts" es condicional y depende del flag runtime; un trigger no puede leer variables Go |
| Paginación vía cursor (keyset) | Limit/offset es suficiente para el volumen esperado (miles, no millones); keyset agrega complejidad innecesaria |
| Transacción por defecto en GetObservation | GetObservation es solo SELECT; no necesita tx; se ahorra overhead |

## TDD plan

1. **Red:** Test `TestAddObservation` — crea observación con todos los campos, espera ID > 0 → falla (no hay implementación)
2. **Green:** Implementar `AddObservation` mínima → pasa
3. **Refactor:** Extraer `computeNormalizedHash`, estructura de tipos
4. **Red:** `TestAddObservationValidation` — omitir title → espera error → falla
5. **Green:** Agregar validación → pasa
6. **Red:** `TestAddObservationCandidates` — crear dos observaciones similares → espera candidates no vacío → falla
7. **Green:** Agregar `findCandidates` query → pasa
8. **Red:** `TestGetObservation` — leer observación existente → espera campos correctos → falla
9. **Green:** Implementar `GetObservation` → pasa
10. **Red:** `TestGetObservationNotFound` — leer ID 9999 → espera ErrNotFound → falla
11. **Green:** Agregar check de rows affected → pasa
12. **Red:** `TestUpdateObservation` — actualizar title y content → espera nuevos valores + revision_count incrementado → falla
13. **Green:** Implementar `UpdateObservation` → pasa
14. **Red:** `TestSoftDelete` + `TestHardDelete` → fallan
15. **Green:** Implementar ambas variantes → pasan
16. **Red:** `TestDoubleSoftDelete` → falla (segundo delete da error)
17. **Green:** Agregar check `deleted_at IS NULL` en WHERE → pasa
18. **Red:** `TestRecentObservations` — crear 5 obs, filtrar por project, limit=3 → espera 3 resultados → falla
19. **Green:** Implementar `RecentObservations` + query builder → pasa
20. **Sabotaje:** Romper FK reference en session_id → test cae → restaurar → pasa

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Candidates lentos con muchas observaciones | Index on `normalized_hash` ya existe (issue-01.1); query es `WHERE normalized_hash = ?` que es O(log n) |
| Prompt capture pierde contexto entre llamadas | Variable global `CurrentPrompt` es suficientemente buena para single-process; si hay concurrencia real en el futuro, migrar a context |
| Transaction en AddObservation puede fallar a medio camino | Todo dentro de una tx; si falla, rollback; no hay estado inconsistente |
| normalized_hash colisión (SHA-256 teórica) | Probabilidad despreciable; si ocurre, candidates listará un falso positivo aceptable |
| UpdateObservation sin cambios (mismos valores) | Siempre actualiza `updated_at` e incrementa `revision_count` — intencional, la app puede optimizar después |
