-- name: InsertWebhook :one
INSERT INTO webhooks
    (created_by, slug, name, secret_encrypted, source_type,
     target_type, target_id, inputs_mapping)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, created_by, slug, name, secret_encrypted, source_type,
          target_type, target_id, inputs_mapping, enabled,
          last_delivery_at, created_at, updated_at, deleted_at;

-- name: GetWebhookBySlug :one
SELECT id, created_by, slug, name, secret_encrypted, source_type,
       target_type, target_id, inputs_mapping, enabled,
       last_delivery_at, created_at, updated_at, deleted_at
FROM webhooks WHERE slug = $1 AND deleted_at IS NULL;

-- name: GetWebhookByID :one
SELECT id, created_by, slug, name, secret_encrypted, source_type,
       target_type, target_id, inputs_mapping, enabled,
       last_delivery_at, created_at, updated_at, deleted_at
FROM webhooks WHERE id = $1 AND deleted_at IS NULL;

-- name: ListWebhooks :many
SELECT id, created_by, slug, name, secret_encrypted, source_type,
       target_type, target_id, inputs_mapping, enabled,
       last_delivery_at, created_at, updated_at, deleted_at
FROM webhooks WHERE deleted_at IS NULL ORDER BY created_at DESC;

-- name: SetWebhookEnabled :one
UPDATE webhooks SET enabled = $1, updated_at = now() WHERE id = $2 AND deleted_at IS NULL
RETURNING id;

-- name: SoftDeleteWebhook :one
UPDATE webhooks SET deleted_at = now(), updated_at = now() WHERE id = $1 AND deleted_at IS NULL
RETURNING id;

-- name: InsertDelivery :exec
INSERT INTO webhook_deliveries
    (webhook_id, payload, headers, source_ip, status, error, triggered_run_id)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: UpdateLastDelivery :exec
UPDATE webhooks SET last_delivery_at = NOW() WHERE id = $1;

-- name: ListDeliveries :many
SELECT id, webhook_id, payload, headers, COALESCE(source_ip,'') AS source_ip,
       status, COALESCE(error,'') AS error, triggered_run_id, received_at
FROM webhook_deliveries WHERE webhook_id = $1
ORDER BY received_at DESC LIMIT $2;

-- name: GetDelivery :one
SELECT id, webhook_id, payload, headers, COALESCE(source_ip,'') AS source_ip,
       status, COALESCE(error,'') AS error, triggered_run_id, received_at
FROM webhook_deliveries WHERE id = $1;
