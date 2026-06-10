# Design: issue-03.7-cross-project-global-search

## Decisión arquitectónica

**Storage:** view materializada `searchable_entities` particionada por org_id.
**Search engine:** Postgres nativo (tsvector GIN + pgvector ivfflat) — sin Elasticsearch/OpenSearch.
**Scoring híbrido:** RRF (Reciprocal Rank Fusion) por default; fallback weighted score si custom weights.
**RBAC:** precompute `user_accessible_projects` en Redis/in-memory cache TTL 1min.

## Alternativas descartadas

- **Elasticsearch:** rompe Postgres-only; overhead operacional alto
- **Search en cada tabla separada:** join post-search es lento e inconsistente en ranking
- **Sin view materializada (live UNION):** query plan grande, lento

## Schema

```sql
CREATE MATERIALIZED VIEW searchable_entities AS
  SELECT id, 'observation' AS entity_type, project_id, organization_id,
         content, content_tsv, embedding, tags, created_at, deleted_at
  FROM observations
  UNION ALL
  SELECT id, 'knowledge_doc', project_id, organization_id,
         body, body_tsv, embedding, tags, created_at, deleted_at
  FROM knowledge_docs
  UNION ALL
  SELECT id, 'prompt', project_id, organization_id,
         body, body_tsv, NULL AS embedding, tags, created_at, deleted_at
  FROM prompts
  UNION ALL
  SELECT id, 'session', project_id, organization_id,
         coalesce(summary, '') AS content, summary_tsv, NULL, tags, created_at, deleted_at
  FROM sessions
  WITH NO DATA;

CREATE INDEX ON searchable_entities USING GIN (content_tsv);
CREATE INDEX ON searchable_entities USING ivfflat (embedding vector_cosine_ops);
CREATE INDEX ON searchable_entities (organization_id, project_id)
  WHERE deleted_at IS NULL;

REFRESH MATERIALIZED VIEW CONCURRENTLY searchable_entities;  -- cron cada 1-5min
```

```sql
CREATE TABLE saved_searches (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id),
  organization_id UUID NOT NULL REFERENCES organizations(id),
  name VARCHAR(255) NOT NULL,
  query TEXT NOT NULL,
  filters JSONB DEFAULT '{}',
  created_at TIMESTAMPTZ DEFAULT NOW()
);
```

## Query híbrida (Reciprocal Rank Fusion)

```sql
WITH fts AS (
  SELECT id, ts_rank_cd(content_tsv, q) AS r, row_number() OVER (ORDER BY ts_rank_cd DESC) AS rk
  FROM searchable_entities, plainto_tsquery('spanish', $1) q
  WHERE content_tsv @@ q AND project_id = ANY($accessible) AND deleted_at IS NULL
  LIMIT 100
),
vec AS (
  SELECT id, embedding <=> $2 AS d, row_number() OVER (ORDER BY embedding <=> $2) AS rk
  FROM searchable_entities
  WHERE embedding IS NOT NULL AND project_id = ANY($accessible) AND deleted_at IS NULL
  ORDER BY embedding <=> $2 LIMIT 100
)
SELECT s.*, (coalesce(1.0/(60+f.rk), 0) + coalesce(1.0/(60+v.rk), 0)) AS score
FROM searchable_entities s
LEFT JOIN fts f USING(id) LEFT JOIN vec v USING(id)
WHERE f.rk IS NOT NULL OR v.rk IS NOT NULL
ORDER BY score DESC LIMIT $limit;
```

## TDD plan

1. Híbrido devuelve unión rankeada
2. RBAC: user A no ve datos de project B
3. Filtros entity_type, project_id, date range
4. Performance: dataset 100k → p99 <500ms
5. Saved searches CRUD
6. Suggestions con Levenshtein cuando empty
