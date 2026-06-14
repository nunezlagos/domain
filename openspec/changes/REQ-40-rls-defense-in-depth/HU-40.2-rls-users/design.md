# Design: HU-40.2-rls-users

## Decisión arquitectónica

- RLS + FORCE en `users` con policy por `organization_id`.
- Login pre-org (resolver email→user→org) usa `app_admin` (BYPASSRLS).
  Documentado.
- Re-grants explícitos siguiendo el patrón.

## Alternativas descartadas

- **No aplicar RLS porque "login es complejo"**: solo deja el gap
  abierto. Login se resuelve con `app_admin` deliberadamente.
- **Policy por `id = current_user_id()`** (cada user solo se ve a sí
  mismo): demasiado restrictivo, rompe listado de team members.
- **Policy híbrida org + user**: complejo y sin valor real para escala
  20 users. Rechazado.

## Migration up

```sql
-- migration: rls_users
-- author: mnunez@saargo.com
-- issue: REQ-40
-- description: RLS + FORCE en users (defense-in-depth multi-tenant)
-- breaking: false
-- estimated_duration: <1s

ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE users FORCE ROW LEVEL SECURITY;

CREATE POLICY users_org_isolation ON users
  FOR ALL TO PUBLIC
  USING (organization_id = current_org_id())
  WITH CHECK (organization_id = current_org_id());

GRANT SELECT, INSERT, UPDATE, DELETE ON users TO app_user;
GRANT ALL ON users TO app_admin;
```

## Migration down

```sql
DROP POLICY IF EXISTS users_org_isolation ON users;
ALTER TABLE users NO FORCE ROW LEVEL SECURITY;
ALTER TABLE users DISABLE ROW LEVEL SECURITY;
```

## Implicación para login flow

```
Caso 1: login conoce la org (token signed con org_id)
  - middleware setea ctx.OrgID
  - service.User.GetByEmail(ctx, email) → WithOrgTx → SELECT ...
  - RLS filtra por org_a → user encontrado o not found

Caso 2: enrollment / pre-login (email único globalmente NO garantizado)
  - middleware NO tiene orgID
  - resolución por token de enrollment (REQ-37) sabe la org
  - usar app_admin solo en pasos previos al hand-off
  - una vez obtenida orgID, switch a app_user + WithOrgTx
```

## Verificación post-deploy

```sql
SELECT rowsecurity, forcerowsecurity FROM pg_tables
WHERE tablename='users';
-- Expected: true, true

SELECT polname FROM pg_policy WHERE polrelid='users'::regclass;
-- Expected: users_org_isolation
```
