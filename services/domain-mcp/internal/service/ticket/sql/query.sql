-- name: TicketNextNumber :one
SELECT COALESCE(MAX(number), 0) + 1
FROM project_tickets
WHERE project_id = sqlc.arg('project_id');

-- name: InsertTicket :one
INSERT INTO project_tickets
  (project_id, client_id, key, number,
   title, description_md, issue_type, priority,
   assignee_id, reporter_id, labels, parent_id,
   estimated_hours, due_date,
   external_provider, external_id, external_url, external_synced_at)
VALUES (
  sqlc.arg('project_id'),
  sqlc.arg('client_id'),
  sqlc.arg('key'),
  sqlc.arg('number'),
  sqlc.arg('title'),
  NULLIF(sqlc.arg('description_md')::text, ''),
  sqlc.arg('issue_type'),
  sqlc.arg('priority'),
  sqlc.arg('assignee_id'),
  sqlc.arg('reporter_id'),
  sqlc.arg('labels'),
  sqlc.arg('parent_id'),
  sqlc.arg('estimated_hours'),
  sqlc.arg('due_date'),
  NULLIF(sqlc.arg('external_provider')::text, ''),
  NULLIF(sqlc.arg('external_id')::text, ''),
  NULLIF(sqlc.arg('external_url')::text, ''),
  sqlc.arg('external_synced_at')
)
RETURNING id, project_id, client_id, key, number,
  title, COALESCE(description_md, '') AS description_md,
  issue_type, status, priority,
  assignee_id, reporter_id, labels,
  COALESCE(external_provider, '') AS external_provider,
  COALESCE(external_id, '') AS external_id,
  COALESCE(external_url, '') AS external_url,
  external_synced_at,
  parent_id, linked_issue_id, estimated_hours, actual_hours,
  due_date, started_at, completed_at,
  locked_by, locked_until, version,
  created_at, updated_at, deleted_at;

-- name: InsertStatusHistory :exec
INSERT INTO project_ticket_status_history
  (ticket_id, from_status, to_status, changed_by, note)
VALUES (sqlc.arg('ticket_id'), sqlc.arg('from_status'), sqlc.arg('to_status'), sqlc.arg('changed_by'), sqlc.arg('note'));

-- name: GetTicketByID :one
SELECT id, project_id, client_id, key, number,
  title, COALESCE(description_md, '') AS description_md,
  issue_type, status, priority,
  assignee_id, reporter_id, labels,
  COALESCE(external_provider, '') AS external_provider,
  COALESCE(external_id, '') AS external_id,
  COALESCE(external_url, '') AS external_url,
  external_synced_at,
  parent_id, linked_issue_id, estimated_hours, actual_hours,
  due_date, started_at, completed_at,
  locked_by, locked_until, version,
  created_at, updated_at, deleted_at
FROM project_tickets
WHERE id = sqlc.arg('id') AND deleted_at IS NULL;

-- name: GetTicketByKey :one
SELECT id, project_id, client_id, key, number,
  title, COALESCE(description_md, '') AS description_md,
  issue_type, status, priority,
  assignee_id, reporter_id, labels,
  COALESCE(external_provider, '') AS external_provider,
  COALESCE(external_id, '') AS external_id,
  COALESCE(external_url, '') AS external_url,
  external_synced_at,
  parent_id, linked_issue_id, estimated_hours, actual_hours,
  due_date, started_at, completed_at,
  locked_by, locked_until, version,
  created_at, updated_at, deleted_at
FROM project_tickets
WHERE project_id = sqlc.arg('project_id')
  AND key = sqlc.arg('key')
  AND deleted_at IS NULL;

-- name: GetTicketIDByKey :one
SELECT id FROM project_tickets
WHERE project_id = sqlc.arg('project_id')
  AND key = sqlc.arg('key')
  AND deleted_at IS NULL;

-- name: SoftDeleteTicket :execrows
UPDATE project_tickets
SET deleted_at = NOW()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL;

-- name: LinkTicketIssue :one
UPDATE project_tickets
SET linked_issue_id = sqlc.arg('linked_issue_id')
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING id, project_id, client_id, key, number,
  title, COALESCE(description_md, '') AS description_md,
  issue_type, status, priority,
  assignee_id, reporter_id, labels,
  COALESCE(external_provider, '') AS external_provider,
  COALESCE(external_id, '') AS external_id,
  COALESCE(external_url, '') AS external_url,
  external_synced_at,
  parent_id, linked_issue_id, estimated_hours, actual_hours,
  due_date, started_at, completed_at,
  locked_by, locked_until, version,
  created_at, updated_at, deleted_at;

-- name: LinkTicketExternal :one
UPDATE project_tickets
SET external_provider    = NULLIF(sqlc.arg('external_provider')::text, ''),
    external_id          = NULLIF(sqlc.arg('external_id')::text, ''),
    external_url         = NULLIF(sqlc.arg('external_url')::text, ''),
    external_synced_at   = NOW()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING id, project_id, client_id, key, number,
  title, COALESCE(description_md, '') AS description_md,
  issue_type, status, priority,
  assignee_id, reporter_id, labels,
  COALESCE(external_provider, '') AS external_provider,
  COALESCE(external_id, '') AS external_id,
  COALESCE(external_url, '') AS external_url,
  external_synced_at,
  parent_id, linked_issue_id, estimated_hours, actual_hours,
  due_date, started_at, completed_at,
  locked_by, locked_until, version,
  created_at, updated_at, deleted_at;

-- name: UnlinkTicketExternal :execrows
UPDATE project_tickets
SET external_provider = NULL, external_id = NULL,
    external_url = NULL, external_synced_at = NULL
WHERE id = sqlc.arg('id') AND deleted_at IS NULL;

-- name: FindTicketByExternal :one
SELECT id, project_id, client_id, key, number,
  title, COALESCE(description_md, '') AS description_md,
  issue_type, status, priority,
  assignee_id, reporter_id, labels,
  COALESCE(external_provider, '') AS external_provider,
  COALESCE(external_id, '') AS external_id,
  COALESCE(external_url, '') AS external_url,
  external_synced_at,
  parent_id, linked_issue_id, estimated_hours, actual_hours,
  due_date, started_at, completed_at,
  locked_by, locked_until, version,
  created_at, updated_at, deleted_at
FROM project_tickets
WHERE external_provider = sqlc.arg('external_provider')
  AND external_id = sqlc.arg('external_id')
  AND deleted_at IS NULL
LIMIT 1;

-- name: ClaimTicket :one
UPDATE project_tickets
SET locked_by    = sqlc.arg('locked_by'),
    locked_until = NOW() + (sqlc.arg('ttl_minutes')::int * INTERVAL '1 minute'),
    version      = version + 1
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING id, project_id, client_id, key, number,
  title, COALESCE(description_md, '') AS description_md,
  issue_type, status, priority,
  assignee_id, reporter_id, labels,
  COALESCE(external_provider, '') AS external_provider,
  COALESCE(external_id, '') AS external_id,
  COALESCE(external_url, '') AS external_url,
  external_synced_at,
  parent_id, linked_issue_id, estimated_hours, actual_hours,
  due_date, started_at, completed_at,
  locked_by, locked_until, version,
  created_at, updated_at, deleted_at;

-- name: ReleaseTicket :one
UPDATE project_tickets
SET locked_by    = NULL,
    locked_until = NULL,
    version      = version + 1
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING id, project_id, client_id, key, number,
  title, COALESCE(description_md, '') AS description_md,
  issue_type, status, priority,
  assignee_id, reporter_id, labels,
  COALESCE(external_provider, '') AS external_provider,
  COALESCE(external_id, '') AS external_id,
  COALESCE(external_url, '') AS external_url,
  external_synced_at,
  parent_id, linked_issue_id, estimated_hours, actual_hours,
  due_date, started_at, completed_at,
  locked_by, locked_until, version,
  created_at, updated_at, deleted_at;

-- name: BulkLinkExternalByID :execrows
UPDATE project_tickets
SET external_provider    = sqlc.arg('external_provider'),
    external_id          = NULLIF(sqlc.arg('external_id')::text, ''),
    external_url         = NULLIF(sqlc.arg('external_url')::text, ''),
    external_synced_at   = NOW()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL;

-- name: InsertComment :one
INSERT INTO project_ticket_comments (ticket_id, author_id, body_md)
VALUES (sqlc.arg('ticket_id'), sqlc.arg('author_id'), sqlc.arg('body_md'))
RETURNING id, ticket_id, author_id, body_md,
  COALESCE(external_id, '') AS external_id,
  created_at, updated_at, deleted_at;

-- name: ListComments :many
SELECT id, ticket_id, author_id, body_md,
  COALESCE(external_id, '') AS external_id,
  created_at, updated_at, deleted_at
FROM project_ticket_comments
WHERE ticket_id = sqlc.arg('ticket_id') AND deleted_at IS NULL
ORDER BY created_at ASC;

-- name: ListStatusHistory :many
SELECT id, ticket_id,
  COALESCE(from_status, '') AS from_status,
  to_status, changed_by,
  COALESCE(note, '') AS note,
  changed_at
FROM project_ticket_status_history
WHERE ticket_id = sqlc.arg('ticket_id')
ORDER BY changed_at ASC;
