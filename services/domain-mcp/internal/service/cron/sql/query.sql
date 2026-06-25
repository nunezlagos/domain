-- name: InsertCron :one
INSERT INTO crons
   (created_by, slug, name, description, cron_expression,
    timezone, target_type, target_id, inputs, enabled, next_run_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING id, organization_id, created_by, slug, name, COALESCE(description,'')::text AS description,
          cron_expression, timezone, target_type, target_id, inputs, enabled,
          last_run_at, next_run_at, created_at, updated_at;

-- name: ListCrons :many
SELECT id, organization_id, created_by, slug, name, COALESCE(description,'')::text AS description,
       cron_expression, timezone, target_type, target_id, inputs, enabled,
       last_run_at, next_run_at, created_at, updated_at
FROM crons WHERE deleted_at IS NULL
ORDER BY created_at DESC
LIMIT sqlc.arg('result_limit')::int;

-- name: GetCronByID :one
SELECT id, organization_id, created_by, slug, name, COALESCE(description,'')::text AS description,
       cron_expression, timezone, target_type, target_id, inputs, enabled,
       last_run_at, next_run_at, created_at, updated_at
FROM crons WHERE id = $1 AND deleted_at IS NULL;

-- name: PickDueCrons :many
SELECT id, organization_id, created_by, slug, name, COALESCE(description,'')::text AS description,
       cron_expression, timezone, target_type, target_id, inputs, enabled,
       last_run_at, next_run_at, created_at, updated_at
FROM crons
WHERE enabled = true AND deleted_at IS NULL
  AND next_run_at IS NOT NULL AND next_run_at <= NOW()
ORDER BY next_run_at ASC
LIMIT sqlc.arg('result_limit')::int
FOR UPDATE SKIP LOCKED;

-- name: UpdateCronRun :exec
UPDATE crons SET last_run_at = $1, next_run_at = $2 WHERE id = $3;

-- name: SetCronEnabled :execrows
UPDATE crons SET enabled = $1 WHERE id = $2 AND deleted_at IS NULL;

-- name: SoftDeleteCron :execrows
UPDATE crons SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL;
