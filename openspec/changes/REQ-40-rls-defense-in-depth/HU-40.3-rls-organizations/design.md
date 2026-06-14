# Design: HU-40.3-rls-organizations

## Decisión arquitectónica

- RLS + FORCE en `organizations` con policy
  `organizations_self_only` referenciando `id` (no
  `organization_id`).
- Aprovisionamiento de nuevas orgs → `app_admin`.
- Grants explícitos.

## Alternativas descartadas

- **No aplicar RLS porque "es tabla raíz"**: deja el gap abierto. Mismo
  riesgo que projects/users.
- **Permitir lectura de TODAS las orgs desde app_user** (policy
  permisiva): rompe el principio multi-tenant.
- **Policy distinta para SELECT y para INSERT** (más permisiva en
  SELECT): innecesariamente complejo. Toda interacción al runtime es
  per-tenant.

## Migration up

```sql
-- migration: rls_organizations
-- author: mnunez@saargo.com
-- issue: REQ-40
-- description: RLS + FORCE en organizations (defense-in-depth multi-tenant)
-- breaking: false
-- estimated_duration: <1s
--
-- Caso especial: organizations es la tabla raíz; la policy referencia
-- `id` directamente porque NO hay columna `organization_id` (la PK ES
-- el tenant). app_admin (BYPASSRLS) sigue creando orgs nuevas durante
-- aprovisionamiento.

ALTER TABLE organizations ENABLE ROW LEVEL SECURITY;
ALTER TABLE organizations FORCE ROW LEVEL SECURITY;

CREATE POLICY organizations_self_only ON organizations
  FOR ALL TO PUBLIC
  USING (id = current_org_id())
  WITH CHECK (id = current_org_id());

GRANT SELECT, INSERT, UPDATE, DELETE ON organizations TO app_user;
GRANT ALL ON organizations TO app_admin;
```

## Migration down

```sql
DROP POLICY IF EXISTS organizations_self_only ON organizations;
ALTER TABLE organizations NO FORCE ROW LEVEL SECURITY;
ALTER TABLE organizations DISABLE ROW LEVEL SECURITY;
```

## Topología

```
session app_user, sin SET LOCAL          → 0 rows
session app_user, SET LOCAL org_a        → 1 row (org_a)
session app_user, SET LOCAL org_a,
  INSERT id=org_b                        → WITH CHECK violation

session app_admin                        → ve todo, puede crear
                                            cualquier org (aprovisionamiento)
```

## Implicación operativa

- El servicio `service.Organization` (si existe) debe usar
  `WithOrgTx(orgID=ctx.OrgID, ...)` para queries de runtime.
- El instalador / enrollment usa `app_admin` o un helper específico
  (`db.WithAdminConn` si existe) para crear nuevas orgs.
- Tests de fixtures que crean orgs corren como `app_admin`.

## Verificación post-deploy

```sql
SELECT rowsecurity, forcerowsecurity FROM pg_tables
WHERE tablename='organizations';
-- Expected: true, true

SELECT polname, pg_get_expr(polqual, polrelid)
FROM pg_policy WHERE polrelid='organizations'::regclass;
-- Expected: organizations_self_only, id = current_org_id()
```
