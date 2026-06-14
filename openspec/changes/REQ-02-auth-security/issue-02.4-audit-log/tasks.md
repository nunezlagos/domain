# Tasks: issue-02.4-audit-log

## Backend

- [x] Agregar migración 021 para tabla `audit_log` (BIGSERIAL PK, columnas, índices)
- [x] Agregar trigger de inmutabilidad (reject UPDATE/DELETE)
- [x] Crear `internal/audit/store.go` con AuditStore interface
- [x] Implementar `Log(ctx, entry)` — INSERT
- [x] Implementar `Query(ctx, filter)` — SELECT con filtros y cursor
- [x] Implementar `Prune(ctx, before)` — DELETE batch
- [x] Crear `internal/api/middleware/audit.go` para extraer IP y exponer logger
- [x] Integrar audit logging en handlers después de operaciones exitosas
- [x] Agregar endpoint `GET /api/v1/audit-logs` (protegido con RBAC admin)
- [x] Agregar comando CLI `domain audit prune` con flag `--dry-run`
- [x] Agregar env `DOMAIN_AUDIT_RETENTION_DAYS` (default 90)

## Tests

- [x] Test unitario: Log escribe registro correctamente
- [x] Test unitario: Query por actor_id
- [x] Test unitario: Query por entity_type + action
- [x] Test unitario: Query con cursor pagination
- [x] Test integración: trigger rechaza UPDATE
- [x] Test integración: trigger rechaza DELETE
- [x] Test integración: Prune respeta retention days
- [x] Test unitario: handler registra audit log en éxito
- [x] Test unitario: handler no registra audit log en error
- [x] Sabotaje: no crear trigger → confirmar que test de UPDATE falla → restaurar
- [x] Sabotaje: omitir índice en entity_type → medir performance → agregar índice

## Cierre

- [x] Verificación manual: realizar acciones, consultar audit logs
- [x] Suite verde
