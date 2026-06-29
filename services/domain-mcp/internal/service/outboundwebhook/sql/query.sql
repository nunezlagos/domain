-- name: InsertSubscription :one
INSERT INTO webhook_outbound_subscriptions
	(name, url, events, filters, secret_cipher)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, created_at, updated_at;

-- name: ListByEvent :many
SELECT id, name, url, events, filters, active,
	failure_count, last_success_at, last_failure_at, created_at, updated_at
FROM webhook_outbound_subscriptions
WHERE active = TRUE
  AND sqlc.arg('event_type')::text = ANY(events);

-- name: GetByID :one
SELECT id, name, url, events, filters, active,
	failure_count, last_success_at, last_failure_at, created_at, updated_at
FROM webhook_outbound_subscriptions
WHERE id = $1;

-- name: ListAll :many
SELECT id, name, url, events, filters, active,
	failure_count, last_success_at, last_failure_at, created_at, updated_at
FROM webhook_outbound_subscriptions
ORDER BY created_at DESC;

-- name: DeleteSubscription :execrows
DELETE FROM webhook_outbound_subscriptions WHERE id = $1;

-- name: GetSecretCipher :one
SELECT secret_cipher FROM webhook_outbound_subscriptions WHERE id = $1;
