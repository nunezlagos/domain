# Design: issue-23.1-legacy-import

## Schema

```sql
CREATE TABLE import_jobs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id),
  project_id UUID NOT NULL REFERENCES projects(id),
  created_by UUID NOT NULL REFERENCES users(id),
  format VARCHAR(50) NOT NULL, -- markdown-vault | json-dump | notion | obsidian
  source_s3_key VARCHAR(500),
  status VARCHAR(20) NOT NULL DEFAULT 'pending',
  counts JSONB DEFAULT '{}',   -- {created, skipped, failed}
  errors JSONB DEFAULT '[]',
  started_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE import_dedup (
  organization_id UUID NOT NULL,
  project_id UUID NOT NULL,
  content_hash BYTEA NOT NULL,
  entity_type VARCHAR(50) NOT NULL,
  entity_id UUID NOT NULL,
  imported_at TIMESTAMPTZ DEFAULT NOW(),
  PRIMARY KEY (organization_id, project_id, content_hash)
);
```

## Components

```
internal/importers/
  importer.go        # interface
  markdown.go        # markdown-vault
  jsondump.go        # json-dump
  notion.go          # notion zip
  obsidian.go        # obsidian vault
  registry.go        # map format → impl
  worker.go          # picks pending jobs

internal/http/handlers/
  imports.go
```

## TDD plan

1. Fixtures por formato (testdata/imports/) → counts esperados
2. Idempotencia: 2da pasada todos skipped
3. Error fixture corrupto → registrado en errors[]
4. Job status polling
