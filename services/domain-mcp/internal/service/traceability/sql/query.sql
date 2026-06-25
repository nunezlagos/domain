-- name: GetRequirementBySlug :one
SELECT id, slug, title, status, created_at
FROM sdd_requirements
WHERE slug = $1;

-- name: ListIssuesByReq :many
SELECT id, slug, title, status
FROM issues
WHERE req_id = $1
ORDER BY slug;

-- name: LatestProposal :one
SELECT version, status
FROM sdd_proposals
WHERE issue_id = $1
ORDER BY version DESC
LIMIT 1;

-- name: LatestDesign :one
SELECT version, status
FROM sdd_designs
WHERE issue_id = $1
ORDER BY version DESC
LIMIT 1;

-- name: TaskProgressByIssue :one
SELECT
    COUNT(*) AS total,
    COUNT(*) FILTER (WHERE status = 'completed') AS completed,
    COALESCE(ROUND(100.0 * COUNT(*) FILTER (WHERE status = 'completed') / GREATEST(COUNT(*), 1), 1), 0)::numeric AS pct
FROM issue_tasks
WHERE issue_id = $1;

-- name: ListCodeRefsByIssue :many
SELECT id, file_path, repo, branch
FROM issue_code_references
WHERE issue_id = $1
ORDER BY file_path;

-- name: GetCodeTraceHU :one
SELECT cr.issue_id, us.slug, us.title, us.status
FROM issue_code_references cr
JOIN issues us ON us.id = cr.issue_id
WHERE cr.file_path = $1
LIMIT 1;

-- name: GetRequirementForIssue :one
SELECT r.id, r.slug, r.title, r.status, r.created_at
FROM sdd_requirements r
JOIN issues us ON us.req_id = r.id
WHERE us.id = $1;

-- name: GetCoverageDashboard :one
SELECT
    COUNT(DISTINCT us.id) AS total_hus,
    COUNT(DISTINCT us.id) FILTER (WHERE p.issue_id IS NOT NULL) AS hus_with_proposal,
    COUNT(DISTINCT us.id) FILTER (WHERE d.issue_id IS NOT NULL) AS hus_with_design,
    COUNT(DISTINCT us.id) FILTER (WHERE t.id IS NOT NULL AND t.status = 'completed') AS hus_with_completed_tasks,
    COUNT(DISTINCT us.id) FILTER (WHERE cr.issue_id IS NOT NULL) AS hus_with_code_refs
FROM issues us
LEFT JOIN LATERAL (SELECT issue_id FROM sdd_proposals WHERE issue_id = us.id LIMIT 1) p ON true
LEFT JOIN LATERAL (SELECT issue_id FROM sdd_designs WHERE issue_id = us.id LIMIT 1) d ON true
LEFT JOIN issue_tasks t ON t.issue_id = us.id
LEFT JOIN LATERAL (SELECT issue_id FROM issue_code_references WHERE issue_id = us.id LIMIT 1) cr ON true;

-- name: GetProgressReport :many
SELECT r.slug, r.title,
    COUNT(DISTINCT us.id) AS total_hus,
    COUNT(DISTINCT us.id) FILTER (WHERE us.status = 'completed') AS completed_hus,
    COUNT(t.id) AS total_tasks,
    COUNT(t.id) FILTER (WHERE t.status = 'completed') AS completed_tasks,
    CASE WHEN COUNT(t.id) > 0
        THEN ROUND(100.0 * COUNT(t.id) FILTER (WHERE t.status = 'completed') / COUNT(t.id), 1)
        ELSE 0
    END::numeric AS task_pct
FROM sdd_requirements r
LEFT JOIN issues us ON us.req_id = r.id
LEFT JOIN issue_tasks t ON t.issue_id = us.id
WHERE r.status = 'active'
GROUP BY r.slug, r.title
ORDER BY task_pct ASC;

-- name: GetHUsWithoutProposals :many
SELECT us.id, us.slug, us.title, COALESCE(r.slug, '')::text AS req_slug
FROM issues us
LEFT JOIN sdd_requirements r ON r.id = us.req_id
LEFT JOIN LATERAL (SELECT 1 AS present FROM sdd_proposals WHERE issue_id = us.id LIMIT 1) p ON true
WHERE p.present IS NULL
ORDER BY us.slug;

-- name: GetHUsWithoutDesigns :many
SELECT us.id, us.slug, us.title, COALESCE(r.slug, '')::text AS req_slug
FROM issues us
LEFT JOIN sdd_requirements r ON r.id = us.req_id
LEFT JOIN LATERAL (SELECT 1 AS present FROM sdd_designs WHERE issue_id = us.id LIMIT 1) d ON true
WHERE d.present IS NULL
ORDER BY us.slug;

-- name: GetHUsWithIncompleteTasks :many
SELECT us.id, us.slug, us.title, COALESCE(r.slug, '')::text AS req_slug
FROM issues us
LEFT JOIN sdd_requirements r ON r.id = us.req_id
WHERE us.id IN (
    SELECT issue_id FROM issue_tasks
    GROUP BY issue_id
    HAVING COUNT(*) FILTER (WHERE status = 'completed') < COUNT(*)
)
ORDER BY us.slug;

-- name: GetConsolidatedReport :many
SELECT
    r.slug, r.title,
    COUNT(DISTINCT us.id) AS total_hus,
    COUNT(DISTINCT us.id) FILTER (WHERE p.issue_id IS NOT NULL) AS hus_with_proposal,
    COUNT(DISTINCT us.id) FILTER (WHERE d.issue_id IS NOT NULL) AS hus_with_design,
    COUNT(DISTINCT us.id) FILTER (WHERE us.status = 'completed') AS completed_hus,
    COUNT(t.id) AS total_tasks,
    COUNT(t.id) FILTER (WHERE t.status = 'completed') AS completed_tasks,
    CASE WHEN COUNT(t.id) > 0
        THEN ROUND(100.0 * COUNT(t.id) FILTER (WHERE t.status = 'completed') / COUNT(t.id), 1)
        ELSE 0
    END::numeric AS task_pct
FROM sdd_requirements r
LEFT JOIN issues us ON us.req_id = r.id
LEFT JOIN LATERAL (SELECT issue_id FROM sdd_proposals WHERE issue_id = us.id LIMIT 1) p ON true
LEFT JOIN LATERAL (SELECT issue_id FROM sdd_designs WHERE issue_id = us.id LIMIT 1) d ON true
LEFT JOIN issue_tasks t ON t.issue_id = us.id
WHERE r.status = 'active'
GROUP BY r.slug, r.title
ORDER BY r.slug;

-- name: AddCodeReference :one
INSERT INTO issue_code_references (issue_id, file_path, repo, branch)
VALUES ($1, $2, $3, $4)
ON CONFLICT (issue_id, file_path) DO NOTHING
RETURNING id, file_path, repo, branch;

-- name: RemoveCodeReference :exec
DELETE FROM issue_code_references WHERE id = $1;
