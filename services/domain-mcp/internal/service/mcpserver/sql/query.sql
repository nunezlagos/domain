-- name: InsertServer :one
INSERT INTO mcp_servers
    (name, transport, command, args, env_cipher, url)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, created_at, updated_at;

-- name: GetServer :one
SELECT id, name, transport, COALESCE(command, '')::text AS command, args,
    COALESCE(url, '')::text AS url, enabled, status, last_connected_at,
    COALESCE(last_error, '')::text AS last_error, retry_count, created_at, updated_at
FROM mcp_servers WHERE id = $1;

-- name: ListServers :many
SELECT id, name, transport, COALESCE(command, '')::text AS command, args,
    COALESCE(url, '')::text AS url, enabled, status, last_connected_at,
    COALESCE(last_error, '')::text AS last_error, retry_count, created_at, updated_at
FROM mcp_servers ORDER BY created_at DESC;

-- name: DeleteServer :execrows
DELETE FROM mcp_servers WHERE id = $1;

-- name: GetServerEnvCipher :one
SELECT env_cipher FROM mcp_servers WHERE id = $1;

-- name: UpsertTool :one
INSERT INTO mcp_server_tools
    (mcp_server_id, tool_name, description, input_schema, discovered_at)
VALUES ($1, $2, $3, $4, NOW())
ON CONFLICT (mcp_server_id, tool_name) DO UPDATE
SET description = EXCLUDED.description,
    input_schema = EXCLUDED.input_schema,
    discovered_at = NOW(),
    updated_at = NOW()
RETURNING id, mcp_server_id, tool_name, description, input_schema, enabled, discovered_at;

-- name: MarkServerConnected :exec
UPDATE mcp_servers SET status = 'connected', last_connected_at = NOW(),
    last_error = NULL, retry_count = 0
WHERE id = $1;

-- name: MarkServerFailed :exec
UPDATE mcp_servers SET status = 'failed', last_error = $2,
    retry_count = retry_count + 1
WHERE id = $1;

-- name: ListToolsByServer :many
SELECT id, mcp_server_id, tool_name, description, input_schema, enabled, discovered_at
FROM mcp_server_tools
WHERE mcp_server_id = $1 AND enabled = TRUE
ORDER BY tool_name;
