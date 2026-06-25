-- name: InsertAttachment :one
INSERT INTO file_attachments (
    entity_type, entity_id, filename, s3_key, size_bytes, mime_type, created_by
) VALUES (
    @entity_type, @entity_id, @filename, @s3_key, @size_bytes, @mime_type, @created_by
)
RETURNING id, entity_type, entity_id, filename, s3_key, size_bytes, mime_type, created_by, created_at;

-- name: GetAttachment :one
SELECT id, entity_type, entity_id, filename, s3_key, size_bytes, mime_type, created_by, created_at
FROM file_attachments
WHERE id = @id;

-- name: ListByEntity :many
SELECT id, entity_type, entity_id, filename, s3_key, size_bytes, mime_type, created_by, created_at
FROM file_attachments
WHERE entity_type = @entity_type AND entity_id = @entity_id
ORDER BY created_at DESC;

-- name: DeleteAttachment :one
DELETE FROM file_attachments WHERE id = @id
RETURNING s3_key;

-- name: CleanupOrphans :many
DELETE FROM file_attachments fa
WHERE NOT EXISTS (SELECT 1 FROM issues WHERE id = fa.entity_id AND entity_type = 'user_story')
  AND NOT EXISTS (SELECT 1 FROM sdd_requirements WHERE id = fa.entity_id AND entity_type = 'requirement')
RETURNING s3_key;

-- name: PromoteEntity :execrows
UPDATE file_attachments
SET entity_type = @to_type, entity_id = @to_id
WHERE entity_type = @from_type AND entity_id = @from_id;
