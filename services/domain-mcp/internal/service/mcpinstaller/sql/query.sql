-- name: ListProviders :many
SELECT name, description, command, default_args, env_template, required_env, tags
FROM mcp_providers
ORDER BY name;
