# Design: HU-09.7-workflow-versioning

## Decisión arquitectónica

**Storage:** tabla flow_versions con spec immutable JSONB + checksum.
**Lifecycle:** draft → published (uno solo "default" a la vez) → deprecated.
**Run binding:** flow_version_id congelado al crear el run.
**Diff:** json-patch RFC 6902.

## Schema

```sql
CREATE TABLE flow_versions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  flow_id UUID NOT NULL REFERENCES flows(id) ON DELETE CASCADE,
  version_number INT NOT NULL,
  spec JSONB NOT NULL,
  spec_checksum BYTEA NOT NULL,    -- SHA256 del spec canonical
  status VARCHAR(20) NOT NULL DEFAULT 'draft',  -- draft | published | deprecated
  is_default BOOLEAN NOT NULL DEFAULT false,
  published_at TIMESTAMPTZ,
  deprecated_at TIMESTAMPTZ,
  notes TEXT,
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ DEFAULT NOW(),
  UNIQUE (flow_id, version_number)
);
CREATE UNIQUE INDEX ON flow_versions (flow_id) WHERE is_default = true;

ALTER TABLE flow_runs
  ADD COLUMN flow_version_id UUID NOT NULL REFERENCES flow_versions(id);
```

## API

| método | path | descripción |
|--------|------|-------------|
| GET | /flows/:id/versions | list versiones |
| POST | /flows/:id (PATCH spec) | crea nueva draft |
| POST | /flows/:id/versions/:n/publish | publish (default flag se mueve) |
| POST | /flows/:id/versions/:n/deprecate | deprecate |
| GET | /flows/:id/versions/diff?from=A&to=B | json-patch diff |
| POST | /flows/:id/run con `{version:N}` | invoca versión específica |

## Publish flow

```sql
BEGIN;
UPDATE flow_versions SET is_default = false WHERE flow_id = $1;
UPDATE flow_versions SET status = 'published', published_at = NOW(), is_default = true
  WHERE id = $version_id;
COMMIT;
```

## Breaking change heurística

```
- step removed: BREAKING
- step type changed: BREAKING
- step input_schema with new required field: BREAKING
- step output_schema with removed field: BREAKING
- step added (new): minor
- step description change: patch
```

## TDD plan

1. Save → draft creado
2. Publish → is_default flipea
3. Run en vuelo con v3 NO afectado por publish v4
4. Invoke versión específica
5. Deprecate → 410 nuevo run
6. Diff json-patch correcto
7. Breaking change flagged
8. Storage cron archive >90d deprecated sin runs
