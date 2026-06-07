# Design: HU-25.13-schema-conventions-linter

## Architecture

```
cmd/domain-lint-schema/
  main.go              # CLI
internal/lint/schema/
  rules.go             # Rule interface + registry
  ast.go               # pg_query_go wrapper
  parser.go            # header extractor
  rules/
    naming.go          # plural snake_case
    required_cols.go   # id/created_at/updated_at
    types.go           # JSON→JSONB, etc.
    fk_naming.go       # _id suffix
    triggers.go        # set_updated_at_<table>
    header.go          # required fields
  fix/
    json_to_jsonb.go
    pluralize.go
  output/
    reviewdog.go       # CI annotation
    plain.go           # human readable
```

## Rule interface

```go
type Rule interface {
  ID() string                          // "prefer-jsonb"
  Description() string
  Check(ctx *Context, stmt *pgquery.Node) []Issue
  Fix(ctx *Context, raw string, stmt *pgquery.Node) (string, bool)  // bool=fixed
}

type Issue struct {
  Rule     string
  Severity string  // error | warning
  Line     int
  Col      int
  Message  string
  Hint     string
}
```

## CLI usage

```bash
# CI default
domain-lint-schema migrations/*.sql --format=reviewdog

# Local human
make db-conventions-lint

# Auto-fix
domain-lint-schema --fix migrations/*.sql

# Baseline (run once tras crear esta HU)
domain-lint-schema --baseline > .schema-lint-baseline
# subsequent runs respect baseline
```

## Override syntax

```sql
-- domain-lint-ignore-next: prefer-jsonb
-- reason: external lib expects raw JSON
ALTER TABLE foo ADD COLUMN raw JSON;
```

## TDD plan

1. Each rule: fixture violation → reported with line/col
2. Override comment → skip
3. Fix mode generates diff for JSON→JSONB
4. Header missing field → error
5. FK without _id → error
6. Migration limpia → 0 issues
7. Baseline marker excluye legacy
8. Self-test: cada rule referenciada en db.md tiene rule implementada
