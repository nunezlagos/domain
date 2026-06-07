# Proposal: HU-25.10-db-secrets-rotation

## Intención

Rotación de passwords DB sin downtime usando dual-credentials window. Cron 90 días, manual override command, integración ESO/AWS Secrets Manager, multi-role staggered.

## Scope

**Incluye:**
- Cron k8s 90 días por role (staggered)
- `domain-mcp rotate-db-password --role X` manual
- ESO/AWS Secrets Manager support (también plain K8s Secret)
- PgBouncer userlist re-sync
- Rollback en failure
- Audit log

**No incluye:**
- Rotation de root postgres password (responsabilidad cloud-managed o operador)
- Rotation de cert TLS (otra HU si necesario)

## Enfoque técnico

1. Two-phase: ALTER ROLE PASSWORD nuevo + actualizar Secret + rollout + después verify no pods viejos
2. PgBouncer userlist regenerada desde Secret update
3. Staggered schedule: app_user día 1, app_admin día 8, etc.
4. Rollback: si verify falla, restaurar password_current

## Riesgos

- Pods mid-rollout fail: helm rollback + ALTER ROLE back
- PgBouncer cache stale: trigger RELOAD config
- Auto-secrets manager downtime: cron retry hasta éxito

## Testing

- Manual rotation funciona zero-downtime
- Cron scheduled rota cada role
- Rollback en failure
- ESO sync verified
- PgBouncer reload after secret update
