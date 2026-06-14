# Tasks: HU-40.2-rls-users

## Migration up

- [ ] **mig-001**: Crear `internal/migrate/migrations/000102_rls_users.up.sql`
      con header estándar.
- [ ] **mig-002**: `ALTER TABLE users ENABLE ROW LEVEL SECURITY`.
- [ ] **mig-003**: `ALTER TABLE users FORCE ROW LEVEL SECURITY`.
- [ ] **mig-004**: `CREATE POLICY users_org_isolation ON users FOR ALL TO
      PUBLIC USING (organization_id = current_org_id()) WITH CHECK
      (organization_id = current_org_id())`.
- [ ] **mig-005**: `GRANT SELECT, INSERT, UPDATE, DELETE ON users TO
      app_user`.
- [ ] **mig-006**: `GRANT ALL ON users TO app_admin`.

## Migration down

- [ ] **down-001**: Crear `internal/migrate/migrations/000102_rls_users.down.sql`
      con DROP POLICY + NO FORCE + DISABLE.

## Tests integración

- [ ] **t-001**: Sesión sin SET LOCAL → SELECT count(*) FROM users = 0.
- [ ] **t-002**: SET LOCAL org_a → count = N(org_a).
- [ ] **t-003**: INSERT cross-tenant rechazado por WITH CHECK.
- [ ] **t-004**: app_admin sin SET LOCAL → count = total.
- [ ] **t-005**: Suite auth/login (REQ-02 / REQ-36) pasa sin cambios.
- [ ] **t-006**: Suite user management (CRUD) pasa.
- [ ] **t-007**: Migrate down + up round-trip sin errores.

## Verificación de impacto

- [ ] **impact-001**: `grep -rn "FROM users" internal/` revisa que toda
      query es vía service.User + WithOrgTx, o vía app_admin explícito.
- [ ] **impact-002**: Verificar que `internal/installer/` (enrollment) y
      `internal/auth/` (login) usan `app_admin` o `WithOrgTx` apropiado.
- [ ] **impact-003**: `make lint-sql` sin warnings críticos.

## Notas para reviewers

- Cambios SOLO en los dos archivos de migración.
- El bug más probable post-merge es un endpoint admin/super-admin que
  listaba users sin filtro y ahora rompe. Identificar antes con grep.
- Si algún test de auth rompe, el fix es: usar `app_admin` para el
  lookup pre-org y luego switchear a `app_user` + `WithOrgTx` con la
  org resuelta.
