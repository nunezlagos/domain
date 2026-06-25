-- wizardplan: sources/agent_history.go
-- name: ListAgentRunsSince :many
SELECT ar.id, COALESCE(a.slug, '')::text AS agent_slug, ar.started_at,
       LEFT(COALESCE(ar.outputs::text, ''), 200)::text AS summary
FROM agent_runs ar
LEFT JOIN agents a ON a.id = ar.agent_id
WHERE ar.started_at >= now() - sqlc.arg('interval_days')::interval
  AND (sqlc.narg('user_id')::uuid IS NULL OR ar.user_id = sqlc.narg('user_id')::uuid)
ORDER BY ar.started_at DESC
LIMIT sqlc.arg('result_limit')::int;

-- wizardplan: sources/issue_dedup.go
-- name: ListIssuesByFTS :many
SELECT id, slug, title, status,
       ts_rank(to_tsvector('spanish', coalesce(title, '') || ' ' || coalesce(description, '')),
               plainto_tsquery('spanish', sqlc.arg('query')::text)) AS score
FROM issues
WHERE to_tsvector('spanish', coalesce(title, '') || ' ' || coalesce(description, ''))
      @@ plainto_tsquery('spanish', sqlc.arg('query')::text)
ORDER BY score DESC
LIMIT sqlc.arg('result_limit')::int;

-- name: GetRequirementSlugByIssueID :one
SELECT r.slug
FROM issues us
JOIN sdd_requirements r ON r.id = us.req_id
WHERE us.id = sqlc.arg('issue_id')::uuid;
