-- name: InsertIntake :one
INSERT INTO issue_intake_payloads (
    source, source_ref, submitted_by, raw_text, raw_payload, project_id
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING id, source, source_ref, submitted_by, raw_text, raw_payload,
          status, classified_type, classified_severity, classified_confidence,
          classification_reasoning, needs_clarification, proposed_title,
          proposed_description, proposed_req_slug, proposed_hu_draft,
          dedup_candidates, merge_action, reviewer_id, reviewed_at,
          rejection_reason, committed_req_id, committed_issue_id, failure_reason,
          created_at, updated_at;

-- name: GetIntake :one
SELECT id, source, source_ref, submitted_by, raw_text, raw_payload,
       status, classified_type, classified_severity, classified_confidence,
       classification_reasoning, needs_clarification, proposed_title,
       proposed_description, proposed_req_slug, proposed_hu_draft,
       dedup_candidates, merge_action, reviewer_id, reviewed_at,
       rejection_reason, committed_req_id, committed_issue_id, failure_reason,
       created_at, updated_at
FROM issue_intake_payloads WHERE id = $1;

-- name: UpdateClassification :exec
UPDATE issue_intake_payloads
SET classified_type = $1, classified_severity = $2, classified_confidence = $3,
    classification_reasoning = $4, needs_clarification = $5,
    status = $6, updated_at = now()
WHERE id = $7;

-- name: MarkPendingReview :exec
UPDATE issue_intake_payloads
SET proposed_title = $1, proposed_description = $2, proposed_req_slug = $3,
    proposed_hu_draft = $4, dedup_candidates = $5, merge_action = $6,
    status = $7, updated_at = now()
WHERE id = $8;

-- name: ApproveIntake :exec
UPDATE issue_intake_payloads
SET status = $1, reviewer_id = $2, reviewed_at = now(), updated_at = now()
WHERE id = $3;

-- name: RejectIntake :exec
UPDATE issue_intake_payloads
SET status = $1, reviewer_id = $2, reviewed_at = now(),
    rejection_reason = $3, updated_at = now()
WHERE id = $4;

-- name: LinkCommitted :exec
UPDATE issue_intake_payloads
SET status = $1, committed_req_id = $2, committed_issue_id = $3, updated_at = now()
WHERE id = $4;

-- name: ListPendingIntakes :many
SELECT id, source, source_ref, submitted_by, raw_text, raw_payload,
       status, classified_type, classified_severity, classified_confidence,
       classification_reasoning, needs_clarification, proposed_title,
       proposed_description, proposed_req_slug, proposed_hu_draft,
       dedup_candidates, merge_action, reviewer_id, reviewed_at,
       rejection_reason, committed_req_id, committed_issue_id, failure_reason,
       created_at, updated_at
FROM issue_intake_payloads
WHERE status NOT IN ('committed', 'rejected', 'failed')
ORDER BY created_at ASC LIMIT $1;
