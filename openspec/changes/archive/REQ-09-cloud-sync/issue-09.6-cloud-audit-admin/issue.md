# issue-09.6-cloud-audit-admin

**Origen:** `REQ-09-cloud-sync`
**Prioridad:** baja
**Tipo:** feature

## Historia de usuario

**Como** administrador del cloud
**Quiero** un log de auditoría de todas las operaciones de sync
**Para** poder investigar incidentes y rastrear cambios

**Como** administrador
**Quiero** poder pausar la sincronización de proyectos específicos
**Para** evitar que proyectos problemáticos afecten al resto

## Criterios de aceptación

```gherkin
Scenario: Audit log registra push operations
  Given un cliente ejecuta un push exitoso
  When se completa la operación
  Then cloud_sync_audit_log tiene un entry con action="sync.push", enrollment_id, project, entry_count, status="success"

Scenario: Audit log registra pull operations
  Given un cliente ejecuta un pull
  When se completa la operación
  Then cloud_sync_audit_log tiene un entry con action="sync.pull", enrollment_id, project, entry_count

Scenario: Audit log registra errores de sync
  Given un push falla por auth_error
  When se registra el error
  Then cloud_sync_audit_log tiene un entry con action="sync.push", status="error", error_detail con reason_code

Scenario: Admin dashboard muestra audit log
  Given hay entries en el audit log
  When navego a /admin/audit
  Then veo tabla paginada con: timestamp, action, enrollment, project, status, detail

Scenario: Admin dashboard permite filtrar audit log
  Given hay entries de distintos tipos
  When navego a /admin/audit?action=sync.push&status=error
  Then veo solo entries que matchean ambos filtros

Scenario: Admin puede pausar sync de un proyecto
  Given estoy en /admin/projects
  When hago click en "Pause sync" para project-x
  Then project-x aparece como "paused"
  And los push/pull de project-x son rechazados con 403 "sync paused"

Scenario: Admin puede reanudar sync de un proyecto
  Given project-x está en paused
  When hago click en "Resume sync"
  Then project-x aparece como "active"
  And los push/pull de project-x son aceptados nuevamente

Scenario: Paused projects se persisten
  Given project-x está paused
  When el servidor se reinicia
  Then project-x sigue en paused después del reinicio

Scenario: Intentar push a proyecto paused retorna error
  Given project-x está paused
  When un cliente hace push a project-x
  Then retorna 403 Forbidden con detail "sync paused for project: project-x"

Scenario: Audit log pagina correctamente
  Given hay más de 100 entries en audit log
  When navego a /admin/audit?page=2
  Then veo los siguientes 100 entries
```

## Análisis breve

- **Qué pide realmente:** Tabla cloud_sync_audit_log, middleware de audit logging, admin dashboard pages (audit log, project pause controls), persistencia de paused projects
- **Módulos sospechados:** `internal/cloud/server/audit.go`, `internal/cloud/dashboard/admin.go`, `internal/cloud/server/project_pause.go`
- **Riesgos / dependencias:** Audit log puede crecer rápido; necesita TTL/cleanup
- **Esfuerzo tentativo:** M

## Verificación previa

- [ ] Revisar codebase (grep)
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:** —
- **Acción derivada:** —
