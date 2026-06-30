-- name: RevokeAllActive :execrows
UPDATE enrollment_tokens SET revoked_at = NOW() WHERE revoked_at IS NULL;

-- name: InsertToken :one
INSERT INTO enrollment_tokens (token_hash, token_prefix, role_on_enroll, created_by_user_id)
VALUES (@token_hash, @token_prefix, @role_on_enroll, @created_by_user_id)
RETURNING created_at;

-- name: GetActiveMetadata :one
SELECT token_prefix, role_on_enroll, created_at
FROM enrollment_tokens
WHERE revoked_at IS NULL;

-- name: FindTokensByPrefix :many
SELECT id, token_hash, role_on_enroll
FROM enrollment_tokens
WHERE token_prefix = @token_prefix AND revoked_at IS NULL;

-- name: InsertUser :one
INSERT INTO users (email, name, role)
VALUES (@email, NULLIF(@name, ''), @role)
RETURNING id, created_at;

-- name: InsertAPIKey :exec
INSERT INTO auth_api_keys (id, user_id, key_hash, key_prefix, name, environment, expires_at)
VALUES (@id, @user_id, @key_hash, @key_prefix, 'default', 'live', NULL);
