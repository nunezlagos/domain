# Proposal: issue-09.3-cloud-server-api

## Intención

Implementar el servidor cloud de memoria con Postgres backend, endpoints de sincronización (push, pull, mutations), autenticación JWT, y filtro de proyectos permitidos. El servidor es autónomo (no requiere proxy) y se inicia con `engram cloud serve`.

## Scope

**Incluye:**
- `engram cloud serve` command
- Postgres schema: `cloud_sync_entries`, `enrollments`, `audit_log`
- Endpoints: POST /api/sync/push, GET /api/sync/pull, POST /api/sync/mutations
- JWT auth middleware + ENGRAM_JWT_SECRET
- ENGRAM_CLOUD_ALLOWED_PROJECTS filter
- GET /health endpoint
- Auto-migración de schema Postgres al iniciar

**No incluye:**
- Dashboard web (issue-09.4)
- Autosync background (issue-09.5)
- Audit admin (issue-09.6)
- Load balancing, rate limiting, TLS termination (asume reverse proxy)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Postgres driver | `pgx/v5` — moderno, performante, sin ORM |
| JWT | `golang-jwt/jwt/v5` — estándar de la industria |
| Auto-migrate | SQL embebido (`embed.FS`), migraciones versionadas igual que SQLite |
| HTTP router | `net/http` + `chi` o `http.ServeMux` (depende de lo que ya use el proyecto) |
| Sync push | Chunks de observaciones como JSON array, upsert por sync_id |
| Sync pull | Query con `WHERE updated_at > $1`, paginado con LIMIT/OFFSET |

