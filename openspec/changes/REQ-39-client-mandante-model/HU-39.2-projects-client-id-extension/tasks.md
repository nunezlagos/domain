# Tasks: HU-39.2-projects-client-id-extension

## Migration up

- [ ] **mig-001**: Crear
      `internal/migrate/migrations/000100_projects_add_client_id.up.sql`
      con header estándar (migration / author / issue / description /
      breaking=false / estimated_duration <1s).
- [ ] **mig-002**: `ALTER TABLE projects ADD COLUMN client_id UUID
      REFERENCES clients(id) ON DELETE SET NULL`.
- [ ] **mig-003**: `CREATE INDEX projects_client_id_idx ON projects
      (client_id) WHERE deleted_at IS NULL AND client_id IS NOT NULL`.
- [ ] **mig-004**: `CREATE OR REPLACE FUNCTION
      projects_check_client_same_org()` con cuerpo plpgsql que maneja:
      - `NEW.client_id IS NULL` → return NEW
      - client no existe → RAISE foreign_key_violation
      - client_org ≠ project_org → RAISE check_violation
- [ ] **mig-005**: `CREATE TRIGGER projects_client_same_org_check BEFORE
      INSERT OR UPDATE OF client_id, organization_id ON projects FOR EACH
      ROW`.

## Migration down

- [ ] **down-001**: Crear
      `internal/migrate/migrations/000100_projects_add_client_id.down.sql`
      con DROP TRIGGER + DROP FUNCTION + DROP INDEX + ALTER TABLE DROP
      COLUMN (en este orden).

## Tests de integración

- [ ] **test-001**: `make migrate-up` aplica 000100 desde estado limpio
      con clients ya creada (000099).
- [ ] **test-002**: Insertar projects sin client_id → client_id NULL.
- [ ] **test-003**: Crear client en org_a, insertar project en org_a con
      ese client_id → OK.
- [ ] **test-004**: Crear client en org_b, intentar insertar project en
      org_a con ese client_id → check_violation (23514).
- [ ] **test-005**: Insertar project con client_id UUID inexistente →
      foreign_key_violation (23503).
- [ ] **test-006**: UPDATE projects SET client_id=$cross_org → falla.
- [ ] **test-007**: UPDATE projects SET name='nuevo' (sin tocar client_id
      ni organization_id) → OK, sin disparar validación SELECT clients.
- [ ] **test-008**: DELETE FROM clients WHERE id=$c → projects asociados
      sobreviven con client_id=NULL.
- [ ] **test-009**: Migrate down → columna desaparece, función desaparece,
      trigger desaparece, projects sigue intacto.
- [ ] **test-010**: Round-trip up/down/up sin errores.

## Verificación de impacto

- [ ] **impact-001**: Tests de integración previos del modelo projects
      siguen pasando (no se rompen por la columna nueva).
- [ ] **impact-002**: `make lint-sql` (squawk) sin warnings críticos.
      Si hay warning sobre "trigger en table grande", documentar como
      acceptado (escala 20 users, irrelevante).
- [ ] **impact-003**: schema-drift dump (si existe en CI) refleja la
      columna nueva, índice nuevo, función + trigger nuevos.

## Notas para reviewers

- Cambios SOLO en los dos archivos de migración. Sin tocar Go, sin tocar
  scripts.
- Verificar que la migración 000099 (clients) está aplicada **antes** que
  esta; el FK lo requiere. Si se corre fuera de orden, falla con relation
  "clients" does not exist.
- El trigger se dispara también si se hace `UPDATE projects SET
  organization_id=$x` (transfer cross-tenant); intencional, evita "fugar"
  un project a otra org sin desasociar primero el cliente.
- No es necesario VACUUM ni REINDEX: la tabla es chica.
