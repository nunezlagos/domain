# Design: issue-01.8-platform-policies

## Schema

```sql
CREATE TABLE platform_policies (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug VARCHAR(100) NOT NULL,                  -- "db", "api", "security"
  name VARCHAR(255) NOT NULL,                  -- "Database Conventions"
  kind VARCHAR(50) NOT NULL,                   -- convention | security_rule | architecture | sdd_workflow | observability | migration_rule | linter_config
  body_md TEXT NOT NULL,
  body_md_tsv TSVECTOR GENERATED ALWAYS AS (to_tsvector('spanish', body_md)) STORED,
  body_structured JSONB,                       -- parsed structured rules; null if parse failed
  version INT NOT NULL DEFAULT 1,
  is_active BOOLEAN NOT NULL DEFAULT true,
  parent_version_id UUID REFERENCES platform_policies(id),
  source VARCHAR(20) NOT NULL DEFAULT 'manual', -- manual | seed | import_md
  seed_managed BOOLEAN DEFAULT false,
  is_user_modified BOOLEAN DEFAULT false,
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ DEFAULT NOW(),
  notes TEXT,
  UNIQUE (slug, version)
);
CREATE UNIQUE INDEX ON platform_policies (slug) WHERE is_active = true;
CREATE INDEX ON platform_policies USING GIN (body_md_tsv);
CREATE INDEX ON platform_policies (kind, is_active);
```

## Components

```
internal/policies/
  service.go        # CRUD + versioning + activate
  parser.go         # markdown → body_structured
  renderer.go       # body_structured → markdown templates
  search.go         # FTS query
  cache.go          # in-proc LRU 5min para MCP
internal/mcp/tools/policies/
  get.go            # domain_policy_get
  search.go         # domain_policies_search
cmd/domain/policies/
  import.go         # CLI domain policies import-md
  export.go         # CLI domain policies export-md
```

## Markdown→structured parser

Detecta secciones convencionales:
- `## Naming` con tablas → `body_structured.naming_rules[]`
- `## Tipos` con tablas → `body_structured.type_mappings[]`
- `## Anti-patterns prohibidos` con lista → `body_structured.banned_patterns[]`
- `## Headers obligatorios` → `body_structured.required_headers{}`
- `## Status codes` con tablas → `body_structured.status_codes{}`

Parser tolerant: si no detecta convención, deja `body_structured = null`. Linters fallback a defaults.

## MCP tools

```go
// domain_policy_get
{
  "name": "domain_policy_get",
  "description": "Get the current active platform policy by slug",
  "input_schema": {
    "type": "object",
    "properties": {
      "slug": {"type": "string", "enum": ["db","api","security","testing","observability","migrations","clean-architecture","sdd","go"]},
      "structured_only": {"type": "boolean", "default": false}
    },
    "required": ["slug"]
  }
}

// domain_policies_search
{
  "name": "domain_policies_search",
  "description": "Search platform policies by query (FTS)",
  "input_schema": {
    "type": "object",
    "properties": {
      "query": {"type": "string"},
      "kind": {"type": "string", "enum": ["convention","security_rule","architecture","..."]},
      "limit": {"type": "integer", "default": 5, "maximum": 20}
    },
    "required": ["query"]
  }
}
```

## Workflow boot

```
1. golang-migrate up
2. issue-01.7 seeders:
   - policies seeder lee .claude/rules/*.md embebidos via go:embed
   - UPSERT en platform_policies con source='seed', seed_managed=true
   - parser.go intenta poblar body_structured
3. App ready: MCP tools sirven desde cache+DB
4. Admin edits via API:
   - is_user_modified = true
   - seeders future no sobrescriben
5. dev local: `domain policies export-md` → .claude/rules/*.md regenerated
```

## TDD plan

1. CRUD + audit
2. Versionado preserva histórico
3. Activate version previa = rollback
4. Import .md crea/actualiza
5. Export regenera md (round-trip idempotent)
6. body_md_tsv FTS search
7. body_structured parser sobre fixtures
8. MCP tool get devuelve estructura esperada
9. MCP tool search FTS funciona
10. Cache hit/miss in-proc
11. is_user_modified protege contra seed
12. Sabotaje: import .md corrompido → fail-fast con error
