# Migrations — Linting y Convenciones

> issue-25.3 (safety linter) + issue-25.13 (conventions linter)
> Fuente normativa: `.claude/rules/migrations.md` y `.claude/rules/db.md`

## Cómo correr

```bash
make db-lint       # conventions + safety sobre internal/migrate/migrations
make db-lint-fix   # fixes automáticos (JSON→JSONB, TIMESTAMP→TIMESTAMPTZ)
```

El enforcement primario es `cmd/db-conventions-lint` (Go, sin dependencias
externas) y corre en CI (job `db-conventions-lint` en `ci.yml`) bloqueando
el merge. `squawk` es complementario y opcional (`.squawk.toml`).

## Reglas de safety (bloquean CI)

| Regla | Por qué |
|-------|---------|
| `CREATE INDEX` sin `CONCURRENTLY` | bloquea writes en tablas con datos |
| `ADD COLUMN ... NOT NULL` sin `DEFAULT` | full table rewrite |
| `DROP TABLE` sin `IF EXISTS` | rompe idempotencia |
| `ADD FOREIGN KEY` sin `NOT VALID` | lock + scan completo |
| `VACUUM FULL` | exclusive lock |
| `LOCK TABLE` explícito | riesgo de deadlock en deploy |

## Reglas de conventions

- Header obligatorio de 6 campos (`migration/author/issue/description/breaking/estimated_duration`)
- Tablas plural snake_case; FKs `<singular>_id` con `ON DELETE` explícito
- `JSONB` (no JSON), `TIMESTAMPTZ` (no TIMESTAMP), `NUMERIC` para money
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()` requerido

## Overrides

Cuando una regla no aplica (ej. índice en tabla recién creada y vacía):

```sql
-- domain-lint-ignore-next: require-concurrent-index-creation
-- reason: tabla nueva sin tráfico
CREATE INDEX idx_x ON new_table(col);
```

El comentario `reason:` es obligatorio en code review. El sabotaje
`TestSabotage_OverrideRemoved_RuleFiresAgain` garantiza que quitar el
override re-dispara la regla.

Baseline para migraciones legacy: `db-conventions-lint -baseline N`
(ignora migraciones con número ≤ N).

## Pre-commit hook (opcional)

```bash
make install-githooks
```

Corre el linter si hay `.sql` staged + `go vet`/tests cortos si hay `.go`.
Bypass de emergencia con `--no-verify` requiere post-mortem
(`.claude/rules/git.md`).

## Tests del linter

`internal/dbconvlint/lint_test.go`: 35+ casos incluyendo los escenarios de
cierre de issue-25.3 — CREATE INDEX sin CONCURRENTLY falla
(`TestSafety_CreateIndexConcurrent`), override pasa (`TestSafety_Override`),
NOT NULL sin DEFAULT falla (`TestSafety_AddColumnNotNullSinDefault`),
migración limpia pasa (casos `_OK`), header faltante warning
(`TestLint_HeaderMissing`).
