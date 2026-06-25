-- name: InsertClient :one
INSERT INTO project_clients
   (name, slug, tax_id, contact_email, contact_phone, address, metadata, status)
 VALUES (sqlc.arg('name'), sqlc.arg('slug'),
         sqlc.arg('tax_id'), sqlc.arg('contact_email'),
         sqlc.arg('contact_phone'), sqlc.arg('address'),
         sqlc.arg('metadata'), sqlc.arg('status'))
 RETURNING id, name, slug,
           COALESCE(tax_id, '')::text, COALESCE(contact_email, '')::text,
           COALESCE(contact_phone, '')::text, COALESCE(address, '')::text,
           metadata, status, created_at, updated_at, deleted_at;

-- name: GetClientByID :one
SELECT id, name, slug,
       COALESCE(tax_id, '')::text, COALESCE(contact_email, '')::text,
       COALESCE(contact_phone, '')::text, COALESCE(address, '')::text,
       metadata, status, created_at, updated_at, deleted_at
FROM project_clients
WHERE id = sqlc.arg('id') AND deleted_at IS NULL;

-- name: GetClientBySlug :one
SELECT id, name, slug,
       COALESCE(tax_id, '')::text, COALESCE(contact_email, '')::text,
       COALESCE(contact_phone, '')::text, COALESCE(address, '')::text,
       metadata, status, created_at, updated_at, deleted_at
FROM project_clients
WHERE slug = sqlc.arg('slug') AND deleted_at IS NULL;

-- name: UpdateClient :one
UPDATE project_clients
SET name = sqlc.arg('name'),
    tax_id = sqlc.arg('tax_id'),
    contact_email = sqlc.arg('contact_email'),
    contact_phone = sqlc.arg('contact_phone'),
    address = sqlc.arg('address'),
    metadata = sqlc.arg('metadata'),
    status = sqlc.arg('status')
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING id, name, slug,
          COALESCE(tax_id, '')::text, COALESCE(contact_email, '')::text,
          COALESCE(contact_phone, '')::text, COALESCE(address, '')::text,
          metadata, status, created_at, updated_at, deleted_at;

-- name: SoftDeleteClient :execrows
UPDATE project_clients SET deleted_at = NOW()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL;

-- name: RestoreClient :execrows
UPDATE project_clients SET deleted_at = NULL
WHERE id = sqlc.arg('id') AND deleted_at IS NOT NULL;

-- name: SetClientStatus :one
UPDATE project_clients SET status = sqlc.arg('status')
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING id, name, slug,
          COALESCE(tax_id, '')::text, COALESCE(contact_email, '')::text,
          COALESCE(contact_phone, '')::text, COALESCE(address, '')::text,
          metadata, status, created_at, updated_at, deleted_at;
