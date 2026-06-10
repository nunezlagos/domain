# Tasks: issue-02.4-audit-log

## Backend

- [ ] Agregar migración 021 para tabla `audit_log` (BIGSERIAL PK, columnas, índices)
- [ ] Agregar trigger de inmutabilidad (reject UPDATE/DELETE)
- [ ] Crear `internal/audit/store.go` con AuditStore interface
- [ ] Implementar `Log(ctx, entry)` — INSERT
- [ ] Implementar `Query(ctx, filter)` — SELECT con filtros y cursor
- [ ] Implementar `Prune(ctx, before)` — DELETE batch
- [ ] Crear `internal/api/middleware/audit.go` para extraer IP y exponer logger
- [ ] Integrar audit logging en handlers después de operaciones exitosas
- [ ] Agregar endpoint `GET /api/v1/audit-logs` (protegido con RBAC admin)
- [ ] Agregar comando CLI `domain audit prune` con flag `--dry-run`
- [ ] Agregar env `DOMAIN_AUDIT_RETENTION_DAYS` (default 90)

## Tests

- [ ] Test unitario: Log escribe registro correctamente
- [ ] Test unitario: Query por actor_id
- [ ] Test unitario: Query por entity_type + action
- [ ] Test unitario: Query con cursor pagination
- [ ] Test integración: trigger rechaza UPDATE
- [ ] Test integración: trigger rechaza DELETE
- [ ] Test integración: Prune respeta retention days
- [ ] Test unitario: handler registra audit log en éxito
- [ ] Test unitario: handler no registra audit log en error
- [ ] Sabotaje: no crear trigger → confirmar que test de UPDATE falla → restaurar
- [ ] Sabotaje: omitir índice en entity_type → medir performance → agregar índice

## Cierre

- [ ] Verificación manual: realizar acciones, consultar audit logs
- [ ] Suite verde
