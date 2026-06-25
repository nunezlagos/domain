-- name: GetIssueProjectID :one
SELECT project_id FROM issues WHERE id = $1;

-- name: IssueExists :one
SELECT EXISTS(SELECT 1 FROM issues WHERE id = $1);

-- name: MaxTaskPosition :one
SELECT COALESCE(MAX(position), 0)::int FROM issue_tasks WHERE issue_id = $1 AND section = $2;

-- name: InsertTask :one
INSERT INTO issue_tasks (issue_id, project_id, section, description, position, status)
VALUES ($1, $2, $3, $4, $5, 'pending')
RETURNING id, issue_id, section, description, status, position, started_at, completed_at, completed_by, created_at, updated_at, project_id;

-- name: ListTasksByIssue :many
SELECT id, issue_id, section, description, status, position, started_at, completed_at, completed_by, created_at, updated_at, project_id
FROM issue_tasks WHERE issue_id = $1 ORDER BY section, position;

-- name: GetTaskByID :one
SELECT id, issue_id, section, description, status, position, started_at, completed_at, completed_by, created_at, updated_at, project_id
FROM issue_tasks WHERE id = $1;

-- name: GetTaskStatus :one
SELECT status FROM issue_tasks WHERE id = $1;

-- name: TaskExists :one
SELECT EXISTS(SELECT 1 FROM issue_tasks WHERE id = $1);

-- name: UpdateTaskStatus :one
UPDATE issue_tasks
SET status = $2,
    started_at = COALESCE(sqlc.narg('started_at')::timestamptz, started_at),
    completed_at = sqlc.narg('completed_at')::timestamptz,
    completed_by = COALESCE(sqlc.narg('completed_by')::varchar, completed_by),
    updated_at = NOW()
WHERE id = $1
RETURNING id, issue_id, section, description, status, position, started_at, completed_at, completed_by, created_at, updated_at, project_id;

-- name: GetProgress :one
SELECT $1::uuid AS issue_id,
       COUNT(*) AS total,
       COUNT(*) FILTER (WHERE status = 'completed') AS completed,
       ROUND(100.0 * COUNT(*) FILTER (WHERE status = 'completed') / GREATEST(COUNT(*), 1), 1)::float8 AS pct
FROM issue_tasks WHERE issue_id = $1;

-- name: InsertVerification :one
INSERT INTO tdd_verification_results (task_id, result, evidence, notes, verified_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, task_id, result, evidence, notes, verified_at, verified_by;

-- name: GetLatestVerification :one
SELECT id, task_id, result, evidence, notes, verified_at, verified_by
FROM tdd_verification_results WHERE task_id = $1 ORDER BY verified_at DESC LIMIT 1;

-- name: InsertSabotage :one
INSERT INTO tdd_sabotage_records (task_id, action, expected_failure, actual_result, restored)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, task_id, action, expected_failure, actual_result, restored, performed_at;

-- name: ListSabotagesByTask :many
SELECT id, task_id, action, expected_failure, actual_result, restored, performed_at
FROM tdd_sabotage_records WHERE task_id = $1 ORDER BY performed_at;
