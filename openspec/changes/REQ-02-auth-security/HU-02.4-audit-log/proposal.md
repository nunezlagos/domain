# Proposal: HU-02.4-audit-log

## Intención

Implementar un sistema de auditoría inmutable que registre cada operación relevante del sistema. Los registros son append-only y permiten consultas por actor, entidad y acción. Política de retención configurable con purge de datos antiguos.

## Scope

**Incluye:**
- Tabla `audit_log` con columnas: id (BIGSERIAL), actor_id (UUID), action (VARCHAR), entity_type (VARCHAR), entity_id (UUID), old_values (JSONB nullable), new_values (JSONB nullable), ip_address (VARCHAR nullable), occurred_at (TIMESTAMPTZ)
- Package `internal/audit/` con Store interface y Logger
- `AuditLogger.Log(ctx, entry)` que escribe en DB
- Middleware que inyecta `AuditLogger` en el contexto (y extrae IP)
- Llamadas a `AuditLogger.Log()` en handlers después de operaciones exitosas
- Consultas: `Query(filter)` por actor_id, entity_type, action, rango de fechas
- Política de retención: default 90 días, configurable via env `DOMAIN_AUDIT_RETENTION_DAYS`
- Comando `domain audit prune` para purgar registros viejos
- Inmutabilidad: la app nunca hace UPDATE/DELETE sobre audit_log; la DB tiene trigger que bloquea modificaciones

**No incluye:**
- Logs de lectura (GET requests) — solo operaciones que cambian estado
- Integración con sistemas externos de SIEM
- Firma criptográfica de logs

## Enfoque técnico

1. Tabla con PK BIGSERIAL (auto-increment, immutabile por diseño)
2. Sin foreign keys para máximo performance de escritura
3. Índices en actor_id, entity_type+action, occurred_at para queries rápidas
4. Trigger BEFORE DELETE + BEFORE UPDATE en DB que rechaza modificaciones
5. La app escribe solo con INSERT, nunca UPDATE/DELETE
6. `AuditLogger` es un struct que recibe DB pool y retentionDays
7. Queries paginadas con cursor (por id) para manejar grandes volúmenes
8. Política de retención: DELETE en batch (1000 por lote) para no bloquear

## Riesgos

- **Crecimiento de la tabla:** Puede crecer rápidamente en producción. Mitigación: retention policy por defecto 90d, particionado mensual (post-MVP).
- **Performance de escritura:** Muchos INSERTs concurrentes. Mitigación: BIGSERIAL no lockea, batch async (post-MVP).
- **Trigger de inmutabilidad olvidado:** Migración down lo eliminaría. Mitigación: test de integración que verifica que el trigger existe.
- **IP detrás de proxy:** X-Forwarded-For vs RemoteAddr. Mitigación: extraer de headers confiables.

## Testing

- Test INSERT y SELECT de audit log
- Test que UPDATE/DELETE directo en DB es rechazado por trigger
- Test Query con todos los filtros
- Test paginación por cursor
- Test prune respeta retention days
- Test que handlers registran audit log después de operaciones exitosas
- Test que handlers NO registran audit log si la operación falla
