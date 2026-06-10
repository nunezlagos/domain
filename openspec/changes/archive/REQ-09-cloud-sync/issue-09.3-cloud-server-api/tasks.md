# Tasks: issue-09.3-cloud-server-api

## Backend

- [ ] **B1: Crear paquete `internal/cloud/server/`**
      - `serve.go` — Serve command, server startup
      - `handlers.go` — sync push/pull/mutations handlers
      - `auth.go` — JWT middleware
      - `migrations.go` — Postgres schema migrations (embed)
      - `db.go` — Postgres connection pool

- [ ] **B2: Implementar Postgres schema y auto-migrate**
      - SQL migrations embebidas via `//go:embed`
      - Tablas: cloud_enrollments, cloud_sync_entries, cloud_audit_log
      - RunMigrationsPostgres() al iniciar serve

- [ ] **B3: Implementar JWT auth middleware**
      - Extraer Bearer token de Authorization header
      - Validar con ENGRAM_JWT_SECRET
      - Extraer claims (enrollment_id) y poner en context

- [ ] **B4: Implementar POST /api/sync/push**
      - Recibir array de SyncEntry
      - Validar allowed projects
      - Upsert en cloud_sync_entries en transacción
      - Retornar accepted count + server_timestamp

- [ ] **B5: Implementar GET /api/sync/pull**
      - Query param `since` (RFC3339), `project` (opcional)
      - Query cloud_sync_entries con paginación
      - Retornar entries + server_timestamp

- [ ] **B6: Implementar POST /api/sync/mutations**
      - Recibir lista de operaciones MUT
      - Aplicar cada operación: Merge, Update, Tag
      - Retornar resultados por operación

- [ ] **B7: Implementar GET /health**
      - Ping a Postgres
      - Retornar status, database, version

- [ ] **B8: Implementar AllowedProjects filter**
      - Parsear ENGRAM_CLOUD_ALLOWED_PROJECTS
      - Validar en push y mutations

- [ ] **B9: Implementar `engram cloud serve` CLI**
      - Leer config de env vars
      - Conectar Postgres
      - Iniciar HTTP server

## Tests

- [ ] **T1: Health endpoint con Postgres mock**
- [ ] **T2: Push de entries exitoso**
- [ ] **T3: Pull retorna entries desde timestamp**
- [ ] **T4: JWT inválido → 401**
- [ ] **T5: JWT válido → 200**
- [ ] **T6: Project no permitido → 403**
- [ ] **T7: Payload excede límite → 413**
- [ ] **T8: Push en transacción (rollback en error)**
- [ ] **T9: Sabotaje — JWT accept any → test 401 falla → restaurar**

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/cloud/server/... -v`
- [ ] Commit: `feat: cloud server with Postgres, sync endpoints, and JWT auth`
