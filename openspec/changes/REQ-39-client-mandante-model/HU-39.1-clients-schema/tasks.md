# Tasks: HU-39.1-clients-schema

## Migration up

- [ ] **mig-001**: Crear `internal/migrate/migrations/000099_create_clients.up.sql`
      con header (migration / author / issue / description / breaking /
      estimated_duration) siguiendo el patrón de 000028 y 000085.
- [ ] **mig-002**: `CREATE TABLE clients` con todas las columnas, defaults,
      checks y UNIQUE listados en design.md.
- [ ] **mig-003**: `CREATE TRIGGER set_updated_at_clients` reutilizando la
      función `set_updated_at()`.
- [ ] **mig-004**: `CREATE INDEX clients_organization_id_idx` parcial sobre
      `WHERE deleted_at IS NULL`.
- [ ] **mig-005**: `ALTER TABLE clients ENABLE ROW LEVEL SECURITY` +
      `FORCE ROW LEVEL SECURITY`.
- [ ] **mig-006**: `CREATE POLICY clients_org_isolation` usando
      `current_org_id()` en USING y WITH CHECK.
- [ ] **mig-007**: `GRANT SELECT, INSERT, UPDATE, DELETE ON clients TO
      app_user` + `GRANT ALL ON clients TO app_admin`.

## Migration down

- [ ] **down-001**: Crear `internal/migrate/migrations/000099_create_clients.down.sql`
      con `DROP TABLE IF EXISTS clients CASCADE`.
- [ ] **down-002**: Verificar que `current_org_id()`, `app_user`, `app_admin`
      y `organizations` siguen intactos tras down.

## Tests de integración

- [ ] **test-001**: `make migrate-up` desde clean state aplica 000099 sin
      error.
- [ ] **test-002**: Insertar 2 clients con mismo slug en orgs distintas → OK.
- [ ] **test-003**: Insertar 2 clients con mismo slug en misma org → 23505.
- [ ] **test-004**: Insertar client con status='foo' → 23514.
- [ ] **test-005**: Con `SET LOCAL app.current_org_id = $org_a`, SELECT
      devuelve solo clients de org_a.
- [ ] **test-006**: Sin `SET LOCAL`, SELECT devuelve 0 rows.
- [ ] **test-007**: DELETE org → clients en cascada borrados.
- [ ] **test-008**: UPDATE de fila incrementa `updated_at` (trigger).
- [ ] **test-009**: `make migrate-down` (000099) → tabla desaparece.
- [ ] **test-010**: `make migrate-down` + `make migrate-up` round-trip OK.

## Verificación de impacto

- [ ] **impact-001**: `make lint-sql` (squawk) pasa sin warnings críticos.
- [ ] **impact-002**: Comparar esquema actual con baseline schema dump (si
      existe `schema-drift`) y confirmar que el delta es solo `clients`
      + sus dependencias.
- [ ] **impact-003**: Tests de integración previos (organizations, users,
      projects) siguen pasando sin modificación.

## Notas para reviewers

- Cambios SOLO en los dos archivos de migración. Sin tocar Go, sin tocar
  scripts, sin tocar fixtures.
- La función `current_org_id()` ya existe desde 000028; NO se redefine.
- RLS se activa en esta misma migración (no esperar a REQ-40) para evitar
  ventana de exposición entre creación y hardening.
- Si `make lint-sql` da warning de "missing index on FK", confirmar que
  el índice parcial `clients_organization_id_idx` cubre el caso (las
  queries de la app siempre filtran `deleted_at IS NULL`).
