-- name: GetFileByProjectAndPath :one
SELECT id, project_id, source_tool, rel_path, original_content,
       content_hash, size_bytes, status, replaced_with, replaced_at,
       restored_at, created_at, updated_at
FROM project_imported_workflow_files
WHERE project_id = sqlc.narg('project_id') AND rel_path = @rel_path;

-- name: UpsertFile :one
INSERT INTO project_imported_workflow_files
  (project_id, source_tool, rel_path, original_content,
   content_hash, size_bytes, status, replaced_with, replaced_at)
VALUES (sqlc.narg('project_id'), @source_tool, @rel_path, @original_content,
        @content_hash, @size_bytes, @status, @replaced_with,
        CASE WHEN @status = 'replaced' THEN now() ELSE NULL END)
ON CONFLICT (project_id, rel_path) DO UPDATE
SET original_content = EXCLUDED.original_content,
    content_hash     = EXCLUDED.content_hash,
    size_bytes       = EXCLUDED.size_bytes,
    status           = EXCLUDED.status,
    replaced_with    = EXCLUDED.replaced_with,
    replaced_at      = EXCLUDED.replaced_at,
    updated_at       = now()
RETURNING id, project_id, source_tool, rel_path, original_content,
          content_hash, size_bytes, status, replaced_with, replaced_at,
          restored_at, created_at, updated_at;

-- name: GetFileByRelPath :one
SELECT id, source_tool, rel_path, original_content, status, replaced_at
FROM project_imported_workflow_files
WHERE (sqlc.narg('project_id')::uuid IS NULL AND project_id IS NULL OR project_id = sqlc.narg('project_id')::uuid)
  AND rel_path = @rel_path;

-- name: SetFileRestored :exec
UPDATE project_imported_workflow_files
SET status = @status, restored_at = now(), updated_at = now()
WHERE id = @id;

-- name: ListProjectFiles :many
SELECT id, project_id, source_tool, rel_path, original_content,
       content_hash, size_bytes, status, replaced_with, replaced_at,
       restored_at, created_at, updated_at
FROM project_imported_workflow_files
WHERE (sqlc.narg('project_id')::uuid IS NULL AND project_id IS NULL OR project_id = sqlc.narg('project_id')::uuid)
ORDER BY rel_path ASC;
