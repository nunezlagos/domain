# Tasks: HU-40.1-rls-projects

## Migration up

- [ ] **mig-001**: Crear `internal/migrate/migrations/000101_rls_projects.up.sql`
      con header estándar.
- [ ] **mig-002**: `ALTER TABLE projects ENABLE ROW LEVEL SECURITY`.
- [ ] **mig-003**: `ALTER TABLE projects FORCE ROW LEVEL SECURITY`.
- [ ] **mig-004**: `CREATE POLICY projects_org_isolation ON projects FOR
      ALL TO PUBLIC USING (organization_id = current_org_id()) WITH
      CHECK (organization_id = current_org_id())`.
- [ ] **mig-005**: `GRANT SELECT, INSERT, UPDATE, DELETE ON projects TO
      app_user`.
- [ ] **mig-006**: `GRANT ALL ON projects TO app_admin`.

## Migration down

- [ ] **down-001**: Crear `internal/migrate/migrations/000101_rls_projects.down.sql`
      con DROP POLICY + NO FORCE + DISABLE.

## Tests integración

- [ ] **t-001**: Sesión sin SET LOCAL → SELECT count(*) FROM projects = 0.
- [ ] **t-002**: SET LOCAL org_a → count = N(org_a).
- [ ] **t-003**: SET LOCAL org_a, INSERT con organization_id=org_b → falla
      por WITH CHECK violation.
- [ ] **t-004**: Conexión app_admin (BYPASSRLS) → count = total.
- [ ] **t-005**: Suite `service.Project` integration tests sigue pasando
      sin modificación.
- [ ] **t-006**: Migrate down → relrowsecurity=false, policy desaparece.
- [ ] **t-007**: Round-trip up/down/up sin errores.

## Verificación de impacto

- [ ] **impact-001**: `grep -rn "FROM projects" internal/` no revela
      paths que ejecuten queries sobre projects fuera de service.Project
      / repos legítimos.
- [ ] **impact-002**: `make lint-sql` (squawk) sin warnings críticos.
- [ ] **impact-003**: Benchmark micro: ejecutar 1000 queries
      `SELECT id FROM projects WHERE slug=$1` con WithOrgTx, comparar
      latencia antes/después de RLS — overhead <5%.

## Notas para reviewers

- Cambios SOLO en los dos archivos de migración. Sin tocar Go.
- Confirmar mentalmente que `service.Project` usa `WithOrgTx` en TODOS
  los métodos (el repo recibe `pgx.Tx`, no `pgxpool.Pool`).
- Si algún test rompe, NO es bug de RLS — es bug de path olvidando
  WithOrgTx. Arreglar el code path antes de mergear.
- Esta migración corre **después** de REQ-39 (que ya creó clients +
  projects.client_id). No hay dependencia inversa.
