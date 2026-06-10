# Design: issue-09.3-cloud-server-api

## Decisión arquitectónica

### Schema Postgres

```sql
-- migration 001_cloud_init

CREATE TABLE IF NOT EXISTS cloud_enrollments (
    id              TEXT PRIMARY KEY,
    machine_id      TEXT NOT NULL UNIQUE,
    hostname        TEXT NOT NULL,
    version         TEXT NOT NULL,
    enrolled_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status          TEXT NOT NULL DEFAULT 'active'
);

CREATE TABLE IF NOT EXISTS cloud_sync_entries (
    sync_id         TEXT PRIMARY KEY,
    enrollment_id   TEXT NOT NULL REFERENCES cloud_enrollments(id),
    project         TEXT NOT NULL,
    entity          TEXT NOT NULL,
    payload         JSONB NOT NULL,
    operation       TEXT NOT NULL DEFAULT 'upsert', -- upsert, delete, merge
    checksum        TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sync_enrollment ON cloud_sync_entries(enrollment_id);
CREATE INDEX IF NOT EXISTS idx_sync_project ON cloud_sync_entries(project);
CREATE INDEX IF NOT EXISTS idx_sync_updated ON cloud_sync_entries(updated_at);

CREATE TABLE IF NOT EXISTS cloud_audit_log (
    id              BIGSERIAL PRIMARY KEY,
    enrollment_id   TEXT REFERENCES cloud_enrollments(id),
    action          TEXT NOT NULL,
    detail          JSONB,
    ip_address      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_enrollment ON cloud_audit_log(enrollment_id);
CREATE INDEX IF NOT EXISTS idx_audit_created ON cloud_audit_log(created_at);
```

### JWT auth middleware

```go
func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        tokenStr := extractBearerToken(r)
        if tokenStr == "" {
            http.Error(w, "missing authorization header", 401)
            return
        }
        token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
            if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
                return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
            }
            return []byte(os.Getenv("ENGRAM_JWT_SECRET")), nil
        })
        if err != nil || !token.Valid {
            http.Error(w, "invalid or expired token", 401)
            return
        }
        claims := token.Claims.(jwt.MapClaims)
        ctx := context.WithValue(r.Context(), ctxKeyClaims, claims)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### Sync push handler

```go
type SyncPushRequest struct {
    Entries []SyncEntry `json:"entries"`
}

type SyncEntry struct {
    SyncID    string          `json:"sync_id"`
    Project   string          `json:"project"`
    Entity    string          `json:"entity"`
    Payload   json.RawMessage `json:"payload"`
    Operation string          `json:"operation"`
    Checksum  string          `json:"checksum"`
}

func handleSyncPush(w http.ResponseWriter, r *http.Request) {
    enrollmentID := getEnrollmentID(r.Context())
    var req SyncPushRequest
    json.NewDecoder(r.Body).Decode(&req)

    // Check allowed projects
    allowed := getAllowedProjects()
    for _, entry := range req.Entries {
        if !isProjectAllowed(entry.Project, allowed) {
            http.Error(w, fmt.Sprintf("project not allowed: %s", entry.Project), 403)
            return
        }
    }

    tx, _ := db.BeginTx(r.Context(), nil)
    for _, entry := range req.Entries {
        tx.Exec(r.Context(),
            `INSERT INTO cloud_sync_entries
             (sync_id, enrollment_id, project, entity, payload, operation, checksum, updated_at)
             VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
             ON CONFLICT (sync_id) DO UPDATE SET
             payload = EXCLUDED.payload, updated_at = NOW()`,
            entry.SyncID, enrollmentID, entry.Project, entry.Entity,
            entry.Payload, entry.Operation, entry.Checksum)
    }
    tx.Commit()

    json.NewEncoder(w).Encode(map[string]interface{}{
        "accepted": len(req.Entries),
        "server_timestamp": time.Now().UTC().Format(time.RFC3339),
    })
}
```

### Sync pull handler

```go
func handleSyncPull(w http.ResponseWriter, r *http.Request) {
    since := r.URL.Query().Get("since")
    project := r.URL.Query().Get("project")
    limit := 1000 // default

    query := `SELECT sync_id, project, entity, payload, operation, checksum, updated_at
              FROM cloud_sync_entries
              WHERE updated_at > $1`
    args := []interface{}{since}
    argN := 2

    if project != "" {
        query += fmt.Sprintf(" AND project = $%d", argN)
        args = append(args, project)
        argN++
    }
    query += " ORDER BY updated_at ASC LIMIT $" + fmt.Sprintf("%d", argN)
    args = append(args, limit)

    rows, _ := db.Query(r.Context(), query, args...)
    defer rows.Close()

    var entries []SyncEntryResponse
    for rows.Next() {
        var e SyncEntryResponse
        rows.Scan(&e.SyncID, &e.Project, &e.Entity, &e.Payload, &e.Operation, &e.Checksum, &e.UpdatedAt)
        entries = append(entries, e)
    }

    json.NewEncoder(w).Encode(map[string]interface{}{
        "entries": entries,
        "server_timestamp": time.Now().UTC().Format(time.RFC3339),
        "count": len(entries),
    })
}
```

### Allowed projects filter

```go
func getAllowedProjects() []string {
    raw := os.Getenv("ENGRAM_CLOUD_ALLOWED_PROJECTS")
    if raw == "" {
        return nil // allow all
    }
    return strings.Split(raw, ",")
}

func isProjectAllowed(project string, allowed []string) bool {
    if allowed == nil {
        return true
    }
    for _, a := range allowed {
        if strings.TrimSpace(a) == project {
            return true
        }
    }
    return false
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| SQLite como backend cloud | Postgres es standard para multi-cliente; SQLite no escala para concurrencia cloud |
| REST vs GraphQL | REST es suficiente para sync operations; GraphQL agrega complejidad innecesaria |
| Protobuf/gRPC | Latency no crítica; JSON es más debuggeable y compatible con cualquier cliente HTTP |

## TDD plan

1. **Red:** POST /api/sync/push persiste entries → falla
2. **Green:** Implement handler + Postgres insert → pasa
3. **Red:** JWT inválido retorna 401 → falla
4. **Green:** Implement auth middleware → pasa
5. **Red:** Allowed projects filter retorna 403 → falla
6. **Green:** Implement isProjectAllowed → pasa
7. **Sabotaje:** Romper JWT validation (accept any token) → test 401 falla → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Postgres no disponible al iniciar | Retry con backoff (3 intentos, 2s intervalo); si falla, error claro |
| Payload grande en push (>10MB) | Limitar tamaño de request body a 10MB; chunking en cliente |
| JWT secret débil | Warning si ENGRAM_JWT_SECRET < 32 chars; recomendar generación segura |
| Inyección SQL en queries | Siempre usar parámetros ($1, $2); nunca concatenar input directamente |
