# Proposal: HU-09.6-cloud-audit-admin

## Intención

Agregar un log de auditoría persistente para todas las operaciones de sync, y controles administrativos para pausar/reanudar proyectos específicos. Dashboard pages para visualizar y filtrar el audit log.

## Scope

**Incluye:**
- Tabla `cloud_sync_audit_log` en Postgres
- Audit middleware/helper que registra operaciones
- GET /admin/audit con paginación y filtros
- Tabla `cloud_project_pauses` + middleware check en push/pull
- Dashboard page /admin/projects con pause/resume controls
- Persistencia de paused projects en Postgres

**No incluye:**
- TTL/cleanup automático de audit log (futuro)
- Rate limiting (HU-09.3 ya lo considera)
- Notificaciones de pause (futuro)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Audit table | cloud_sync_audit_log con action, status, detail (JSONB), enrollment_id, project, created_at |
| Audit helper | `AuditLog(db, action, status, detail)` function, llamada desde handlers |
| Paused projects | Tabla cloud_project_pauses (project TEXT PK, paused_at, paused_by) + middleware |
| Admin pages | HTMX templ components: audit table con filtros, project status cards |
| Pagination | Keyset pagination (created_at + id) para audit log |

