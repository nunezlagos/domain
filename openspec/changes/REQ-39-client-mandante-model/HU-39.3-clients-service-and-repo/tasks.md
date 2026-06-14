# Tasks: HU-39.3-clients-service-and-repo

## Tipos y entidad

- [ ] **types-001**: Definir struct `Client` en `repository.go` con campos
      mapeados a columnas de la tabla.
- [ ] **types-002**: Definir `CreateInput`, `UpdateInput`, `ListFilter`,
      `ListResult`.
- [ ] **types-003**: Definir variables `ErrSlugConflict`, `ErrNotFound`,
      `ErrInvalidInput`.

## Interface Repository

- [ ] **repo-001**: Declarar interface `Repository` con métodos `Insert`,
      `GetByID`, `GetBySlug`, `List`, `Update`, `Archive`, `Restore`.
- [ ] **repo-002**: Cada método recibe `ctx context.Context, tx pgx.Tx, ...`
      (no `*pgxpool.Pool`).

## Implementación pgx (pg_repository.go)

- [ ] **pg-001**: Struct `pgRepository` con dependencias mínimas (logger).
- [ ] **pg-002**: `Insert` ejecuta `INSERT INTO clients (...) VALUES (...)
      RETURNING id, created_at, updated_at`.
- [ ] **pg-003**: `GetByID` ejecuta `SELECT ... FROM clients WHERE id=$1
      AND deleted_at IS NULL`.
- [ ] **pg-004**: `GetBySlug` ejecuta `SELECT ... FROM clients WHERE
      slug=$1 AND deleted_at IS NULL` (RLS filtra org automáticamente).
- [ ] **pg-005**: `List` ejecuta `SELECT ... FROM clients WHERE
      [deleted_at filter] ORDER BY created_at DESC, id DESC LIMIT $n
      [+ cursor predicate]`.
- [ ] **pg-006**: `Update` con SET parcial (solo columnas presentes en
      UpdateInput) + WHERE id=$1.
- [ ] **pg-007**: `Archive` ejecuta `UPDATE SET status='archived',
      deleted_at=NOW() WHERE id=$1 AND deleted_at IS NULL`.
- [ ] **pg-008**: `Restore` ejecuta `UPDATE SET status='active',
      deleted_at=NULL WHERE id=$1 AND deleted_at IS NOT NULL`.
- [ ] **pg-009**: Helper `mapPgError(err)` que convierte `23505`/`23514`/
      `pgx.ErrNoRows` a errores domain.

## Service

- [ ] **svc-001**: Struct `Service` con dependencias: pool, repo, logger.
- [ ] **svc-002**: Constructor `New(pool *pgxpool.Pool, repo Repository,
      log *slog.Logger) *Service`.
- [ ] **svc-003**: Método `Create(ctx, in)`: validate → WithOrgTx →
      repo.Insert → mapPgError → return Client.
- [ ] **svc-004**: Método `GetByID(ctx, id)`: WithOrgTx → repo.GetByID →
      mapPgError → return.
- [ ] **svc-005**: Método `GetBySlug(ctx, slug)`: idem GetByID con slug.
- [ ] **svc-006**: Método `List(ctx, filter)`: clamp Limit → WithOrgTx →
      repo.List → cursor encoding.
- [ ] **svc-007**: Método `Update(ctx, id, in)`: validate → WithOrgTx →
      repo.Update → return Client actualizado.
- [ ] **svc-008**: Método `Archive(ctx, id)`: WithOrgTx → repo.Archive.
- [ ] **svc-009**: Método `Restore(ctx, id)`: WithOrgTx → repo.Restore →
      mapPgError (23505 puede aparecer).
- [ ] **svc-010**: Helper `validateCreate(in)` y `validateUpdate(in)`.

## Tests unitarios

- [ ] **test-001**: Fake repo en `service_test.go`.
- [ ] **test-002**: Create con name vacío → ErrInvalidInput.
- [ ] **test-003**: Create con slug "Foo Bar!" → ErrInvalidInput.
- [ ] **test-004**: Create con email "no-arroba" → ErrInvalidInput.
- [ ] **test-005**: Create OK → fake repo recibió row con orgID del ctx.
- [ ] **test-006**: List clamp Limit a 200.
- [ ] **test-007**: mapPgError 23505 → ErrSlugConflict.
- [ ] **test-008**: mapPgError ErrNoRows → ErrNotFound.

## Tests de integración (Postgres real)

- [ ] **int-001**: Crear org_a + org_b vía fixture.
- [ ] **int-002**: Create en org_a → fila visible con SET LOCAL=org_a.
- [ ] **int-003**: List desde org_b NO ve clients de org_a.
- [ ] **int-004**: Create con slug duplicado per-org → ErrSlugConflict.
- [ ] **int-005**: Create con mismo slug en orgs distintas → ambos OK.
- [ ] **int-006**: GetByID de cliente de org_b desde ctx org_a →
      ErrNotFound.
- [ ] **int-007**: Archive → fila tiene deleted_at + status='archived';
      List default no la incluye, List con IncludeArchived sí.
- [ ] **int-008**: Restore tras archive → vuelve a status='active',
      deleted_at=NULL.
- [ ] **int-009**: Restore con slug colisionado → ErrSlugConflict.

## Wiring

- [ ] **wire-001**: NO se hace wiring del Service en el server bootstrap
      en esta HU. Eso ocurre en HU-39.4 (donde el handler lo necesita).
- [ ] **wire-002**: Documentar en service.go el helper `New(...)` y los
      requisitos del contexto (ctx debe tener orgID).

## Notas para reviewers

- Cambios SOLO en `internal/service/client/*.go`. Sin tocar `project/`,
  `api/handler/`, `mcp/`.
- Coverage objetivo ≥80% (consistente con `project/`).
- Si hay duda sobre el regex de slug, mirar el ya usado en
  `internal/service/project/` para consistencia.
