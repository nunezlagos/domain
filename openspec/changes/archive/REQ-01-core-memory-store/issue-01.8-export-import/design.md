# Design: issue-01.8-export-import

## Arquitectura

El export/import vive en `internal/store/export.go` con un struct contenedor `ExportData` que refleja la estructura del JSON. Export serializa desde SQLite, import deserializa y persiste en una transacción.

### ExportData struct

```go
type ExportData struct {
    Sessions     []Session     `json:"sessions"`
    Observations []Observation `json:"observations"`
    Prompts      []Prompt      `json:"prompts"`
}
```

Reutiliza los structs `Session`, `Observation` y `Prompt` ya definidos en el paquete store. Si estos structs no existen aún como estructuras anotadas con JSON tags, se crean o se definen structs específicos para export.

### Export

```go
// Export devuelve un JSON con todas las sesiones, observaciones (no soft-deleteadas)
// y prompts. Si project no está vacío, filtra observaciones y prompts por proyecto.
func (s *Store) Export(ctx context.Context, project string) ([]byte, error) {
    data := &ExportData{}

    // Sessions: siempre todas (no tienen project filter)
    rows, err := s.db.QueryContext(ctx, "SELECT id, project, directory, started_at, ended_at, summary, status FROM sessions")
    if err != nil {
        return nil, fmt.Errorf("export sessions: %w", err)
    }
    defer rows.Close()
    // scan rows...

    // Observations: excluir soft-delete, filtrar por project si aplica
    query := "SELECT ... FROM observations WHERE deleted_at IS NULL"
    var args []interface{}
    if project != "" {
        query += " AND project = ?"
        args = append(args, project)
    }
    query += " ORDER BY id"
    rows, err = s.db.QueryContext(ctx, query, args...)
    // scan rows...

    // Prompts: mismo filtro por project
    query = "SELECT ... FROM user_prompts"
    if project != "" {
        query += " WHERE project = ?"
    }
    query += " ORDER BY id"
    rows, err = s.db.QueryContext(ctx, query, args...)
    // scan rows...

    return json.MarshalIndent(data, "", "  ")
}
```

### Import

```go
// Import carga un JSON exportado en la base de datos.
// La operación es atómica: todo o nada.
func (s *Store) Import(ctx context.Context, data []byte) error {
    // 1. Validar que es JSON
    var export ExportData
    if err := json.Unmarshal(data, &export); err != nil {
        return fmt.Errorf("invalid JSON: %w", err)
    }

    // 2. Validar estructura
    if err := validateExportData(&export); err != nil {
        return fmt.Errorf("invalid export data: %w", err)
    }

    // 3. Iniciar transacción
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("begin tx: %w", err)
    }
    defer tx.Rollback() // no-op si ya se hizo Commit

    // 4. Insertar sessions con INSERT OR IGNORE
    stmtSession, err := tx.PrepareContext(ctx,
        `INSERT OR IGNORE INTO sessions (id, project, directory, started_at, ended_at, summary, status)
         VALUES (?, ?, ?, ?, ?, ?, ?)`)
    if err != nil {
        return fmt.Errorf("prepare sessions: %w", err)
    }
    defer stmtSession.Close()

    for _, s := range export.Sessions {
        _, err := stmtSession.ExecContext(ctx, s.ID, s.Project, s.Directory,
            s.StartedAt, s.EndedAt, s.Summary, s.Status)
        if err != nil {
            return fmt.Errorf("insert session %s: %w", s.ID, err)
        }
    }

    // 5. Insertar observations
    stmtObs, err := tx.PrepareContext(ctx,
        `INSERT INTO observations (session_id, type, title, content, tool_name, project, scope, topic_key, normalized_hash, revision_count, duplicate_count, last_seen_at, created_at, updated_at, deleted_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
    if err != nil {
        return fmt.Errorf("prepare observations: %w", err)
    }
    defer stmtObs.Close()

    for _, o := range export.Observations {
        _, err := stmtObs.ExecContext(ctx, o.SessionID, o.Type, o.Title, o.Content,
            o.ToolName, o.Project, o.Scope, o.TopicKey, o.NormalizedHash,
            o.RevisionCount, o.DuplicateCount, o.LastSeenAt,
            o.CreatedAt, o.UpdatedAt, o.DeletedAt)
        if err != nil {
            return fmt.Errorf("insert observation %d: %w", o.ID, err)
        }
    }

    // 6. Insertar prompts
    stmtPrompt, err := tx.PrepareContext(ctx,
        `INSERT INTO user_prompts (session_id, content, project, created_at)
         VALUES (?, ?, ?, ?)`)
    if err != nil {
        return fmt.Errorf("prepare prompts: %w", err)
    }
    defer stmtPrompt.Close()

    for _, p := range export.Prompts {
        _, err := stmtPrompt.ExecContext(ctx, p.SessionID, p.Content, p.Project, p.CreatedAt)
        if err != nil {
            return fmt.Errorf("insert prompt %d: %w", p.ID, err)
        }
    }

    // 7. Commit
    if err := tx.Commit(); err != nil {
        return fmt.Errorf("commit: %w", err)
    }

    return nil
}
```

### validateExportData

```go
func validateExportData(data *ExportData) error {
    if data.Sessions == nil {
        return errors.New("missing required field: sessions")
    }
    if data.Observations == nil {
        return errors.New("missing required field: observations")
    }
    if data.Prompts == nil {
        return errors.New("missing required field: prompts")
    }

    // Validar que todos los session_id referenciados existen
    sessionIDs := make(map[string]bool)
    for _, s := range data.Sessions {
        sessionIDs[s.ID] = true
    }

    for _, o := range data.Observations {
        if !sessionIDs[o.SessionID] {
            return fmt.Errorf("observation %d references unknown session: %s", o.ID, o.SessionID)
        }
    }

    for _, p := range data.Prompts {
        if !sessionIDs[p.SessionID] {
            return fmt.Errorf("prompt %d references unknown session: %s", p.ID, p.SessionID)
        }
    }

    return nil
}
```

### Consideraciones de IDs en import

Las observaciones y prompts tienen `id INTEGER PRIMARY KEY AUTOINCREMENT`. Al importar, se inserta el ID explícito. Si el ID ya existe (colisión), el INSERT fallará y la transacción hará rollback. Esto es deliberado: preferimos fallar a perder datos.

Una alternativa es no insertar el ID y dejar que SQLite lo asigne, pero eso rompe la fidelidad del export/import. Para este diseño, insertamos con ID original. Si hay colisión, el usuario debe importar en una DB vacía o manejar la colisión manualmente.

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Export asíncrono con streaming | Overkill para volúmenes esperados (< 100k registros); Marshal es suficiente |
| Import sin transacción | Riesgo de estado inconsistente si falla a medio camino |
| `INSERT OR REPLACE` en sessions | Podría sobrescribir sesiones existentes; `INSERT OR IGNORE` es más seguro |
| Validación inline durante INSERT | Validar antes es más eficiente y da errores más claros |
| Export comprimido (gzip) | No es responsabilidad del store; CLI puede hacer piping |
| Incluir sync_chunks, memory_relations | Son tablas internas de sync, no representan "memoria del usuario" |
| Formato CSV en vez de JSON | JSON es más expresivo, soporta anidación y es el estándar en el ecosistema |

## TDD plan

1. **Red:** Test Export con datos → falla sin implementación
2. **Green:** Implementar Export básico con consultas → pasa
3. **Red:** Test Export con filtro project
4. **Green:** Agregar WHERE project = ? → pasa
5. **Red:** Test Export excluye soft-delete
6. **Green:** Agregar WHERE deleted_at IS NULL → pasa
7. **Red:** Test Import exitoso (round-trip)
8. **Green:** Implementar Import con tx → pasa
9. **Red:** Test Import JSON inválido
10. **Green:** json.Unmarshal falla → error "invalid JSON"
11. **Red:** Test Import falta campo sessions
12. **Green:** validateExportData detecta nil → pasa
13. **Red:** Test Import atómico (error a medio tx → rollback)
14. **Green:** Insert falla → tx.Rollback → DB sin cambios
15. **Red:** Test Import INSERT OR IGNORE sessions duplicadas
16. **Green:** Sesión existente no se duplica → pasa
17. **Sabotaje:** No validar sessions antes de insertar → error FK → test cae → restaurar → pasa

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| JSON enorme en memoria | Para < 100k registros, Marshal es suficiente; benchmark antes de optimizar |
| Colisión de IDs en import | Transacción revierte todo si hay colisión; documentar que import a DB no vacía puede fallar |
| FK violation en import | Validación previa de referencias session_id |
| Import lento con muchos datos | Prepared statements dentro de tx; si persiste, batch INSERT con chunks |
