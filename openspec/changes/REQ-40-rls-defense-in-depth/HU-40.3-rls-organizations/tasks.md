# Tasks: HU-40.3-rls-organizations

## Migration up

- [ ] **mig-001**: Crear `internal/migrate/migrations/000103_rls_organizations.up.sql`
      con header estándar.
- [ ] **mig-002**: `ALTER TABLE organizations ENABLE ROW LEVEL SECURITY`.
- [ ] **mig-003**: `ALTER TABLE organizations FORCE ROW LEVEL SECURITY`.
- [ ] **mig-004**: `CREATE POLICY organizations_self_only ON organizations
      FOR ALL TO PUBLIC USING (id = current_org_id()) WITH CHECK (id =
      current_org_id())`.
- [ ] **mig-005**: `GRANT SELECT, INSERT, UPDATE, DELETE ON organizations
      TO app_user`.
- [ ] **mig-006**: `GRANT ALL ON organizations TO app_admin`.

## Migration down

- [ ] **down-001**: Crear `internal/migrate/migrations/000103_rls_organizations.down.sql`
      con DROP POLICY + NO FORCE + DISABLE.

## Tests integración

- [ ] **t-001**: Sin SET LOCAL → SELECT count(*) FROM organizations = 0.
- [ ] **t-002**: SET LOCAL org_a → SELECT * retorna exactamente 1 fila
      (org_a).
- [ ] **t-003**: SET LOCAL org_a, INSERT id=$org_b → falla por WITH CHECK.
- [ ] **t-004**: app_admin → SELECT count(*) = total; puede INSERT
      cualquier nueva org.
- [ ] **t-005**: Endpoint "/me org" o equivalente sigue retornando la
      org correcta.
- [ ] **t-006**: Bootstrap / enrollment (REQ-37) crea nueva org usando
      app_admin OK.
- [ ] **t-007**: Migrate down + up round-trip sin errores.

## Verificación de impacto

- [ ] **impact-001**: `grep -rn "FROM organizations" internal/` revisa
      paths legítimos (vía service / WithOrgTx) vs paths sospechosos.
- [ ] **impact-002**: Verificar que `internal/installer/` usa app_admin
      al crear orgs nuevas.
- [ ] **impact-003**: Verificar que tests fixtures usan app_admin.
- [ ] **impact-004**: `make lint-sql` sin warnings críticos.

## Notas para reviewers

- Cambios SOLO en los dos archivos de migración.
- Caso especial: la policy referencia `id`, no `organization_id`.
  Confirmar visualmente.
- Si rompe el bootstrap del primer usuario, el fix es asegurar que
  use `app_admin` para crear la org (probablemente ya lo hace; verificar).
- Esta es la última de las 3 migraciones de RLS. Tras mergear, todas
  las tablas multi-tenant tienen defense-in-depth.
