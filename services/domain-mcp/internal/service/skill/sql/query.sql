-- skill: service.go
-- name: SkillCreate :one
INSERT INTO skills (
  slug, name, description, skill_type, content,
  input_schema, output_schema, timeout_seconds, idempotent,
  has_side_effects, depends_on, tags, embedding
) VALUES (
  sqlc.arg('slug'), sqlc.arg('name'), sqlc.arg('description'),
  sqlc.arg('skill_type'), sqlc.arg('content'),
  sqlc.arg('input_schema'), sqlc.arg('output_schema'),
  sqlc.arg('timeout_seconds'), sqlc.arg('idempotent'),
  sqlc.arg('has_side_effects'), sqlc.arg('depends_on'), sqlc.arg('tags'),
  sqlc.arg('embedding')::vector
)
RETURNING id, slug, name, COALESCE(description,'')::text AS description,
          skill_type, COALESCE(content,'')::text AS content,
          input_schema, output_schema,
          timeout_seconds, idempotent, has_side_effects, depends_on, tags,
          seed_managed, seed_version, is_user_modified, created_at, updated_at;

-- name: SkillUpdate :one
UPDATE skills
SET name = sqlc.arg('name'),
    description = sqlc.arg('description'),
    content = sqlc.arg('content'),
    input_schema = sqlc.arg('input_schema'),
    output_schema = sqlc.arg('output_schema'),
    timeout_seconds = sqlc.arg('timeout_seconds'),
    idempotent = sqlc.arg('idempotent'),
    has_side_effects = sqlc.arg('has_side_effects'),
    depends_on = sqlc.arg('depends_on'),
    tags = sqlc.arg('tags'),
    is_user_modified = sqlc.arg('is_user_modified')
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING id, slug, name, COALESCE(description,'')::text AS description,
          skill_type, COALESCE(content,'')::text AS content,
          input_schema, output_schema,
          timeout_seconds, idempotent, has_side_effects, depends_on, tags,
          seed_managed, seed_version, is_user_modified, created_at, updated_at;

-- name: SkillUpdateWithEmbedding :one
UPDATE skills
SET name = sqlc.arg('name'),
    description = sqlc.arg('description'),
    content = sqlc.arg('content'),
    input_schema = sqlc.arg('input_schema'),
    output_schema = sqlc.arg('output_schema'),
    timeout_seconds = sqlc.arg('timeout_seconds'),
    idempotent = sqlc.arg('idempotent'),
    has_side_effects = sqlc.arg('has_side_effects'),
    depends_on = sqlc.arg('depends_on'),
    tags = sqlc.arg('tags'),
    is_user_modified = sqlc.arg('is_user_modified'),
    embedding = sqlc.arg('embedding')::vector
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING id, slug, name, COALESCE(description,'')::text AS description,
          skill_type, COALESCE(content,'')::text AS content,
          input_schema, output_schema,
          timeout_seconds, idempotent, has_side_effects, depends_on, tags,
          seed_managed, seed_version, is_user_modified, created_at, updated_at;

-- name: SkillGetByID :one
SELECT id, slug, name, COALESCE(description,'')::text AS description,
       skill_type, COALESCE(content,'')::text AS content,
       input_schema, output_schema,
       timeout_seconds, idempotent, has_side_effects, depends_on, tags,
       seed_managed, seed_version, is_user_modified, created_at, updated_at
FROM skills WHERE id = sqlc.arg('id') AND deleted_at IS NULL;

-- name: SkillGetBySlug :one
SELECT id, slug, name, COALESCE(description,'')::text AS description,
       skill_type, COALESCE(content,'')::text AS content,
       input_schema, output_schema,
       timeout_seconds, idempotent, has_side_effects, depends_on, tags,
       seed_managed, seed_version, is_user_modified, created_at, updated_at
FROM skills WHERE slug = sqlc.arg('slug') AND deleted_at IS NULL AND proposed = false;

-- name: SkillList :many
SELECT id, slug, name, COALESCE(description,'')::text AS description,
       skill_type, COALESCE(content,'')::text AS content,
       input_schema, output_schema,
       timeout_seconds, idempotent, has_side_effects, depends_on, tags,
       seed_managed, seed_version, is_user_modified, created_at, updated_at
FROM skills
WHERE deleted_at IS NULL AND proposed = false
  AND (sqlc.narg('skill_type')::text IS NULL OR skill_type = sqlc.narg('skill_type')::text)
  AND (sqlc.narg('tag')::text IS NULL OR sqlc.narg('tag')::text = ANY(tags))
ORDER BY created_at DESC
LIMIT sqlc.arg('result_limit')::int;

-- name: SkillSearchHybridWithVector :many
WITH bm25 AS (
  SELECT id, ROW_NUMBER() OVER (ORDER BY ts_rank(description_tsv, q) DESC) AS r
  FROM skills, plainto_tsquery('spanish', sqlc.arg('query_text')) AS q
  WHERE deleted_at IS NULL AND proposed = false AND description_tsv @@ q
  LIMIT sqlc.arg('candidates')::int
),
vec AS (
  SELECT id, ROW_NUMBER() OVER (ORDER BY embedding <=> sqlc.arg('query_vec')::vector ASC) AS r
  FROM skills
  WHERE deleted_at IS NULL AND proposed = false AND embedding IS NOT NULL
  LIMIT sqlc.arg('candidates')::int
),
fused AS (
  SELECT COALESCE(bm25.id, vec.id) AS id,
         COALESCE(1.0 / (sqlc.arg('rrf_k')::int + bm25.r), 0) + COALESCE(1.0 / (sqlc.arg('rrf_k')::int + vec.r), 0) AS score,
         COALESCE(bm25.r, 0) AS bm25_rank,
         COALESCE(vec.r, 0) AS vec_rank
  FROM bm25 FULL OUTER JOIN vec ON bm25.id = vec.id
)
SELECT s.id, s.slug, s.name, COALESCE(s.description,'')::text AS description,
       s.skill_type, COALESCE(s.content,'')::text AS content,
       s.input_schema, s.output_schema,
       s.timeout_seconds, s.idempotent, s.has_side_effects, s.depends_on, s.tags,
       s.seed_managed, s.seed_version, s.is_user_modified, s.created_at, s.updated_at,
       f.score::float8 AS score, f.bm25_rank::bigint AS bm25_rank, f.vec_rank::bigint AS vec_rank
FROM fused f
JOIN skills s ON s.id = f.id
ORDER BY f.score DESC
LIMIT sqlc.arg('result_limit')::int;

-- name: SkillSearchHybridBM25Only :many
SELECT s.id, s.slug, s.name, COALESCE(s.description,'')::text AS description,
       s.skill_type, COALESCE(s.content,'')::text AS content,
       s.input_schema, s.output_schema,
       s.timeout_seconds, s.idempotent, s.has_side_effects, s.depends_on, s.tags,
       s.seed_managed, s.seed_version, s.is_user_modified, s.created_at, s.updated_at,
       ts_rank(s.description_tsv, q)::float8 AS score,
       0::bigint AS bm25_rank,
       0::bigint AS vec_rank
FROM skills s, plainto_tsquery('spanish', sqlc.arg('query_text')) AS q
WHERE s.deleted_at IS NULL AND s.proposed = false AND s.description_tsv @@ q
ORDER BY score DESC
LIMIT sqlc.arg('result_limit')::int;

-- name: SkillApplicableIDs :many
SELECT s.id FROM skills s
WHERE s.deleted_at IS NULL AND s.proposed = FALSE
  AND (s.project_id IS NULL OR s.project_id = sqlc.arg('project_id'))
  AND NOT EXISTS (
    SELECT 1 FROM project_skills ps
    WHERE ps.skill_id = s.id AND ps.project_id = sqlc.arg('project_id') AND ps.is_enabled = FALSE
  );

-- name: SkillSoftDeleteCountDeps :one
SELECT COUNT(*) FROM skills
WHERE deleted_at IS NULL AND sqlc.arg('slug')::text = ANY(depends_on);

-- name: SkillSoftDelete :exec
UPDATE skills SET deleted_at = NOW() WHERE id = sqlc.arg('id') AND deleted_at IS NULL;

-- skill: versioning.go
-- name: VersionMaxVersion :one
SELECT COALESCE(MAX(version), 0) + 1 FROM skill_versions WHERE skill_id = sqlc.arg('skill_id');

-- name: VersionCreate :one
INSERT INTO skill_versions (skill_id, version, content, input_schema, output_schema, changelog, created_by)
VALUES (sqlc.arg('skill_id'), sqlc.arg('version'), sqlc.arg('content'), sqlc.arg('input_schema'), sqlc.arg('output_schema'), sqlc.arg('changelog'), sqlc.arg('created_by'))
RETURNING id, skill_id, version, content, input_schema, output_schema, changelog, created_by, created_at;

-- name: VersionGetBySkillAndVersion :one
SELECT id, skill_id, version, content, input_schema, output_schema, changelog, created_by, created_at
FROM skill_versions WHERE skill_id = sqlc.arg('skill_id') AND version = sqlc.arg('version');

-- name: VersionListBySkill :many
SELECT id, skill_id, version, content, input_schema, output_schema, changelog, created_by, created_at
FROM skill_versions WHERE skill_id = sqlc.arg('skill_id') ORDER BY version DESC;

-- name: VersionPin :exec
UPDATE skills SET pinned_version = sqlc.arg('version') WHERE id = sqlc.arg('id');

-- name: VersionUnpin :exec
UPDATE skills SET pinned_version = NULL WHERE id = sqlc.arg('id');

-- name: VersionGetPinned :one
SELECT pinned_version FROM skills WHERE id = sqlc.arg('id');

-- skill: execution.go
-- name: ExecutionCreate :one
INSERT INTO skill_executions (skill_id, version_used, mode, status, parameters, started_at)
VALUES (sqlc.arg('skill_id'), sqlc.arg('version_used'), sqlc.arg('mode'), sqlc.arg('status'), sqlc.arg('parameters'), NOW())
RETURNING id, skill_id, version_used, mode, status, parameters, output, error, execution_time_ms, started_at, completed_at, created_at;

-- name: ExecutionGetByID :one
SELECT id, skill_id, version_used, mode, status, parameters, output, error, execution_time_ms, started_at, completed_at, created_at
FROM skill_executions WHERE id = sqlc.arg('id');

-- name: ExecutionSetRunning :exec
UPDATE skill_executions SET status = 'running' WHERE id = sqlc.arg('id') AND status = 'pending';

-- name: ExecutionSetFailed :exec
UPDATE skill_executions SET status = 'failed', error = sqlc.arg('error'), execution_time_ms = sqlc.arg('execution_time_ms'), completed_at = NOW() WHERE id = sqlc.arg('id');

-- name: ExecutionSetCompleted :exec
UPDATE skill_executions SET status = 'completed', output = sqlc.arg('output'), execution_time_ms = sqlc.arg('execution_time_ms'), completed_at = NOW() WHERE id = sqlc.arg('id');

