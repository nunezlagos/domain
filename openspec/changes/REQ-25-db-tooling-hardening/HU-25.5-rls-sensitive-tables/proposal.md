# Proposal: HU-25.5-rls-sensitive-tables

## Intención

Activar RLS en tablas de alto impacto (secrets, billing, audit_log, sessions, otp, idempotency, notifications, custom_roles, webhooks, memory_kv, export_jobs) con policies por `organization_id` o `user_id`. Bug en RBAC ya no es leak total.

## Scope

**Incluye:**
- Migrations ENABLE RLS + CREATE POLICY en 12 tablas
- Helper `db.WithOrgContext(ctx, orgID, userID)` que ejecuta SET LOCAL al inicio de cada tx
- Role `app_admin` con BYPASSRLS para batch jobs admin
- Tests adversariales: tx sin SET LOCAL → 0 rows
- Performance test: <5% regression

**No incluye:**
- RLS en TODAS las tablas (overkill; observations/skills etc usan app RBAC)
- Dynamic policies basadas en RBAC roles (futuro)

## Enfoque técnico

1. Policy USING con `current_setting('app.current_org', true)::uuid` (true permite NULL si no set → 0 rows)
2. Helper en `internal/store/` wrap tx para SET LOCAL
3. Tests integration con 2 orgs verifican aislamiento
4. Roles `app_user NOBYPASSRLS`, `app_admin BYPASSRLS`

## Riesgos

- Olvido SET LOCAL → bug "no devuelve datos": linter test enforces helper wrap
- Performance: usar mismo predicate que indices existentes
- PgBouncer transaction-pooling compatible (SET LOCAL muere al COMMIT, perfecto)

## Testing

- Policy bloquea cross-org
- SET LOCAL correcto → ve sólo su org
- Sin SET LOCAL → 0 rows
- app_admin BYPASSRLS funciona
- Performance <5% regression bench
- Sabotaje: app intenta query sin helper → linter fails CI
