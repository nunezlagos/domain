// Package spec — issue-04.3 proposals and designs management.
//
// Proposals: intention, scope, approach (markdown). Append-only versionado.
// Designs: arch decisions, alternatives, data flow, TDD plan (markdown).
// Ambos vinculados a issues con UNIQUE(issue_id, version).
package spec

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
)

const (
	PropStatusDraft    = "draft"
	PropStatusApproved = "approved"
	PropStatusRejected = "rejected"

	DesignStatusDraft = "draft"
	DesignStatusFinal = "final"
)

var (
	ErrNotFound         = errors.New("not found")
	ErrInvalidStatus    = errors.New("invalid status")
	ErrInvalidTransition = errors.New("invalid status transition")
	ErrHUNotFound       = errors.New("user story not found")
)

var validPropStatuses = map[string]bool{PropStatusDraft: true, PropStatusApproved: true, PropStatusRejected: true}
var validDesignStatuses = map[string]bool{DesignStatusDraft: true, DesignStatusFinal: true}

var allowedPropTransitions = map[string][]string{
	PropStatusDraft:   {PropStatusApproved, PropStatusRejected},
	PropStatusApproved: {},
	PropStatusRejected: {},
}

// Proposal snapshot.
type Proposal struct {
	ID              uuid.UUID  `json:"id"`
	HuID            uuid.UUID  `json:"issue_id"`
	Version         int        `json:"version"`
	Status          string     `json:"status"`
	Intention       string     `json:"intention"`
	Scope           string     `json:"scope"`
	Approach        string     `json:"approach"`
	Risks           *string    `json:"risks,omitempty"`
	TestingNotes    *string    `json:"testing_notes,omitempty"`
	RejectionReason *string    `json:"rejection_reason,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// Design snapshot.
type Design struct {
	ID              uuid.UUID  `json:"id"`
	HuID            uuid.UUID  `json:"issue_id"`
	ProposalID      *uuid.UUID `json:"proposal_id,omitempty"`
	Version         int        `json:"version"`
	Status          string     `json:"status"`
	ArchDecisions   string     `json:"arch_decisions"`
	Alternatives    *string    `json:"alternatives,omitempty"`
	DataFlow        *string    `json:"data_flow,omitempty"`
	TDDPlan         *string    `json:"tdd_plan,omitempty"`
	RisksMitigation *string    `json:"risks_mitigation,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// Service CRUD para proposals + designs.
type Service struct {
	Pool  *pgxpool.Pool
	Audit *audit.PGRecorder
}

// --- Proposals ---

// CreateProposal inserta nueva versión de proposal para una HU.
func (s *Service) CreateProposal(ctx context.Context, issueID uuid.UUID, intention, scope, approach, risks, testingNotes string) (*Proposal, error) {
	if err := s.requireHU(ctx, issueID); err != nil {
		return nil, err
	}

	var version int
	_ = s.Pool.QueryRow(ctx, `SELECT COALESCE(MAX(version), 0) FROM proposals WHERE issue_id = $1`, issueID).Scan(&version)
	version++

	var p Proposal
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO proposals (issue_id, version, status, intention, scope, approach, risks, testing_notes)
		 VALUES ($1, $2, 'draft', $3, $4, $5, $6, $7)
		 RETURNING id, issue_id, version, status, intention, scope, approach, risks, testing_notes, rejection_reason, created_at, updated_at`,
		issueID, version, intention, scope, approach, nullStr(risks), nullStr(testingNotes),
	).Scan(&p.ID, &p.HuID, &p.Version, &p.Status, &p.Intention, &p.Scope, &p.Approach, &p.Risks, &p.TestingNotes, &p.RejectionReason, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert proposal: %w", err)
	}

	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorType:  audit.ActorSystem,
			Action:     "proposal.created",
			EntityType: "proposal",
			EntityID:   &p.ID,
			NewValues:  map[string]any{"issue_id": issueID.String(), "version": version},
		})
	}
	return &p, nil
}

// GetLatestProposal retorna la última versión de proposal para una HU.
func (s *Service) GetLatestProposal(ctx context.Context, issueID uuid.UUID) (*Proposal, error) {
	var p Proposal
	err := s.Pool.QueryRow(ctx,
		`SELECT id, issue_id, version, status, intention, scope, approach, risks, testing_notes, rejection_reason, created_at, updated_at
		 FROM proposals WHERE issue_id = $1 ORDER BY version DESC LIMIT 1`, issueID,
	).Scan(&p.ID, &p.HuID, &p.Version, &p.Status, &p.Intention, &p.Scope, &p.Approach, &p.Risks, &p.TestingNotes, &p.RejectionReason, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get latest proposal: %w", err)
	}
	return &p, nil
}

// GetProposalVersion retorna una versión específica.
func (s *Service) GetProposalVersion(ctx context.Context, issueID uuid.UUID, version int) (*Proposal, error) {
	var p Proposal
	err := s.Pool.QueryRow(ctx,
		`SELECT id, issue_id, version, status, intention, scope, approach, risks, testing_notes, rejection_reason, created_at, updated_at
		 FROM proposals WHERE issue_id = $1 AND version = $2`, issueID, version,
	).Scan(&p.ID, &p.HuID, &p.Version, &p.Status, &p.Intention, &p.Scope, &p.Approach, &p.Risks, &p.TestingNotes, &p.RejectionReason, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get proposal version: %w", err)
	}
	return &p, nil
}

// ListProposalVersions lista todas las versiones de proposal para una HU.
func (s *Service) ListProposalVersions(ctx context.Context, issueID uuid.UUID) ([]Proposal, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, issue_id, version, status, intention, scope, approach, risks, testing_notes, rejection_reason, created_at, updated_at
		 FROM proposals WHERE issue_id = $1 ORDER BY version DESC`, issueID)
	if err != nil {
		return nil, fmt.Errorf("list proposals: %w", err)
	}
	defer rows.Close()
	return scanProposals(rows)
}

// ChangeProposalStatus cambia status con validación de transición.
func (s *Service) ChangeProposalStatus(ctx context.Context, proposalID uuid.UUID, newStatus, rejectionReason string) (*Proposal, error) {
	var current Proposal
	err := s.Pool.QueryRow(ctx,
		`SELECT id, issue_id, version, status, intention, scope, approach, risks, testing_notes, rejection_reason, created_at, updated_at
		 FROM proposals WHERE id = $1`, proposalID,
	).Scan(&current.ID, &current.HuID, &current.Version, &current.Status, &current.Intention, &current.Scope, &current.Approach, &current.Risks, &current.TestingNotes, &current.RejectionReason, &current.CreatedAt, &current.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get proposal: %w", err)
	}

	if !validPropStatuses[newStatus] {
		return nil, ErrInvalidStatus
	}
	allowed, ok := allowedPropTransitions[current.Status]
	if !ok {
		return nil, ErrInvalidTransition
	}
	valid := false
	for _, a := range allowed {
		if a == newStatus {
			valid = true
			break
		}
	}
	if !valid {
		return nil, fmt.Errorf("%w: %s → %s", ErrInvalidTransition, current.Status, newStatus)
	}

	var reason *string
	if newStatus == PropStatusRejected && rejectionReason != "" {
		reason = &rejectionReason
	}

	var updated Proposal
	err = s.Pool.QueryRow(ctx,
		`UPDATE proposals SET status = $2, rejection_reason = $3, updated_at = NOW()
		 WHERE id = $1
		 RETURNING id, issue_id, version, status, intention, scope, approach, risks, testing_notes, rejection_reason, created_at, updated_at`,
		proposalID, newStatus, reason,
	).Scan(&updated.ID, &updated.HuID, &updated.Version, &updated.Status, &updated.Intention, &updated.Scope, &updated.Approach, &updated.Risks, &updated.TestingNotes, &updated.RejectionReason, &updated.CreatedAt, &updated.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update proposal status: %w", err)
	}

	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorType:  audit.ActorSystem,
			Action:     "proposal.status_changed",
			EntityType: "proposal",
			EntityID:   &updated.ID,
			OldValues:  map[string]any{"status": current.Status},
			NewValues:  map[string]any{"status": newStatus},
		})
	}
	return &updated, nil
}

// --- Designs ---

// CreateDesign inserta nuevo design para una HU.
func (s *Service) CreateDesign(ctx context.Context, issueID uuid.UUID, proposalID *uuid.UUID, archDecisions, alternatives, dataFlow, tddPlan, risksMitigation string) (*Design, error) {
	if err := s.requireHU(ctx, issueID); err != nil {
		return nil, err
	}

	var version int
	_ = s.Pool.QueryRow(ctx, `SELECT COALESCE(MAX(version), 0) FROM designs WHERE issue_id = $1`, issueID).Scan(&version)
	version++

	if proposalID != nil && *proposalID == uuid.Nil {
		proposalID = nil
	}

	var d Design
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO designs (issue_id, proposal_id, version, status, arch_decisions, alternatives, data_flow, tdd_plan, risks_mitigation)
		 VALUES ($1, $2, $3, 'draft', $4, $5, $6, $7, $8)
		 RETURNING id, issue_id, proposal_id, version, status, arch_decisions, alternatives, data_flow, tdd_plan, risks_mitigation, created_at, updated_at`,
		issueID, proposalID, version, archDecisions, nullStr(alternatives), nullStr(dataFlow), nullStr(tddPlan), nullStr(risksMitigation),
	).Scan(&d.ID, &d.HuID, &d.ProposalID, &d.Version, &d.Status, &d.ArchDecisions, &d.Alternatives, &d.DataFlow, &d.TDDPlan, &d.RisksMitigation, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert design: %w", err)
	}

	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorType:  audit.ActorSystem,
			Action:     "design.created",
			EntityType: "design",
			EntityID:   &d.ID,
			NewValues:  map[string]any{"issue_id": issueID.String(), "version": version},
		})
	}
	return &d, nil
}

// GetLatestDesign retorna el último design para una HU.
func (s *Service) GetLatestDesign(ctx context.Context, issueID uuid.UUID) (*Design, error) {
	var d Design
	err := s.Pool.QueryRow(ctx,
		`SELECT id, issue_id, proposal_id, version, status, arch_decisions, alternatives, data_flow, tdd_plan, risks_mitigation, created_at, updated_at
		 FROM designs WHERE issue_id = $1 ORDER BY version DESC LIMIT 1`, issueID,
	).Scan(&d.ID, &d.HuID, &d.ProposalID, &d.Version, &d.Status, &d.ArchDecisions, &d.Alternatives, &d.DataFlow, &d.TDDPlan, &d.RisksMitigation, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get latest design: %w", err)
	}
	return &d, nil
}

// ListDesignsByHU lista designs de una HU.
func (s *Service) ListDesignsByHU(ctx context.Context, issueID uuid.UUID) ([]Design, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, issue_id, proposal_id, version, status, arch_decisions, alternatives, data_flow, tdd_plan, risks_mitigation, created_at, updated_at
		 FROM designs WHERE issue_id = $1 ORDER BY version DESC`, issueID)
	if err != nil {
		return nil, fmt.Errorf("list designs: %w", err)
	}
	defer rows.Close()
	return scanDesigns(rows)
}

// ChangeDesignStatus cambia status de un design.
func (s *Service) ChangeDesignStatus(ctx context.Context, designID uuid.UUID, newStatus string) (*Design, error) {
	if !validDesignStatuses[newStatus] {
		return nil, ErrInvalidStatus
	}

	var updated Design
	err := s.Pool.QueryRow(ctx,
		`UPDATE designs SET status = $2, updated_at = NOW()
		 WHERE id = $1
		 RETURNING id, issue_id, proposal_id, version, status, arch_decisions, alternatives, data_flow, tdd_plan, risks_mitigation, created_at, updated_at`,
		designID, newStatus,
	).Scan(&updated.ID, &updated.HuID, &updated.ProposalID, &updated.Version, &updated.Status, &updated.ArchDecisions, &updated.Alternatives, &updated.DataFlow, &updated.TDDPlan, &updated.RisksMitigation, &updated.CreatedAt, &updated.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update design status: %w", err)
	}

	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorType:  audit.ActorSystem,
			Action:     "design.status_changed",
			EntityType: "design",
			EntityID:   &updated.ID,
			OldValues:  map[string]any{"status": "previous"},
			NewValues:  map[string]any{"status": newStatus},
		})
	}
	return &updated, nil
}

// --- helpers ---

func (s *Service) requireHU(ctx context.Context, issueID uuid.UUID) error {
	var exists bool
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM issues WHERE id = $1)`, issueID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check hu: %w", err)
	}
	if !exists {
		return ErrHUNotFound
	}
	return nil
}

func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func scanProposals(rows pgx.Rows) ([]Proposal, error) {
	defer rows.Close()
	var out []Proposal
	for rows.Next() {
		var p Proposal
		if err := rows.Scan(&p.ID, &p.HuID, &p.Version, &p.Status, &p.Intention, &p.Scope, &p.Approach, &p.Risks, &p.TestingNotes, &p.RejectionReason, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan proposal: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func scanDesigns(rows pgx.Rows) ([]Design, error) {
	defer rows.Close()
	var out []Design
	for rows.Next() {
		var d Design
		if err := rows.Scan(&d.ID, &d.HuID, &d.ProposalID, &d.Version, &d.Status, &d.ArchDecisions, &d.Alternatives, &d.DataFlow, &d.TDDPlan, &d.RisksMitigation, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan design: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation
}
