# Design: issue-42.11-lint-enforce-prefix

## Decisión arquitectónica

**Una regla más del linter Go existente, NO infra nueva.** El enforce real de convenciones SQL ya lo hace `internal/dbconvlint` + `cmd/db-conventions-lint`, enganchado en CI (`db-conventions-lint`), `make db-lint` y el pre-commit opcional. Agregamos `require-table-prefix` dentro de `checkCreateTableConventions`, reusando `extractCreateTables` (balanceo manual de paréntesis, ya probado contra `gen_random_uuid()` y string literals). No tocamos workflows ni agregamos parsers: la regla queda enganchada en los tres puntos automáticamente.

**Allowlist estática en código (no `.taxonomy.json`).** Cada grupo nuevo exige editar `validTablePrefixes`, pero el tradeoff es a favor: simple, auditable y testeable, sin I/O ni un punto de fallo extra. Se descarta leer la allowlist de un JSON versionado por flexibilidad innecesaria en este punto.

**El underscore final es parte del prefijo.** `flow_` (no `flow`) evita que `flowers` pase la regla por contener `flow`. El reviewer humano cubre el resto de colisiones improbables.

**Excepciones canónicas explícitas (RESUELTAS).** `users`, `issues`, `roles`, `user_roles` y `schema_migrations` quedan en `canonicalTableExceptions` (nombre = grupo, estilo Rails/Postgres + tooling interno de golang-migrate). Es decisión CANÓNICA RESUELTA de REQ-42 (ver 42.8): `users`/`roles`/`user_roles` conservan su nombre, NO se renombran a `users_users`/`users_roles`/`users_user_roles`; `issues` es el nombre canónico del grupo `issue_`. El lint PERMITE esos 5 nombres exactos sin prefijo y RECHAZA cualquier OTRA tabla nueva sin prefijo válido. NO es open_question.

**Solo CREATE TABLE, no ALTER ... RENAME TO.** `extractCreateTables` solo matchea `CREATE TABLE`; los renames (estilo 000146) no pasan por ahí. Es lo correcto: el objetivo es enforcing de tablas FUTURAS, no reescribir históricas.

## DDL / snippets concretos

### `internal/dbconvlint/lint.go` — doc-comment de cabecera (líneas 4-14)

Agregar a la lista de reglas core:

```go
//	require-table-prefix   — CREATE TABLE debe empezar con prefijo de dominio (taxonomía)
```

### `internal/dbconvlint/lint.go` — nuevas declaraciones

```go
// Prefijos de funcionalidad válidos (taxonomía objetivo). Incluye el underscore
// final para forzar agrupación real (flow_, no que pase "flowers"). Mantener
// sincronizado con la taxonomía del proyecto.
var validTablePrefixes = []string{
	"agent_", "audit_", "auth_", "cron_", "external_", "file_",
	"flow_", "issue_", "knowledge_", "mcp_", "notification_",
	"platform_", "project_", "prompt_", "runner_", "sdd_", "seed_",
	"skill_", "tdd_", "usage_", "users_", "webhook_",
	"enrollment_", // enrollment_tokens (single-org; ver risks)
}

// Nombres canónicos RESUELTOS (allowlist del lint): nombre = grupo, excepción
// documentada a la regla "toda tabla lleva prefijo" (estilo Rails/Postgres).
// Decisión CANÓNICA RESUELTA de REQ-42 (no open_question). NO requieren prefijo.
var canonicalTableExceptions = map[string]bool{
	"users":             true, // grupo users_, nombre canónico (REQ-42.8)
	"roles":             true, // grupo users_, catálogo RBAC (REQ-42.8)
	"user_roles":        true, // grupo users_, tabla puente (REQ-42.8)
	"issues":            true, // grupo issue_, nombre canónico
	"schema_migrations": true, // tooling interno golang-migrate
}

func hasValidTablePrefix(name string) bool {
	for _, p := range validTablePrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}
```

### `internal/dbconvlint/lint.go` — el check dentro de `checkCreateTableConventions`

Insertar TRAS el bloque `naming-plural-table` y ANTES de `require-created-at`:

```go
		// require-table-prefix: la tabla debe agruparse por dominio
		if !canonicalTableExceptions[name] && !hasValidTablePrefix(name) {
			add(line, "require-table-prefix",
				fmt.Sprintf("table '%s' must start with a functional-domain prefix (%s) "+
					"or be a documented canonical name. "+
					"Override: -- domain-lint-ignore-next: require-table-prefix",
					name, strings.Join(validTablePrefixes, ", ")))
		}
```

`add`, `name`, `line` y `body` ya están en scope dentro del loop `for _, t := range extractCreateTables(src)`. No requiere cambios de firma.

### `Makefile` — target `db-lint`

```makefile
db-lint:
	go run ./cmd/db-conventions-lint -dir internal/migrate/migrations -baseline 146
	@command -v squawk >/dev/null 2>&1 && squawk internal/migrate/migrations/*.up.sql || echo "squawk no instalado (opcional)"
```

### CI — job `db-conventions-lint`

```yaml
      - name: db-conventions-lint
        run: go run ./cmd/db-conventions-lint -dir internal/migrate/migrations -baseline 146
```

`-baseline 146` es válido: `cmd/main.go` ya parsea el flag y hace `continue` si el número de migration `<= baseline`. Última migration aplicada = 146 → solo se enforce > 146.

## Riesgos y mitigaciones

| Riesgo | Mitigación |
|---|---|
| Falsos positivos históricos: 131 migrations (plans, budgets, sessions, model_registry, observations, clients...) NO cumplen. Sin baseline el CI se pone rojo al instante. | `-baseline 146` en `make db-lint` y en CI. DECISIÓN tomada: baseline 146 (última aplicada). Alternativa: correr primero la migration de renames de la taxonomía. |
| Allowlist estática: cada grupo nuevo exige editar `validTablePrefixes`. | Tradeoff aceptado: simple/auditable/testeable. Se documenta en el comentario "Mantener sincronizado con la taxonomía". |
| Excepciones canónicas (`users`, `roles`, `user_roles`, `issues`). | RESUELTAS (REQ-42.8): conservan su nombre, van en `canonicalTableExceptions` (allowlist). NO es open_question. Cualquier OTRA tabla nueva sin prefijo válido sí se rechaza. |
| `enrollment_` huérfano rompe la propia regla "prefijo DE GRUPO". | Deuda de naming, no de seguridad. Si se decide `auth_enrollment_tokens`, sacar `enrollment_` de la allowlist. |
| Solo CREATE TABLE estático: un CREATE vía función PL/pgSQL no se detecta. | Bajo riesgo: las migrations del repo son DDL estático. Igual limitación que el resto de las reglas (regex-based, no parser SQL). |
| Colisión por prefijo (`flowers` contiene `flow`). | Mitigado: cada entry lleva el underscore (`flow_`), exige el separador. |
| La regla NO aplica a `ALTER TABLE ... RENAME TO` un nombre sin prefijo. | Por diseño correcto: enforce de creaciones futuras. Un rename a nombre malo lo cubre el reviewer humano, no esta regla. |

## TDD plan

1. **Red**: `TestLint_RequireTablePrefix_FailsUnprefixed` — `CREATE TABLE budgets (...)` → `require.Contains(issueRules, "require-table-prefix")`. (`budgets` es plural → no dispara `naming-plural-table`, aísla la regla nueva.)
2. **Green**: agregar las declaraciones + el check.
3. **Red**: `TestLint_RequireTablePrefix_AllowsPrefixed` — `CREATE TABLE agent_runs (...)` → `require.NotContains`.
4. **Green**: confirmado por `hasValidTablePrefix`.
5. **Red**: `TestLint_RequireTablePrefix_CanonicalException` — `CREATE TABLE users (...)`, `CREATE TABLE roles (...)`, `CREATE TABLE user_roles (...)` e `CREATE TABLE issues (...)` → `require.NotContains`.
6. **Green**: confirmado por `canonicalTableExceptions`.
7. **Red**: `TestLint_RequireTablePrefix_Override` — `-- domain-lint-ignore-next: require-table-prefix` sobre `CREATE TABLE budgets` → `require.NotContains`.
8. **Green**: confirmado por `parseOverrides` existente (sin tocar; cubre cualquier rule por nombre).
9. **Sabotaje** (ver tasks.md): quitar el check `require-table-prefix` del código → el test `_FailsUnprefixed` debe ponerse ROJO. Confirma que el test mide la regla, no un efecto colateral.
