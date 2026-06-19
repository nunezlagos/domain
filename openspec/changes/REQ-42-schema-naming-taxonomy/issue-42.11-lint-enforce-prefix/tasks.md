# Tasks: issue-42.11-lint-enforce-prefix

## Verificación previa (bloqueante)

- [x] Confirmar que `cmd/db-conventions-lint/main.go` soporta `-baseline` (flag + `n <= *baseline` → continue)
- [x] Confirmar que `checkCreateTableConventions` (lint.go:247) reusa `extractCreateTables`
- [x] Confirmar que el bloque `naming-plural-table` está antes de `require-created-at` (punto de inserción)
- [x] Confirmar que `parseOverrides` cubre cualquier `rule` por nombre (override funciona sin tocar nada)
- [x] Confirmar que `make db-lint` corre el cmd SIN baseline hoy (hay que agregar `-baseline 146`)
- [x] Excepciones canónicas RESUELTAS (REQ-42.8): `users`, `roles`, `user_roles`, `issues` (+ `schema_migrations`) en allowlist `canonicalTableExceptions`. NO es open_question.
- [x] `enrollment_` es prefijo válido de la allowlist (`enrollment_tokens`, single-org). Decisión tomada.

## Implementación — `internal/dbconvlint/lint.go`

- [ ] Agregar `require-table-prefix` al doc-comment de cabecera (líneas 4-14)
- [ ] Declarar `validTablePrefixes` (allowlist con underscore final en cada entry)
- [ ] Declarar `canonicalTableExceptions` (`users`, `roles`, `user_roles`, `issues`, `schema_migrations`) — allowlist canónica RESUELTA (REQ-42.8)
- [ ] Declarar helper `hasValidTablePrefix(name string) bool`
- [ ] Insertar el check `require-table-prefix` en `checkCreateTableConventions`, tras `naming-plural-table` y antes de `require-created-at`
- [ ] Verificar que `name`, `line`, `add` están en scope (no cambia firma)

## Tests — `internal/dbconvlint/lint_test.go`

- [ ] `TestLint_RequireTablePrefix_FailsUnprefixed`: `CREATE TABLE budgets (...)` → `require.Contains(issueRules(...), "require-table-prefix")`
- [ ] `TestLint_RequireTablePrefix_AllowsPrefixed`: `CREATE TABLE agent_runs (...)` y `CREATE TABLE flow_config (...)` → `require.NotContains`
- [ ] `TestLint_RequireTablePrefix_CanonicalException`: `CREATE TABLE users (...)`, `CREATE TABLE roles (...)`, `CREATE TABLE user_roles (...)` y `CREATE TABLE issues (...)` → `require.NotContains`
- [ ] `TestLint_RequireTablePrefix_Override`: `-- domain-lint-ignore-next: require-table-prefix` sobre el CREATE → `require.NotContains`
- [ ] Usar `validHeader` + tablas en plural en los fixtures (para no contaminar con `naming-plural-table`)
- [ ] Para nombres canónicos, sumar el `created_at` en el body para no contaminar con `require-created-at`

## Enganche — Makefile + CI

- [ ] Agregar `-baseline 146` al target `db-lint` del `Makefile`
- [ ] Agregar `-baseline 146` al job `db-conventions-lint` del workflow CI
- [ ] Confirmar que el pre-commit (`scripts/githooks/pre-commit`) hereda el cambio vía `make db-lint` (sin tocar el hook)

## Sabotaje (anti-falsos positivos)

OBLIGATORIO. El test debe medir la REGLA, no un efecto colateral.

- [ ] **Sabotaje 1 (la regla detecta):** comentar/eliminar el bloque `require-table-prefix` en `lint.go` y correr `go test ./internal/dbconvlint/`. El test `TestLint_RequireTablePrefix_FailsUnprefixed` DEBE ponerse ROJO. Si sigue verde, el test no está midiendo la regla → corregir el test.
- [ ] **Sabotaje 2 (la regla no es over-eager):** con el check activo, cambiar el fixture de `_AllowsPrefixed` de `agent_runs` a `agentruns` (sin underscore). El test DEBE detectar que ahora dispara `require-table-prefix` (la regla exige el separador). Restaurar a `agent_runs`.
- [ ] **Sabotaje 3 (baseline real):** crear un archivo temporal `000100_legacy_test.up.sql` con `CREATE TABLE budgets (...)`. Correr `db-conventions-lint -baseline 146` → NO debe reportar (número <= 146). Renombrarlo a `000147_...` → SÍ debe reportar. Borrar el temporal.
- [ ] Después de cada sabotaje: restaurar el fix → todos los tests verdes.

## Cierre

- [ ] `go vet ./...` sin warnings
- [ ] `go build ./...` OK
- [ ] `go test ./internal/dbconvlint/...` verde (incluye los 4 tests nuevos)
- [ ] `make db-lint` corre limpio con `-baseline 146` (no enrojece por históricas)
- [ ] Commit en rama `services` (español, Conventional Commits, SIN Co-Authored-By):
      `feat(dbconvlint): regla require-table-prefix en CREATE TABLE (issue 42.11)`
- [ ] NO git push (repo local-only)
