# Design: issue-05.9-http-sync-auth

## Decisión arquitectónica

### Auth middleware

```go
package api

import (
    "net/http"
    "os"
    "strings"
)

var errTokenNotConfigured = apiError{500, "ENGRAM_HTTP_TOKEN not configured"}
var errUnauthorized = apiError{401, "invalid or missing Bearer token"}

func RequireToken(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := os.Getenv("ENGRAM_HTTP_TOKEN")
        if token == "" {
            // Token not configured at all — fail with 500
            writeError(w, errTokenNotConfigured)
            return
        }

        auth := r.Header.Get("Authorization")
        if !strings.HasPrefix(auth, "Bearer ") {
            writeError(w, errUnauthorized)
            return
        }

        provided := strings.TrimPrefix(auth, "Bearer ")
        if provided != token {
            writeError(w, errUnauthorized)
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

### SyncStatusRepo interface

```go
type SyncStatusRepo interface {
    GetStatus(ctx context.Context) (SyncStatus, error)
}

type SyncStatus struct {
    SyncState     string `json:"sync_state"`
    Reason        string `json:"reason"`
    ReasonCode    int    `json:"reason_code"`
    UpgradeStage  string `json:"upgrade_stage"`
    LastSyncAt    string `json:"last_sync_at"`
    PendingChunks int    `json:"pending_chunks"`
}
```

### Integration with issue-07.3

El handler de sync status delega en `sync.GetStatus()` de issue-07.3. Si el módulo no existe aún, retornar estado default:

```go
func (r *syncStatusRepo) GetStatus(ctx context.Context) (SyncStatus, error) {
    // Attempt to delegate to sync module (issue-07.3)
    // If not available, return default status
    status := SyncStatus{
        SyncState:    "idle",
        Reason:       "no sync configured",
        ReasonCode:   0,
        UpgradeStage: "none",
        LastSyncAt:   "",
        PendingChunks: 0,
    }

    // Try to use sync package if available
    if s, err := sync.GetStatus(ctx, r.db); err == nil {
        return s, nil
    }

    return status, nil
}
```

### Applying auth middleware to protected routes

El auth middleware se aplica a nivel de registro de rutas en el main server setup:

```go
func NewServer(db *sql.DB) *http.ServeMux {
    mux := http.NewServeMux()

    // Repos
    sessionRepo := store.NewSessionRepo(db)
    obsRepo := store.NewObservationRepo(db)
    // ... etc

    // Public routes
    RegisterSessionRoutes(mux, sessionRepo)      // POST, GET only
    RegisterObservationRoutes(mux, obsRepo)      // POST, GET, PATCH
    RegisterPromptRoutes(mux, promptRepo)
    RegisterSearchRoutes(mux, searchRepo)
    RegisterStatsRoutes(mux, statsRepo)
    RegisterConflictRoutes(mux, conflictRepo)

    // Auth-protected routes
    auth := RequireToken
    RegisterProtectedSessionRoutes(mux, sessionRepo, auth)   // DELETE
    RegisterProtectedObservationRoutes(mux, obsRepo, auth)   // DELETE with hard
    RegisterProtectedPromptRoutes(mux, promptRepo, auth)     // DELETE
    RegisterExportRoutes(mux, exportRepo, auth)
    RegisterProjectRoutes(mux, projectRepo, auth)

    // Sync status (public)
    RegisterSyncRoutes(mux, syncRepo)

    return mux
}
```

Alternativamente, un wrapper más simple: cada handler sensible se envuelve individualmente.

### Route registration for sync

```go
func RegisterSyncRoutes(mux *http.ServeMux, repo SyncStatusRepo) {
    mux.HandleFunc("GET /sync/status", handleSyncStatus(repo))
}
```

## Reason codes specification

| Code | Name | Description |
|------|------|-------------|
| 0 | IDLE | No sync operation in progress or pending |
| 1 | SYNCING | Sync operation actively running |
| 2 | ERROR | Last sync failed with error |
| 3 | CONFLICT | Sync completed with merge conflicts |
| 4 | PENDING | Chunks pending sync (not yet pushed) |

## Upgrade stages

| Stage | Description |
|-------|-------------|
| "none" | No schema upgrade has been needed |
| "migrating" | Schema migration is in progress |
| "complete" | Last migration completed successfully |
| "rollback" | Migration was rolled back due to error |

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Basic Auth en vez de Bearer | Bearer es estándar en REST APIs; Basic Auth envía credenciales en cada request |
| JWT tokens | Overkill para server local con un solo token estático |
| API Key en query param | Menos seguro que header; se loguea en URLs |
| Auth en todas las rutas | GET /health, /stats, /sync/status deben ser accesibles sin auth para monitoreo |
| Config file para token | Env var es más seguro (no se commitea), sigue 12-factor app |

## Diagrama

```
Client HTTP                          memoria server (localhost:7437)
    |                                          |
    | [Public] GET /health                      |
    | [Public] GET /stats                       |
    | [Public] GET /sync/status                  |
    | [Public] GET /search, /context, /timeline  |
    | [Public] POST /sessions (create)           |
    | [Public] POST /observations (create)       |
    |                                          |
    | [Auth: Bearer <token>] DELETE /sessions   |
    | [Auth: Bearer <token>] DELETE /observations|
    | [Auth: Bearer <token>] GET /export         |
    | [Auth: Bearer <token>] POST /import         |
    | [Auth: Bearer <token>] POST /projects/migrate|
    |                                          |
    | 1. Request arrives                         |
    | 2. RequireToken middleware checks:          |
    |    a. ENGRAM_HTTP_TOKEN exists? -> 500     |
    |    b. Authorization: Bearer matches? -> 401|
    |    c. OK -> next handler                    |
    |                                          |
    +--------> api/middleware.go -------------> api/handlers
                                                    |
                                                SQLite DB
```

## TDD plan

1. **Red:** Test GET /sync/status → 200, all fields → falla
2. **Green:** Sync handler → pasa
3. **Red:** Test DELETE sin token → 401 → falla
4. **Green:** RequireToken middleware → pasa
5. **Red:** Test DELETE con token válido → 204 → falla
6. **Green:** Middleware permite token correcto → pasa
7. **Red:** Test DELETE con token inválido → 401 → falla
8. **Green:** Middleware rechaza token incorrecto → pasa
9. **Red:** Test GET /health sin token → 200 → falla (si protegimos todo)
10. **Green:** Asegurar rutas públicas no pasan por middleware → pasa
11. **Sabotaje:** No proteger DELETE → cualquiera borra sin token → test cae → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Token no configurado → rutas protegidas dan 500 | Documentar que ENGRAM_HTTP_TOKEN es requerido para DELETE/export/import |
| Middleware aplicado incorrectamente a ruta pública | Test explícito que verifica GET /health sin token funciona |
| sync.GetStatus() no implementado | Fallback a valores default; no bloquear el endpoint |
| Reason code inconsistente con issue-07.3 | SyncStatus struct compartido entre ambos módulos |
