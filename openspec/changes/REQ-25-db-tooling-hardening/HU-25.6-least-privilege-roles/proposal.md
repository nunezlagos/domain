# Proposal: HU-25.6-least-privilege-roles

## Intención

Definir y enforzar roles Postgres con least-privilege: `app_user` (runtime), `app_migrator` (DDL CI), `app_readonly`, `app_admin`, `pgbouncer_*`. Default deny + grants explícitos por tabla.

## Scope

**Incluye:**
- Migration que crea roles + revoke public defaults
- Grants por tabla en cada migration nueva (template + lint check)
- Helper `db-grants` Makefile target
- DEFAULT PRIVILEGES para grants automáticos en nuevas tables
- Secrets K8s separados por role
- Doc rol-tabla matrix

**No incluye:**
- Row-level grants (RLS lo cubre HU-25.5)
- Column-level masking (futuro)

## Enfoque técnico

1. Migration 000XXX crea roles
2. `ALTER DEFAULT PRIVILEGES FOR ROLE app_migrator IN SCHEMA public GRANT SELECT,INSERT,UPDATE,DELETE ON TABLES TO app_user` (auto-grant for future tables)
3. REVOKE CREATE/USAGE ON SCHEMA public FROM PUBLIC
4. K8s Secrets separados per role; helm values referencian existingSecret

## Riesgos

- Migration nueva olvida GRANT: DEFAULT PRIVILEGES mitiga
- TRUNCATE necesario en testing: rol test separado opcional
- Breaking deploy si role mal definido: tests integration validate

## Testing

- app_user DDL → permission denied
- app_user UPDATE audit_log → denied
- app_migrator DDL OK
- app_readonly SELECT OK, INSERT denied
- TRUNCATE app_user denied
- Public schema lockdown
- Nueva tabla en migration → app_user puede CRUD (default privileges)
