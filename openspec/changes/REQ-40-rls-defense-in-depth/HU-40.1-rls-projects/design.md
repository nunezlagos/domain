# Design: HU-40.1-rls-projects

## Decisión arquitectónica

- **RLS + FORCE**: ambas activas. FORCE es no negociable para que el
  patrón sea efectivo contra el role owner.
- **Policy `FOR ALL`**: aplica a SELECT/INSERT/UPDATE/DELETE. Una sola
  policy con USING + WITH CHECK cubre los 4 verbos.
- **`current_org_id()` reutilizada**: misma función que ya usan
  observations, sessions, etc.
- **Grants idempotentes**: re-emitir SELECT/INSERT/UPDATE/DELETE para
  app_user y ALL para app_admin.

## Alternativas descartadas

- **RLS sin FORCE**: rompe el modelo cuando el owner accede. Sin sentido.
  Rechazado.
- **Policies separadas por verbo** (`FOR SELECT`, `FOR INSERT`, ...): más
  granular pero el caso de uso no lo amerita. Rechazado.
- **Función nueva tipo `current_org_id_strict()`** que falle si NULL:
  rompe el contrato de migration 000028 (devuelve NULL → 0 rows). Sin
  beneficio claro. Rechazado.
- **No aplicar FORCE**: dejar libre al owner. Inseguro. Rechazado.

## Migration up

```sql
-- migration: rls_projects
-- author: mnunez@saargo.com
-- issue: REQ-40
-- description: RLS + FORCE en projects (defense-in-depth multi-tenant)
-- breaking: false
-- estimated_duration: <1s

ALTER TABLE projects ENABLE ROW LEVEL SECURITY;
ALTER TABLE projects FORCE ROW LEVEL SECURITY;

CREATE POLICY projects_org_isolation ON projects
  FOR ALL TO PUBLIC
  USING (organization_id = current_org_id())
  WITH CHECK (organization_id = current_org_id());

GRANT SELECT, INSERT, UPDATE, DELETE ON projects TO app_user;
GRANT ALL ON projects TO app_admin;
```

## Migration down

```sql
DROP POLICY IF EXISTS projects_org_isolation ON projects;
ALTER TABLE projects NO FORCE ROW LEVEL SECURITY;
ALTER TABLE projects DISABLE ROW LEVEL SECURITY;
```

(GRANTs no se revocan en down.)

## Topología

```
session (sin SET LOCAL)               postgres
─────────────────                     ─────────
SELECT FROM projects             ──▶  policy: organization_id = NULL
                                       → 0 rows

session (con SET LOCAL org_a)
SELECT FROM projects             ──▶  policy: organization_id = $org_a
                                       → solo proyectos de org_a

session app_admin (BYPASSRLS)
SELECT FROM projects             ──▶  bypass policy
                                       → todos los proyectos
```

## Impacto en code path

| Path actual | Estado tras 000101 |
|-------------|--------------------|
| service.Project usa WithOrgTx | OK (sin cambios) |
| handler.Project usa WithOrgTx vía service | OK |
| mcp project tools usan WithOrgTx vía service | OK |
| Migrations / seeds corren como app_admin | OK (BYPASSRLS) |
| Algún path olvida WithOrgTx | 0 rows — **detectado por tests** |

## Verificación post-deploy

```sql
SELECT
  schemaname, tablename, rowsecurity, forcerowsecurity
FROM pg_tables
WHERE tablename = 'projects';
-- Expected: rowsecurity=t, forcerowsecurity=t

SELECT polname, polcmd, pg_get_expr(polqual, polrelid)
FROM pg_policy WHERE polrelid = 'projects'::regclass;
-- Expected: projects_org_isolation, FOR ALL, organization_id = current_org_id()
```
