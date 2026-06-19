# Tasks: issue-42.7-rename-enrollment

## Verificación previa (bloqueante)

- [ ] Confirmar contra el catálogo que la tabla `org_enrollment_tokens` tiene 4 índices: `org_enrollment_tokens_pkey`, `org_enrollment_tokens_prefix_idx`, `org_enrollment_tokens_singleton_active_uniq`, `org_enrollment_tokens_status_idx`.
- [ ] Confirmar las 3 constraints: `org_enrollment_tokens_pkey` (PK), `org_enrollment_tokens_created_by_user_id_fkey` (FK), `org_enrollment_tokens_role_check` (CHECK).
- [ ] Confirmar que la PK es UUID → NO hay sequence (`org_enrollment_tokens_id_seq` no existe).
- [ ] Confirmar `relrowsecurity = f` → NO hay RLS policy que renombrar.
- [ ] Confirmar 0 FKs entrantes (ninguna tabla referencia a esta).
- [ ] Confirmar que 000153 es el siguiente número libre coordinado con las otras HUs del REQ-42.

## Migración 000153 (DDL)

- [ ] Crear `000153_rename_org_enrollment_tokens.up.sql` con header de metadatos completo.
- [ ] up: `ALTER TABLE org_enrollment_tokens RENAME TO enrollment_tokens`.
- [ ] up: 4× `ALTER INDEX ... RENAME` (pkey, prefix_idx, singleton_active_uniq, status_idx).
- [ ] up: 2× `ALTER TABLE enrollment_tokens RENAME CONSTRAINT` (FK created_by_user_id_fkey + CHECK role_check). NO incluir el pkey acá.
- [ ] Crear `000153_rename_org_enrollment_tokens.down.sql` con el reverso exacto.
- [ ] Verificar que ambos archivos están envueltos en `BEGIN; ... COMMIT;`.
- [ ] Pasar `squawk` sobre up y down sin findings.

## Código Go (mismo commit)

- [ ] `internal/auth/bootstrap/service.go`: renombrar `org_enrollment_tokens` → `enrollment_tokens` en el `INSERT INTO` (~línea 167).
- [ ] `internal/service/enrollment/service.go`: renombrar las 5 ocurrencias (UPDATE/INSERT/SELECT, ~líneas 100/115/160/177/224).
- [ ] `internal/service/enrollment/service_integration_test.go`: renombrar fixtures/asserts (`SELECT COUNT(*) FROM org_enrollment_tokens` y variantes, ~líneas 64/66) y el comentario de cabecera.
- [ ] Verificar que ninguna firma Go ni tipo cambió (solo strings SQL).

## Tests

- [ ] Test de integración: aplicar 000153 y ejecutar bootstrap del primer usuario → inserta en `enrollment_tokens` sin error.
- [ ] Test de integración: `Rotate` revoca el token activo y crea uno nuevo respetando `enrollment_tokens_singleton_active_uniq`.
- [ ] Test de integración: `Revoke` / `Enroll` / `GetMetadata` operan contra `enrollment_tokens`.
- [ ] Test de migración down: tras `down`, la tabla `org_enrollment_tokens` existe de nuevo con sus índices/constraints originales.
- [ ] Assert de catálogo: `enrollment_tokens` existe y `org_enrollment_tokens` NO (post-up).

## Sabotaje (anti-falsos positivos)

Objetivo: probar que la suite REALMENTE falla si una query queda apuntando al nombre viejo (un test que pasa con el bug presente es un falso positivo inútil).

- [ ] **Romper a propósito:** en `internal/service/enrollment/service.go`, revertir UNA sola query al nombre viejo, p.ej.:
  ```go
  // SABOTAJE temporal:
  query := `SELECT ... FROM org_enrollment_tokens WHERE revoked_at IS NULL ...`
  ```
- [ ] **Esperado:** con la migración 000153 aplicada (tabla ya renombrada a `enrollment_tokens`), el test de integración de `GetMetadata`/`Rotate` debe FALLAR con `ERROR: relation "org_enrollment_tokens" does not exist`.
- [ ] **Confirmar el rojo:** correr `go test ./internal/service/enrollment/...` y verificar que falla por esa relación inexistente (no por otra causa).
- [ ] **Restaurar el fix:** volver a `enrollment_tokens`.
- [ ] **Confirmar el verde:** re-correr el test; pasa. Esto prueba que el test discrimina el bug, no que pasa por inercia.
- [ ] **Sabotaje del DDL:** en un entorno descartable, comentar el `ALTER INDEX org_enrollment_tokens_pkey RENAME` y agregar en su lugar un `ALTER TABLE ... RENAME CONSTRAINT org_enrollment_tokens_pkey` → confirmar que la migración ABORTA (demuestra por qué el pkey va solo por ALTER INDEX). Restaurar.

## Cierre

- [ ] `grep -rn "org_enrollment_tokens" --include="*.go" services/domain-backend` → 0 resultados (las migraciones históricas .sql 000098/000139/000141/000142 quedan intactas y NO cuentan).
- [ ] `go vet ./...` sin warnings.
- [ ] `go build ./...` OK.
- [ ] `go test ./...` verde (incluye los tests de integración de enrollment).
- [ ] Actualizar la doc afectada si referencia el nombre viejo (`docs/single-org.md`, `docs/db/rls.md`) — solo si describen el schema vigente, no históricos.
- [ ] Commit en rama `services` (Conventional Commits, español, SIN Co-Authored-By):
  ```
  refactor: rename org_enrollment_tokens → enrollment_tokens (REQ-42.7)
  ```
- [ ] NO hacer git push (repo local-only).
