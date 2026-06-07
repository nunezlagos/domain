# Design: HU-09.6-cloud-audit-admin

## Decisión arquitectónica

### Audit log schema

```sql
-- migration 002_audit_log

CREATE TABLE IF NOT EXISTS cloud_sync_audit_log (
    id              BIGSERIAL PRIMARY KEY,
    action          TEXT NOT NULL,        -- sync.push, sync.pull, sync.mutation, admin.pause, admin.resume
    status          TEXT NOT NULL DEFAULT 'success', -- success, error
    enrollment_id   TEXT,
    project         TEXT,
    entry_count     INTEGER,
    error_detail    JSONB,
    ip_address      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_action ON cloud_sync_audit_log(action);
CREATE INDEX IF NOT EXISTS idx_audit_status ON cloud_sync_audit_log(status);
CREATE INDEX IF NOT EXISTS idx_audit_project ON cloud_sync_audit_log(project);
CREATE INDEX IF NOT EXISTS idx_audit_created ON cloud_sync_audit_log(created_at DESC);

CREATE TABLE IF NOT EXISTS cloud_project_pauses (
    project     TEXT PRIMARY KEY,
    paused_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    paused_by   TEXT NOT NULL,            -- enrollment_id del admin
    reason      TEXT,
    active      BOOLEAN NOT NULL DEFAULT TRUE
);
```

### Audit helper

```go
type AuditEntry struct {
    Action      string          `json:"action"`
    Status      string          `json:"status"`
    EnrollmentID string         `json:"enrollment_id,omitempty"`
    Project     string          `json:"project,omitempty"`
    EntryCount  int             `json:"entry_count,omitempty"`
    ErrorDetail json.RawMessage `json:"error_detail,omitempty"`
    IPAddress   string          `json:"-"`
}

func AuditLog(ctx context.Context, db *pgxpool.Pool, entry AuditEntry) error {
    _, err := db.Exec(ctx,
        `INSERT INTO cloud_sync_audit_log
         (action, status, enrollment_id, project, entry_count, error_detail, ip_address)
         VALUES ($1, $2, $3, $4, $5, $6, $7)`,
        entry.Action, entry.Status, entry.EnrollmentID,
        entry.Project, entry.EntryCount, entry.ErrorDetail, entry.IPAddress,
    )
    return err
}
```

### Paused projects middleware

```go
func pauseMiddleware(db *pgxpool.Pool, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Extract project from request body or query
        project := extractProject(r)
        if project == "" {
            next.ServeHTTP(w, r)
            return
        }

        var paused bool
        err := db.QueryRow(r.Context(),
            `SELECT EXISTS(SELECT 1 FROM cloud_project_pauses WHERE project = $1 AND active = TRUE)`,
            project,
        ).Scan(&paused)

        if err == nil && paused {
            http.Error(w, `{"error":"sync paused for project: `+project+`"}`, 403)
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

### Admin dashboard pages

```
GET /admin/audit?action=sync.push&status=error&page=2
  → Table: ID | Action | Status | Enrollment | Project | Timestamp | Detail
  → Filtros: action (select), status (select), project (input), date range
  → Paginación: "Older" button (HTMX)

POST /admin/projects/{project}/pause
  → Inserta/actualiza cloud_project_pauses
  → Registra audit entry action=admin.pause
  → Retorna 200 + reemplaza botón "Pause" por "Resume"

POST /admin/projects/{project}/resume
  → UPDATE cloud_project_pauses SET active = FALSE
  → Registra audit entry action=admin.resume
  → Retorna 200 + reemplaza botón "Resume" por "Pause"
```

### Keyset pagination

```go
func queryAuditLog(ctx context.Context, db *pgxpool.Pool, filters AuditFilters, cursor *Cursor, limit int) ([]AuditEntry, *Cursor, error) {
    query := `SELECT id, action, status, enrollment_id, project, entry_count, error_detail, created_at
              FROM cloud_sync_audit_log WHERE 1=1`
    args := []interface{}{}
    argN := 1

    if filters.Action != "" {
        query += fmt.Sprintf(" AND action = $%d", argN); argN++
        args = append(args, filters.Action)
    }
    if filters.Status != "" {
        query += fmt.Sprintf(" AND status = $%d", argN); argN++
        args = append(args, filters.Status)
    }
    if filters.Project != "" {
        query += fmt.Sprintf(" AND project = $%d", argN); argN++
        args = append(args, filters.Project)
    }
    if cursor != nil {
        query += fmt.Sprintf(" AND (created_at, id) < ($%d, $%d)", argN, argN+1); argN += 2
        args = append(args, cursor.Timestamp, cursor.ID)
    }
    query += " ORDER BY created_at DESC, id DESC LIMIT " + fmt.Sprintf("$%d", argN)
    args = append(args, limit+1)

    // Execute, check for hasMore
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Audit log en archivo plano | Difícil de filtrar y paginar desde dashboard; Postgres permite queries estructuradas |
| Offset pagination | Keyset es más performante para tablas grandes; evita offset drift |
| Redis para paused projects | Necesita persistencia; Postgres ya está disponible |

## TDD plan

1. **Red:** Audit log insert después de push → falla
2. **Green:** Implementar AuditLog helper + call en push handler → pasa
3. **Red:** Paused project rechaza push con 403 → falla
4. **Green:** Implementar pauseMiddleware → pasa
5. **Red:** Admin page /admin/audit retorna entries paginados → falla
6. **Green:** Implementar handler con keyset pagination → pasa
7. **Sabotaje:** No checkear active=TRUE en pause query → paused projects no efectivos → test cae → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Audit log sin límite de tamaño | TTL cleanup futura; por ahora, índice en created_at permite DELETE eficiente |
| Race condition en pause/resume concurrente | UPDATE es atómico en Postgres; no hay race |
| Paused project check impacta performance | Query existencial simple con índice PK; < 1ms |
