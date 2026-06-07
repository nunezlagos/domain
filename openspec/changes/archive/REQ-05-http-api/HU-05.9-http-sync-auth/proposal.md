# Proposal: HU-05.9-http-sync-auth

## Intención

Implementar dos funcionalidades transversales: (1) endpoint GET /sync/status con información del estado de sincronización (sync_state con reason codes, upgrade stage, metadata), (2) middleware de autenticación Bearer token que protege rutas sensibles (DELETE, EXPORT, IMPORT, MIGRATE).

## Scope

**Incluye:**
- `GET /sync/status` — sync state (idle/syncing/error), reason (texto), reason_code (int), upgrade_stage (none/migrating/complete), last_sync_at, pending_chunks count
- Middleware `RequireToken` que lee `ENGRAM_HTTP_TOKEN` del entorno
- Protección de rutas:
  - DELETE /sessions/{id}
  - DELETE /observations/{id}
  - DELETE /prompts/{id}
  - GET /export
  - POST /import
  - POST /projects/migrate
  - POST /conflicts/scan (opcional)
- 500 si ENGRAM_HTTP_TOKEN no está configurado y se accede a ruta protegida
- 401 si token inválido o ausente

**No incluye:**
- Autenticación para GET /health, GET /stats, GET /sync/status, GET /search, GET /context
- Tokens rotativos o expiración
- Múltiples tokens o usuarios
- HTTPS (asumimos localhost)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Auth middleware | `func RequireToken(next http.Handler) http.Handler` — lee ENGRAM_HTTP_TOKEN, compara con Bearer header |
| Token source | Variable de entorno `ENGRAM_HTTP_TOKEN` |
| Sync status source | `sync.GetStatus()` de HU-07.3; si no implementado, retornar valores default |
| Reason codes | 0=idle, 1=syncing, 2=error, 3=conflict |
| Upgrade stages | "none", "migrating", "complete", "rollback" |

```go
type SyncStatus struct {
    SyncState    string `json:"sync_state"`    // idle, syncing, error
    Reason       string `json:"reason"`
    ReasonCode   int    `json:"reason_code"`
    UpgradeStage string `json:"upgrade_stage"` // none, migrating, complete
    LastSyncAt   string `json:"last_sync_at"`
    PendingChunks int   `json:"pending_chunks"`
}

// Reason codes
const (
    ReasonIdle     = 0
    ReasonSyncing  = 1
    ReasonError    = 2
    ReasonConflict = 3
)
```

### Routes protected

```go
var ProtectedRoutes = []string{
    "DELETE /sessions/{id}",
    "DELETE /observations/{id}",
    "DELETE /prompts/{id}",
    "GET /export",
    "POST /import",
    "POST /projects/migrate",
}

// All other routes are public (GET /health, /stats, /sync/status, /search, etc.)
```

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| Token en env var se loguea | Media | No loguear headers de Authorization; sanitizar en logs |
| Token hardcodeado en docker-compose | Alta | Doc recomienda usar secrets o .env; warning si token es "changeme" |
| Sync status sin implementación de sync | Alta | Retornar valores default (idle, reason_code=0); no fallar |
| Olvidar proteger una ruta DELETE | Media | Code review + test que verifica que toda ruta DELETE requiere auth |

## Testing

- **Sync status:** GET /sync/status → 200, sync_state, reason_code, upgrade_stage
- **Sync status default:** Sin sync implementado → idle, code=0
- **Auth success:** DELETE con token válido → 204
- **Auth missing:** DELETE sin token → 401
- **Auth invalid:** DELETE con token incorrecto → 401
- **Auth no token env:** Sin ENGRAM_HTTP_TOKEN → protected routes retornan 500
- **Public routes:** GET /health, /stats, /sync/status sin token → 200
- **Sabotaje:** No proteger DELETE → cualquiera borra sin token → test cae → restaurar
