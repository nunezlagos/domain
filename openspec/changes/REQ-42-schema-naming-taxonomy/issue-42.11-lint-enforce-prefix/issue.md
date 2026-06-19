# issue-42.11-lint-enforce-prefix

**Origen:** `REQ-42-schema-naming-taxonomy`
**Prioridad tentativa:** alta
**Tipo:** tooling / enforcement

## Historia de usuario

**Como** mantenedor del schema de `domain-backend`
**Quiero** que el linter de migraciones (`internal/dbconvlint`) rechace toda `CREATE TABLE` cuyo nombre no empiece con un prefijo de dominio válido de la taxonomía (salvo nombres canónicos documentados)
**Para** que la convención "TODA tabla lleva prefijo de su funcionalidad" deje de ser una guía manual y pase a estar enforced de forma automática en CI, `make db-lint` y el pre-commit, sin que nadie pueda introducir una tabla sin prefijo por descuido

## Criterios de aceptación

```gherkin
Feature: Enforce de prefijo de dominio en CREATE TABLE (regla require-table-prefix)

  Background:
    Given el linter dbconvlint corre sobre archivos *.up.sql de migrations
    And la regla require-table-prefix forma parte de checkCreateTableConventions

  Scenario: una migration con CREATE TABLE sin prefijo válido falla el lint
    Given una migration que declara "CREATE TABLE budgets (...)"
    When ejecuto el linter sobre ese archivo
    Then el resultado incluye la regla "require-table-prefix"
    And el mensaje nombra la tabla 'budgets' y lista los prefijos válidos

  Scenario: una migration con CREATE TABLE con prefijo válido pasa
    Given una migration que declara "CREATE TABLE agent_runs (...)"
    When ejecuto el linter sobre ese archivo
    Then el resultado NO incluye la regla "require-table-prefix"

  Scenario: los nombres canónicos resueltos (allowlist) no requieren prefijo
    Given una migration que declara "CREATE TABLE users (...)", "CREATE TABLE roles (...)", "CREATE TABLE user_roles (...)" o "CREATE TABLE issues (...)"
    When ejecuto el linter sobre ese archivo
    Then el resultado NO incluye la regla "require-table-prefix"

  Scenario: cualquier OTRA tabla nueva sin prefijo válido se rechaza
    Given una migration que declara "CREATE TABLE clients (...)" (no canónica, sin prefijo)
    When ejecuto el linter sobre ese archivo
    Then el resultado incluye la regla "require-table-prefix"

  Scenario: el override silencia la regla en una línea puntual
    Given una migration con "-- domain-lint-ignore-next: require-table-prefix" sobre el CREATE TABLE
    When ejecuto el linter sobre ese archivo
    Then el resultado NO incluye la regla "require-table-prefix" para esa tabla

  Scenario: la regla NO rompe las migrations históricas (baseline)
    Given las 131 migrations históricas con tablas sin prefijo (plans, budgets, sessions, ...)
    And el linter corre con "-baseline 146"
    When ejecuto el job CI db-conventions-lint
    Then las migrations con número <= 146 se ignoran
    And solo las migrations nuevas (número > 146) son enforced

  Scenario: la regla NO aplica a ALTER TABLE ... RENAME TO
    Given una migration estilo 000146 con "ALTER TABLE org_flow_config RENAME TO flow_config"
    When ejecuto el linter sobre ese archivo
    Then el resultado NO incluye la regla "require-table-prefix"
    # extractCreateTables solo detecta CREATE TABLE; los renames no pasan por ahí (correcto)
```

## Qué se toca (verificado contra el código real)

| Archivo | Cambio |
|---|---|
| `services/domain-backend/internal/dbconvlint/lint.go` | Agregar `validTablePrefixes` (allowlist), `canonicalTableExceptions`, helper `hasValidTablePrefix`, y el check `require-table-prefix` dentro de `checkCreateTableConventions` (línea ~247), justo tras el bloque `naming-plural-table` y antes de `require-created-at`. Sumar la regla al doc-comment de cabecera (líneas 4-14). |
| `services/domain-backend/internal/dbconvlint/lint_test.go` | Tests nuevos siguiendo el patrón `issueRules()` + `validHeader`: `TestLint_RequireTablePrefix_FailsUnprefixed`, `_AllowsPrefixed`, `_CanonicalException`, `_Override`. |
| `services/domain-backend/Makefile` (target `db-lint`) | Agregar `-baseline 146` al comando `go run ./cmd/db-conventions-lint -dir internal/migrate/migrations`. |
| `services/domain-backend/.github/workflows/ci-backend.yml` (o raíz) job `db-conventions-lint` | Agregar `-baseline 146` para no enrojecer contra históricas. |

## Por qué NO hay infra nueva

El repo YA resolvió el linter de migraciones. **NO es squawk** (squawk figura como best-effort/opcional en `Makefile` y `.squawk.toml`). El enforce real lo hace el linter Go propio:

- Paquete `internal/dbconvlint` + entrypoint `cmd/db-conventions-lint`.
- Enganchado en TRES lugares ya existentes: (1) job `db-conventions-lint` en CI, (2) target `make db-lint`, (3) pre-commit opcional vía `make install-githooks`.

Por eso la regla de prefijo se agrega como UNA regla más (`require-table-prefix`) dentro de la función existente `checkCreateTableConventions`. Reusa `extractCreateTables` (balanceo de paréntesis robusto, ya probado). Queda enganchada automáticamente en CI/Makefile/pre-commit sin tocar workflows nuevos. Respeta el sistema de overrides (`-- domain-lint-ignore-next: require-table-prefix`) y el flag `-baseline` (ya soportado por `cmd/main.go`).

## Análisis breve

- **Qué pide realmente:** que el linter falle cuando una `CREATE TABLE` nueva no tiene prefijo de dominio. Test de sabotaje: tabla sin prefijo → falla; tabla con prefijo → pasa.
- **Módulos a tocar:** `internal/dbconvlint/lint.go`, `internal/dbconvlint/lint_test.go`, `Makefile`, workflow CI.
- **Riesgos / dependencias:** las 131 migrations históricas no cumplen la regla; sin `-baseline 146` el CI se pone rojo de inmediato. Las excepciones canónicas (`users`, `roles`, `user_roles`, `issues`) están RESUELTAS (REQ-42.8): van en la allowlist, no son open_question.
- **Esfuerzo tentativo:** S
- **Esta HU NO tiene migration** (es tooling de enforcement, no cambia el schema).

## Verificación previa

- [x] `cmd/db-conventions-lint/main.go` ya soporta `-baseline` (flag `baseline` con `n <= *baseline` → continue). **Confirmado.**
- [x] `checkCreateTableConventions` existe en `lint.go:247` y reusa `extractCreateTables`. **Confirmado.**
- [x] El bloque `naming-plural-table` está antes de `require-created-at`; el nuevo check va entre ambos. **Confirmado.**
- [x] El sistema de overrides (`parseOverrides` + `overrideKey`) ya cubre cualquier `rule` arbitraria por nombre. **Confirmado.**
- [x] `make db-lint` corre `go run ./cmd/db-conventions-lint -dir internal/migrate/migrations` SIN baseline hoy. **Confirmado — hay que agregar `-baseline 146`.**
- [x] Excepciones canónicas RESUELTAS (REQ-42.8): `users`, `roles`, `user_roles`, `issues` (+ `schema_migrations`) van en `canonicalTableExceptions` (allowlist). NO se prefijan. El lint los permite exactos y rechaza cualquier OTRA tabla nueva sin prefijo. **Decisión canónica, no open_question.**
- [x] `enrollment_` es un prefijo válido de la allowlist (`enrollment_tokens`, single-org). Decisión tomada: queda en `validTablePrefixes`.

### Resultado de verificación

- **Estado:** verificado (naming canónico RESUELTO: `users`/`roles`/`user_roles`/`issues` en allowlist; baseline 146 confirmado).
- **Evidencia:** lectura directa de `lint.go` (líneas 4-14, 187-289) y `cmd/db-conventions-lint/main.go` (flag baseline); decisión canónica de REQ-42.8.
- **Acción derivada:** implementar la regla; el baseline 146 es decisión confirmada por contexto del proyecto (última migration aplicada = 146).
