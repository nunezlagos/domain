-- name: InsertDraft :one
INSERT INTO issue_drafts (created_by, project_id, mode, initial_idea, total_steps, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, created_by, mode, initial_idea, answers,
          current_step, total_steps, status, pending_clarifications,
          preview, target_path, committed_at, expires_at, created_at, updated_at;

-- name: InsertStepLog :exec
INSERT INTO issue_draft_steps_log (issue_draft_id, step_key, question, options, answer)
VALUES ($1, $2, $3, $4, $5);

-- name: UpdateDraftAfterAnswer :one
UPDATE issue_drafts
SET answers = $1, current_step = $2, status = $3, updated_at = now()
WHERE id = $4
RETURNING id, created_by, mode, initial_idea, answers,
          current_step, total_steps, status, pending_clarifications,
          preview, target_path, committed_at, expires_at, created_at, updated_at;

-- name: GetDraft :one
SELECT id, created_by, mode, initial_idea, answers,
       current_step, total_steps, status, pending_clarifications,
       preview, target_path, committed_at, expires_at, created_at, updated_at
FROM issue_drafts WHERE id = $1;

-- name: UpdateDraftPreview :exec
UPDATE issue_drafts SET preview = $1, target_path = $2, updated_at = now()
WHERE id = $3;

-- name: UpdateDraftAnswers :exec
UPDATE issue_drafts SET answers = $1, updated_at = now() WHERE id = $2;

-- name: CommitDraft :one
UPDATE issue_drafts SET status = $1, committed_at = now(), updated_at = now(), issue_id = $3
WHERE id = $2
RETURNING id, created_by, mode, initial_idea, answers,
          current_step, total_steps, status, pending_clarifications,
          preview, target_path, committed_at, expires_at, created_at, updated_at;

-- name: FindRequirementBySlug :one
SELECT id FROM sdd_requirements WHERE slug = $1;

-- name: GetDraftProjectID :one
SELECT project_id FROM issue_drafts WHERE id = $1;

-- name: CountIssuesByReqID :one
SELECT COUNT(*)::int FROM issues WHERE req_id = $1;

-- name: AbandonDraft :exec
UPDATE issue_drafts SET status = $1, updated_at = now() WHERE id = $2;

-- name: ListDrafts :many
SELECT id, created_by, mode, initial_idea, answers,
       current_step, total_steps, status, pending_clarifications,
       preview, target_path, committed_at, expires_at, created_at, updated_at
FROM issue_drafts
WHERE (sqlc.narg('status_filter')::text IS NULL OR status = sqlc.narg('status_filter')::text)
ORDER BY created_at DESC
LIMIT 100;

-- name: MarkDraftStatus :exec
UPDATE issue_drafts SET status = $1, updated_at = now() WHERE id = $2;
