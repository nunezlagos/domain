-- name: MaxProposalVersion :one
SELECT COALESCE(MAX(version), 0)::int FROM sdd_proposals WHERE issue_id = $1;

-- name: InsertProposal :one
INSERT INTO sdd_proposals (issue_id, version, status, intention, scope, approach, risks, testing_notes)
VALUES ($1, $2, 'draft', $3, $4, $5, $6, $7)
RETURNING id, issue_id, version, status, intention, scope, approach, risks, testing_notes, rejection_reason, created_at, updated_at;

-- name: GetLatestProposal :one
SELECT id, issue_id, version, status, intention, scope, approach, risks, testing_notes, rejection_reason, created_at, updated_at
FROM sdd_proposals WHERE issue_id = $1 ORDER BY version DESC LIMIT 1;

-- name: GetProposalVersion :one
SELECT id, issue_id, version, status, intention, scope, approach, risks, testing_notes, rejection_reason, created_at, updated_at
FROM sdd_proposals WHERE issue_id = $1 AND version = $2;

-- name: ListProposalVersions :many
SELECT id, issue_id, version, status, intention, scope, approach, risks, testing_notes, rejection_reason, created_at, updated_at
FROM sdd_proposals WHERE issue_id = $1 ORDER BY version DESC;

-- name: GetProposalByID :one
SELECT id, issue_id, version, status, intention, scope, approach, risks, testing_notes, rejection_reason, created_at, updated_at
FROM sdd_proposals WHERE id = $1;

-- name: UpdateProposalStatus :one
UPDATE sdd_proposals SET status = $2, rejection_reason = $3, updated_at = NOW()
WHERE id = $1
RETURNING id, issue_id, version, status, intention, scope, approach, risks, testing_notes, rejection_reason, created_at, updated_at;

-- name: MaxDesignVersion :one
SELECT COALESCE(MAX(version), 0)::int FROM sdd_designs WHERE issue_id = $1;

-- name: InsertDesign :one
INSERT INTO sdd_designs (issue_id, proposal_id, version, status, arch_decisions, alternatives, data_flow, tdd_plan, risks_mitigation)
VALUES ($1, $2, $3, 'draft', $4, $5, $6, $7, $8)
RETURNING id, issue_id, proposal_id, version, status, arch_decisions, alternatives, data_flow, tdd_plan, risks_mitigation, created_at, updated_at;

-- name: GetLatestDesign :one
SELECT id, issue_id, proposal_id, version, status, arch_decisions, alternatives, data_flow, tdd_plan, risks_mitigation, created_at, updated_at
FROM sdd_designs WHERE issue_id = $1 ORDER BY version DESC LIMIT 1;

-- name: ListDesignsByIssue :many
SELECT id, issue_id, proposal_id, version, status, arch_decisions, alternatives, data_flow, tdd_plan, risks_mitigation, created_at, updated_at
FROM sdd_designs WHERE issue_id = $1 ORDER BY version DESC;

-- name: UpdateDesignStatus :one
UPDATE sdd_designs SET status = $2, updated_at = NOW()
WHERE id = $1
RETURNING id, issue_id, proposal_id, version, status, arch_decisions, alternatives, data_flow, tdd_plan, risks_mitigation, created_at, updated_at;

-- name: IssueExists :one
SELECT EXISTS(SELECT 1 FROM issues WHERE id = $1);
