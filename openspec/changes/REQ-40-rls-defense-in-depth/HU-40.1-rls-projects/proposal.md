# Proposal: HU-40.1-rls-projects

## Intención

Activar RLS + FORCE sobre `projects` con policy
`projects_org_isolation` por `organization_id = current_org_id()`,
replicando exactamente el patrón ya validado en migraciones 000028
(secrets, audit_log, otp_codes, activity_log, api_keys) y 000085
(observations, sessions).

## Scope

**Incluye:**
- `internal/migrate/migrations/000101_rls_projects.up.sql`
- `internal/migrate/migrations/000101_rls_projects.down.sql`
- `ALTER TABLE projects ENABLE ROW LEVEL SECURITY`.
- `ALTER TABLE projects FORCE ROW LEVEL SECURITY`.
- `CREATE POLICY projects_org_isolation ON projects FOR ALL TO PUBLIC
   USING (organization_id = current_org_id()) WITH CHECK (organization_id
   = current_org_id())`.
- `GRANT SELECT, INSERT, UPDATE, DELETE ON projects TO app_user` +
  `GRANT ALL ON projects TO app_admin`.

**No incluye:**
- Cambios al modelo Go o a queries (no hace falta — `WithOrgTx` ya
  setea el session var).
- Test de regresión (los tests existentes ya cubren el code path con
  `WithOrgTx`; se agregan tests específicos para RLS en
  `internal/migrate/migrations/projects_rls_integration_test.go` o
  equivalente).
- Cambios a tablas dependientes (issues, agents, sessions ya tienen RLS
  propia).
- RLS para `users` (HU-40.2) o `organizations` (HU-40.3).

## Enfoque técnico

1. **Reutilizar función helper**: `current_org_id()` ya existe.
2. **FORCE obligatorio**: sin FORCE, el role owner de la tabla bypassea
   RLS y el modelo se rompe. Patrón ya documentado en 000028.
3. **Grants explícitos**: por seguridad, re-emitir GRANTs aunque la
   tabla ya los tenga implícitos.
4. **Down migration limpia**: DROP POLICY → DISABLE FORCE → DISABLE RLS.
   No tocar GRANTs (no se revocan; el patrón ya documentado).

## Riesgos

- **Olvido de `WithOrgTx` en algún path**: si existe un handler/seed
  que ejecuta queries directas sobre `projects` sin `WithOrgTx`,
  empezará a ver 0 rows tras esta migración. **Eso es la intención**
  (forzar al code path correcto), pero puede causar regresiones
  visibles en endpoints que no se testearon.
  - Mitigación: correr suite de integration tests completa antes de
    mergear. Cualquier test que rompa identifica el path defectuoso.
  - Mitigación: revisar manualmente `grep -rn "FROM projects" internal/`
    para detectar queries sospechosas (sin `WithOrgTx`).
- **Performance**: RLS evalúa policy por fila. En escala "20 users" es
  irrelevante. Para queries que escaneen >100k filas, overhead 1-3%.
  Mitigación: aceptado.
- **Bypass por role owner**: si la tabla fue creada por un role distinto
  de `app_migrator`, FORCE asegura que igual aplique. La migración 000025
  ya equipara grants.

## Testing

- Test integración nuevo (puede vivir en el mismo archivo que
  `roles_integration_test.go`):
  - Conectar como app_user, sin SET LOCAL → SELECT count(*) = 0.
  - Conectar como app_user, SET LOCAL org_a → count = N(org_a).
  - Conectar como app_user, SET LOCAL org_a, INSERT cross-tenant → fail.
  - Conectar como app_admin → count = total.
- Test regresión: suite completa de `service.Project`
  (`service_integration_test.go`) sigue pasando.
- Test migrate down → RLS off.
- Test round-trip up/down/up.
