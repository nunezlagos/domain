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
	"nunezlagos/domain/internal/service/spec/specdb"
	"nunezlagos/domain/internal/store/txctx"
)

const (
	PropStatusDraft    = "draft"
	PropStatusApproved = "approved"
	PropStatusRejected = "rejected"

	DesignStatusDraft = "draft"
	DesignStatusFinal = "final"
)

var (
	ErrNotFound          = errors.New("not found")
	ErrInvalidStatus     = errors.New("invalid status")
	ErrInvalidTransition = errors.New("invalid status transition")
	ErrHUNotFound        = errors.New("user story not found")
)

var validPropStatuses = map[string]bool{PropStatusDraft: true, PropStatusApproved: true, PropStatusRejected: true}
var validDesignStatuses = map[string]bool{DesignStatusDraft: true, DesignStatusFinal: true}

var allowedPropTransitions = map[string][]string{
	PropStatusDraft:    {PropStatusApproved, PropStatusRejected},
	PropStatusApproved: {},
	PropStatusRejected: {},
}

// Proposal snapshot.
type Proposal struct {
	ID              uuid.UUID `json:"id"`
	HuID            uuid.UUID `json:"issue_id"`
	Version         int       `json:"version"`
	Status          string    `json:"status"`
	Intention       string    `json:"intention"`
	Scope           string    `json:"scope"`
	Approach        string    `json:"approach"`
	Risks           *string   `json:"risks,omitempty"`
	TestingNotes    *string   `json:"testing_notes,omitempty"`
	RejectionReason *string   `json:"rejection_reason,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
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

func (s *Service) q(ctx context.Context) *specdb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return specdb.New(tx)
	}
	return specdb.New(s.Pool)
}

// CreateProposal inserta nueva versión de proposal para una HU.
func (s *Service) CreateProposal(ctx context.Context, issueID uuid.UUID, intention, scope, approach, risks, testingNotes string) (*Proposal, error) {
	if err := s.requireHU(ctx, issueID); err != nil {
		return nil, err
	}

	// MAX(version)+1 e INSERT no son atómicos: dos usuarios concurrentes
	// calculan la misma versión y el UNIQUE(issue_id, version) rechaza al
	// segundo. Retry con versión recalculada en vez de propagar el error crudo.
	var row specdb.SddProposal
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		maxV, _ := s.q(ctx).MaxProposalVersion(ctx, issueID)
		version := maxV + 1
		row, err = s.q(ctx).InsertProposal(ctx, specdb.InsertProposalParams{
			IssueID:      issueID,
			Version:      version,
			Intention:    intention,
			Scope:        scope,
			Approach:     approach,
			Risks:        nullStr(risks),
			TestingNotes: nullStr(testingNotes),
		})
		if err == nil || !isUniqueViolation(err) {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("insert proposal: %w", err)
	}
	p := toProposal(row)

	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorType:  audit.ActorSystem,
			Action:     "proposal.created",
			EntityType: "proposal",
			EntityID:   &p.ID,
			NewValues:  map[string]any{"issue_id": issueID.String(), "version": p.Version},
		})
	}
	return &p, nil
}

// GetLatestProposal retorna la última versión de proposal para una HU.
func (s *Service) GetLatestProposal(ctx context.Context, issueID uuid.UUID) (*Proposal, error) {
	row, err := s.q(ctx).GetLatestProposal(ctx, issueID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get latest proposal: %w", err)
	}
	p := toProposal(row)
	return &p, nil
}

// GetProposalVersion retorna una versión específica.
func (s *Service) GetProposalVersion(ctx context.Context, issueID uuid.UUID, version int) (*Proposal, error) {
	row, err := s.q(ctx).GetProposalVersion(ctx, specdb.GetProposalVersionParams{
		IssueID: issueID,
		Version: int32(version),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get proposal version: %w", err)
	}
	p := toProposal(row)
	return &p, nil
}

// ListProposalVersions lista todas las versiones de proposal para una HU.
func (s *Service) ListProposalVersions(ctx context.Context, issueID uuid.UUID) ([]Proposal, error) {
	rows, err := s.q(ctx).ListProposalVersions(ctx, issueID)
	if err != nil {
		return nil, fmt.Errorf("list proposals: %w", err)
	}
	out := make([]Proposal, len(rows))
	for i, r := range rows {
		out[i] = toProposal(r)
	}
	return out, nil
}

// ChangeProposalStatus cambia status con validación de transición.
func (s *Service) ChangeProposalStatus(ctx context.Context, proposalID uuid.UUID, newStatus, rejectionReason string) (*Proposal, error) {
	current, err := s.q(ctx).GetProposalByID(ctx, proposalID)
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

	row, err := s.q(ctx).UpdateProposalStatus(ctx, specdb.UpdateProposalStatusParams{
		ID:              proposalID,
		Status:          newStatus,
		RejectionReason: reason,
		ExpectedStatus:  current.Status,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// Otra transacción cambió el status entre la lectura y este UPDATE
		// (guard optimista): reportar como transición inválida, no pisar.
		return nil, fmt.Errorf("%w: el proposal cambió de estado concurrentemente (leído %s)", ErrInvalidTransition, current.Status)
	}
	if err != nil {
		return nil, fmt.Errorf("update proposal status: %w", err)
	}
	updated := toProposal(row)

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

// CreateDesign inserta nuevo design para una HU.
func (s *Service) CreateDesign(ctx context.Context, issueID uuid.UUID, proposalID *uuid.UUID, archDecisions, alternatives, dataFlow, tddPlan, risksMitigation string) (*Design, error) {
	if err := s.requireHU(ctx, issueID); err != nil {
		return nil, err
	}

	if proposalID != nil && *proposalID == uuid.Nil {
		proposalID = nil
	}

	// Retry ante carrera de versionado concurrente (ver CreateProposal).
	var row specdb.SddDesign
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		maxV, _ := s.q(ctx).MaxDesignVersion(ctx, issueID)
		version := maxV + 1
		row, err = s.q(ctx).InsertDesign(ctx, specdb.InsertDesignParams{
			IssueID:         issueID,
			ProposalID:      proposalID,
			Version:         version,
			ArchDecisions:   archDecisions,
			Alternatives:    nullStr(alternatives),
			DataFlow:        nullStr(dataFlow),
			TddPlan:         nullStr(tddPlan),
			RisksMitigation: nullStr(risksMitigation),
		})
		if err == nil || !isUniqueViolation(err) {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("insert design: %w", err)
	}
	d := toDesign(row)

	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorType:  audit.ActorSystem,
			Action:     "design.created",
			EntityType: "design",
			EntityID:   &d.ID,
			NewValues:  map[string]any{"issue_id": issueID.String(), "version": d.Version},
		})
	}
	return &d, nil
}

// GetLatestDesign retorna el último design para una HU.
func (s *Service) GetLatestDesign(ctx context.Context, issueID uuid.UUID) (*Design, error) {
	row, err := s.q(ctx).GetLatestDesign(ctx, issueID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get latest design: %w", err)
	}
	d := toDesign(row)
	return &d, nil
}

// ListDesignsByHU lista designs de una HU.
func (s *Service) ListDesignsByHU(ctx context.Context, issueID uuid.UUID) ([]Design, error) {
	rows, err := s.q(ctx).ListDesignsByIssue(ctx, issueID)
	if err != nil {
		return nil, fmt.Errorf("list designs: %w", err)
	}
	out := make([]Design, len(rows))
	for i, r := range rows {
		out[i] = toDesign(r)
	}
	return out, nil
}

// ChangeDesignStatus cambia status de un design.
func (s *Service) ChangeDesignStatus(ctx context.Context, designID uuid.UUID, newStatus string) (*Design, error) {
	if !validDesignStatuses[newStatus] {
		return nil, ErrInvalidStatus
	}

	current, err := s.q(ctx).GetDesignByID(ctx, designID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get design: %w", err)
	}

	row, err := s.q(ctx).UpdateDesignStatus(ctx, specdb.UpdateDesignStatusParams{
		ID:             designID,
		Status:         newStatus,
		ExpectedStatus: current.Status,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// Otra transacción cambió el status entre la lectura y este UPDATE
		// (guard optimista): reportar en vez de pisar.
		return nil, fmt.Errorf("%w: el design cambió de estado concurrentemente (leído %s)", ErrInvalidTransition, current.Status)
	}
	if err != nil {
		return nil, fmt.Errorf("update design status: %w", err)
	}
	updated := toDesign(row)

	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorType:  audit.ActorSystem,
			Action:     "design.status_changed",
			EntityType: "design",
			EntityID:   &updated.ID,
			OldValues:  map[string]any{"status": current.Status},
			NewValues:  map[string]any{"status": newStatus},
		})
	}
	return &updated, nil
}

func (s *Service) requireHU(ctx context.Context, issueID uuid.UUID) error {
	exists, err := s.q(ctx).IssueExists(ctx, issueID)
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

func toProposal(r specdb.SddProposal) Proposal {
	return Proposal{
		ID:              r.ID,
		HuID:            r.IssueID,
		Version:         int(r.Version),
		Status:          r.Status,
		Intention:       r.Intention,
		Scope:           r.Scope,
		Approach:        r.Approach,
		Risks:           r.Risks,
		TestingNotes:    r.TestingNotes,
		RejectionReason: r.RejectionReason,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
}

func toDesign(r specdb.SddDesign) Design {
	return Design{
		ID:              r.ID,
		HuID:            r.IssueID,
		ProposalID:      r.ProposalID,
		Version:         int(r.Version),
		Status:          r.Status,
		ArchDecisions:   r.ArchDecisions,
		Alternatives:    r.Alternatives,
		DataFlow:        r.DataFlow,
		TDDPlan:         r.TddPlan,
		RisksMitigation: r.RisksMitigation,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
}

// isUniqueViolation detecta la violación de UNIQUE(issue_id, version) que
// produce la carrera MAX(version)+INSERT entre dos usuarios concurrentes.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation
}
