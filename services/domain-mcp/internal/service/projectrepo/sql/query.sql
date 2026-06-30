-- name: InsertRepo :one
INSERT INTO project_repositories
  (project_id, name, url, branch_default, kind, is_default, workflow, notes, root_path)
VALUES (
  sqlc.arg('project_id'),
  sqlc.arg('name'),
  sqlc.arg('url'),
  NULLIF(sqlc.arg('branch_default')::text, ''),
  NULLIF(sqlc.arg('kind')::text, ''),
  sqlc.arg('is_default'),
  NULLIF(sqlc.arg('workflow')::text, ''),
  NULLIF(sqlc.arg('notes')::text, ''),
  NULLIF(sqlc.arg('root_path')::text, '')
)
RETURNING id, project_id, name, url,
  COALESCE(branch_default, '')::text AS branch_default,
  COALESCE(kind, '')::text AS kind,
  is_default,
  COALESCE(workflow, '')::text AS workflow,
  COALESCE(notes, '')::text AS notes,
  COALESCE(root_path, '')::text AS root_path,
  created_at, updated_at, deleted_at;

-- name: ListReposByProject :many
SELECT id, project_id, name, url,
  COALESCE(branch_default, '')::text AS branch_default,
  COALESCE(kind, '')::text AS kind,
  is_default,
  COALESCE(workflow, '')::text AS workflow,
  COALESCE(notes, '')::text AS notes,
  COALESCE(root_path, '')::text AS root_path,
  created_at, updated_at, deleted_at
FROM project_repositories
WHERE project_id = sqlc.arg('project_id') AND deleted_at IS NULL
ORDER BY is_default DESC, name ASC;

-- name: GetRepoByID :one
SELECT id, project_id, name, url,
  COALESCE(branch_default, '')::text AS branch_default,
  COALESCE(kind, '')::text AS kind,
  is_default,
  COALESCE(workflow, '')::text AS workflow,
  COALESCE(notes, '')::text AS notes,
  COALESCE(root_path, '')::text AS root_path,
  created_at, updated_at, deleted_at
FROM project_repositories
WHERE id = sqlc.arg('id') AND deleted_at IS NULL;

-- name: GetRepoByName :one
SELECT id, project_id, name, url,
  COALESCE(branch_default, '')::text AS branch_default,
  COALESCE(kind, '')::text AS kind,
  is_default,
  COALESCE(workflow, '')::text AS workflow,
  COALESCE(notes, '')::text AS notes,
  COALESCE(root_path, '')::text AS root_path,
  created_at, updated_at, deleted_at
FROM project_repositories
WHERE project_id = sqlc.arg('project_id') AND name = sqlc.arg('name') AND deleted_at IS NULL;

-- name: UpdateRepo :one
UPDATE project_repositories
SET
  url            = CASE WHEN sqlc.narg('url')::text            IS NOT NULL THEN sqlc.narg('url')::text            ELSE url            END,
  branch_default = CASE WHEN sqlc.narg('branch_default')::text IS NOT NULL THEN NULLIF(sqlc.narg('branch_default')::text, '') ELSE branch_default END,
  kind           = CASE WHEN sqlc.narg('kind')::text           IS NOT NULL THEN NULLIF(sqlc.narg('kind')::text, '')           ELSE kind           END,
  workflow       = CASE WHEN sqlc.narg('workflow')::text       IS NOT NULL THEN NULLIF(sqlc.narg('workflow')::text, '')       ELSE workflow       END,
  notes          = CASE WHEN sqlc.narg('notes')::text          IS NOT NULL THEN NULLIF(sqlc.narg('notes')::text, '')          ELSE notes          END,
  root_path      = CASE WHEN sqlc.narg('root_path')::text      IS NOT NULL THEN NULLIF(sqlc.narg('root_path')::text, '')      ELSE root_path      END
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING id, project_id, name, url,
  COALESCE(branch_default, '')::text AS branch_default,
  COALESCE(kind, '')::text AS kind,
  is_default,
  COALESCE(workflow, '')::text AS workflow,
  COALESCE(notes, '')::text AS notes,
  COALESCE(root_path, '')::text AS root_path,
  created_at, updated_at, deleted_at;

-- name: GetRepoProjectID :one
SELECT project_id FROM project_repositories
WHERE id = sqlc.arg('id') AND deleted_at IS NULL;

-- name: ClearProjectDefault :exec
UPDATE project_repositories
SET is_default = false
WHERE project_id = sqlc.arg('project_id') AND id <> sqlc.arg('id') AND is_default = true;

-- name: SetRepoAsDefault :one
UPDATE project_repositories
SET is_default = true
WHERE id = sqlc.arg('id')
RETURNING id, project_id, name, url,
  COALESCE(branch_default, '')::text AS branch_default,
  COALESCE(kind, '')::text AS kind,
  is_default,
  COALESCE(workflow, '')::text AS workflow,
  COALESCE(notes, '')::text AS notes,
  COALESCE(root_path, '')::text AS root_path,
  created_at, updated_at, deleted_at;

-- name: SoftDeleteRepo :execrows
UPDATE project_repositories
SET deleted_at = NOW(), is_default = false
WHERE id = sqlc.arg('id') AND deleted_at IS NULL;
