# Design: issue-25.3-migration-linter

## Tool

**Squawk** (`sbdchd/squawk`) — Rust binary, CI-friendly, opinionated rules sobre migrations Postgres.

Alternativa considerada: **Atlas migrate lint** — más features pero ecosystem heavier; squawk es suficiente para nuestro scope.

## `.squawk.toml`

```toml
[upload-to-github]
exclude_pull_request_summary = false

[rules]
# enforced as errors
adding-required-field = "error"        # NOT NULL sin DEFAULT
disallowed-unique-constraint = "error" # ALTER ADD UNIQUE sin CONCURRENTLY index
require-concurrent-index-creation = "error"
require-concurrent-index-deletion = "error"
prefer-text-field = "warning"          # avoid varchar(N) sin razón
prefer-robust-stmts = "error"          # IF EXISTS / IF NOT EXISTS
ban-drop-column = "warning"            # deprecate first
ban-drop-table = "warning"             # deprecate first
ban-drop-database = "error"
constraint-missing-not-valid = "error" # FK debe ser NOT VALID + VALIDATE later
disallowed-data-types = ["json"]       # prefer jsonb
```

## CI step (issue-19.1)

```yaml
- name: squawk migrations
  run: |
    curl -L https://github.com/sbdchd/squawk/releases/.../squawk-linux > squawk && chmod +x squawk
    ./squawk migrations/*.sql
```

## Override syntax

```sql
-- squawk-ignore: require-concurrent-index-creation
-- reason: first migration on empty table, no traffic yet
CREATE INDEX idx_foo ON observations(content);
```

## Header convention

```sql
-- migration: add_user_rut_column
-- author: alice@x.com
-- issue: #1234
-- description: agrega columna RUT con validation módulo 11
-- breaking: false
-- estimated_duration: 1s (empty table)

ALTER TABLE users ADD COLUMN rut VARCHAR(12) UNIQUE;
```

Linter custom (Go script o pre-commit) valida header presente.

## TDD plan

1. Migration con CREATE INDEX sin CONCURRENTLY → CI fail
2. Override comment hace pasar + audit log CI
3. NOT NULL sin DEFAULT → fail
4. Clean migration → pass
5. Header missing → warning (no fail)
6. make db-lint local idéntico a CI
