-- name: GetRequirementForIssue :one
SELECT id, project_id FROM sdd_requirements WHERE slug = $1;

-- name: InsertIssue :one
INSERT INTO issues (slug, title, description, status, priority, req_id, project_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, slug, title, description, status, priority, req_id, created_at, updated_at, project_id;

-- name: GetIssueByID :one
SELECT id, slug, title, description, status, priority, req_id, created_at, updated_at, project_id
FROM issues WHERE id = $1;

-- name: GetIssueBySlug :one
SELECT id, slug, title, description, status, priority, req_id, created_at, updated_at, project_id
FROM issues WHERE slug = $1;

-- name: ListIssues :many
SELECT us.id, us.slug, us.title, us.description, us.status, us.priority, us.req_id, us.created_at, us.updated_at, us.project_id
FROM issues us
LEFT JOIN sdd_requirements r ON r.id = us.req_id
WHERE (sqlc.narg('status')::text IS NULL OR us.status = sqlc.narg('status')::text)
  AND (sqlc.narg('priority')::text IS NULL OR us.priority = sqlc.narg('priority')::text)
  AND (sqlc.narg('req_slug')::text IS NULL OR r.slug = sqlc.narg('req_slug')::text)
  AND (sqlc.narg('project_id')::uuid IS NULL OR us.project_id = sqlc.narg('project_id')::uuid)
ORDER BY us.slug
LIMIT $1 OFFSET $2;

-- name: UpdateIssue :one
UPDATE issues SET title = $2, description = $3, status = $4, priority = $5, updated_at = NOW()
WHERE slug = $1
RETURNING id, slug, title, description, status, priority, req_id, created_at, updated_at, project_id;

-- name: DeleteIssue :execrows
DELETE FROM issues WHERE slug = $1;

-- name: MaxScenarioPosition :one
SELECT COALESCE(MAX(position), -1)::int FROM issue_gherkin_scenarios WHERE issue_id = $1;

-- name: InsertScenario :one
INSERT INTO issue_gherkin_scenarios (issue_id, project_id, feature, scenario, given, when_text, then_rows, position)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, issue_id, feature, scenario, given, when_text, then_rows, position, created_at, project_id;

-- name: ListScenariosByIssue :many
SELECT id, issue_id, feature, scenario, given, when_text, then_rows, position, created_at, project_id
FROM issue_gherkin_scenarios WHERE issue_id = $1 ORDER BY position;

-- name: ListScenariosByIssueIDs :many
SELECT id, issue_id, feature, scenario, given, when_text, then_rows, position, created_at, project_id
FROM issue_gherkin_scenarios WHERE issue_id = ANY($1::uuid[]) ORDER BY issue_id, position;

-- name: DeleteScenario :execrows
DELETE FROM issue_gherkin_scenarios WHERE id = $1;
