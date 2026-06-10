# Tasks: issue-09.6-cloud-audit-admin

## Backend

- [ ] **B1: Crear migración 002 para audit log y paused projects**
      - `cloud_sync_audit_log` table
      - `cloud_project_pauses` table
      - Índices

- [ ] **B2: Implementar AuditLog helper**
      - `internal/cloud/server/audit.go`
      - Función `AuditLog(ctx, db, entry)` que inserta registro

- [ ] **B3: Integrar audit logging en sync handlers**
      - Push handler → log action=sync.push con status y entry_count
      - Pull handler → log action=sync.pull con entry_count
      - Mutations handler → log action=sync.mutation

- [ ] **B4: Implementar pauseMiddleware**
      - Extraer project de request (body o query)
      - Check en cloud_project_pauses WHERE active=TRUE
      - Si paused → 403 con mensaje claro

- [ ] **B5: Implementar admin pause/resume endpoints**
      - POST /admin/projects/{project}/pause
      - POST /admin/projects/{project}/resume
      - Audit log de cada acción

- [ ] **B6: Implementar /admin/audit handler**
      - Keyset pagination (created_at DESC, id DESC)
      - Filtros: action, status, project
      - templ component de audit table

- [ ] **B7: Implementar /admin/projects dashboard page**
      - Lista de proyectos con status (active/paused)
      - Botones pause/resume con HTMX
      - templ components

## Tests

- [ ] **T1: Audit log se inserta después de push exitoso**
- [ ] **T2: Audit log registra errores**
- [ ] **T3: Paused project → 403 en push**
- [ ] **T4: Active project pasa pause middleware**
- [ ] **T5: Resume reactiva proyecto**
- [ ] **T6: Pausa persiste después de "restart" (re-crear pool)**
- [ ] **T7: Admin audit page retorna entries paginados**
- [ ] **T8: Filtros en audit page funcionan**
- [ ] **T9: Sabotaje — no checkear active=TRUE → middleware nunca bloquea → test falla → restaurar**

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/cloud/... -v`
- [ ] Commit: `feat: audit logging and project pause controls`
