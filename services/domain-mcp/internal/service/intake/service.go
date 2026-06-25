// Package intake — issue-04.8 unified requirement intake pipeline.
//
// MVP scope: Submit + transitions + Approve/Reject/Commit. LLM classification,
// embedding dedup y external sync quedan como futuras extensiones del pipeline
// (no incluidas en este commit inicial). El estado y los hooks de transición
// están listos para que un worker async procese los pasos pendientes.
//
//go:generate go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
package intake

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/service/intake/intakedb"
	"nunezlagos/domain/internal/store/txctx"
)

var (
	ErrNotFound      = errors.New("intake not found")
	ErrInvalidStatus = errors.New("invalid status for operation")
	ErrInvalidSource = errors.New("invalid source")

	ErrProjectIDRequired = errors.New("project_id required")
)

const (
	SourceAgent   = "agent"
	SourceEmail   = "email"
	SourceWebhook = "webhook"
	SourceSlack   = "slack"
	SourceSheet   = "sheet"
	SourceManual  = "manual"

	StatusReceived      = "received"
	StatusClassifying   = "classifying"
	StatusClassified    = "classified"
	StatusDeduping      = "deduping"
	StatusStructuring   = "structuring"
	StatusPendingReview = "pending_review"
	StatusApproved      = "approved"
	StatusRejected      = "rejected"
	StatusCommitted     = "committed"
	StatusFailed        = "failed"

	MergeActionCreateNew    = "create_new"
	MergeActionAppendToHU   = "append_to_hu"
	MergeActionMergeWithREQ = "merge_with_req"
)

type Payload struct {
	ID                   uuid.UUID       `json:"id"`
	Source               string          `json:"source"`
	SourceRef            *string         `json:"source_ref,omitempty"`
	OrganizationID       *uuid.UUID      `json:"organization_id,omitempty"`
	SubmittedBy          *string         `json:"submitted_by,omitempty"`
	RawText              string          `json:"raw_text"`
	RawPayload           json.RawMessage `json:"raw_payload"`
	Status               string          `json:"status"`
	ClassifiedType       *string         `json:"classified_type,omitempty"`
	ClassifiedSeverity   *string         `json:"classified_severity,omitempty"`
	ClassifiedConfidence *float64        `json:"classified_confidence,omitempty"`
	ClassificationReason *string         `json:"classification_reasoning,omitempty"`
	NeedsClarification   bool            `json:"needs_clarification"`
	ProposedTitle        *string         `json:"proposed_title,omitempty"`
	ProposedDescription  *string         `json:"proposed_description,omitempty"`
	ProposedReqSlug      *string         `json:"proposed_req_slug,omitempty"`
	ProposedHUDraft      json.RawMessage `json:"proposed_hu_draft,omitempty"`
	DedupCandidates      json.RawMessage `json:"dedup_candidates"`
	MergeAction          *string         `json:"merge_action,omitempty"`
	ReviewerID           *uuid.UUID      `json:"reviewer_id,omitempty"`
	ReviewedAt           *time.Time      `json:"reviewed_at,omitempty"`
	RejectionReason      *string         `json:"rejection_reason,omitempty"`
	CommittedREQ         *uuid.UUID      `json:"committed_req_id,omitempty"`
	CommittedHU          *uuid.UUID      `json:"committed_issue_id,omitempty"`
	FailureReason        *string         `json:"failure_reason,omitempty"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
}

type SubmitInput struct {
	Source         string
	SourceRef      string
	OrganizationID *uuid.UUID
	ProjectID      *uuid.UUID
	SubmittedBy    string
	RawText        string
	RawPayload     map[string]any
}

type Service struct {
	Pool  *pgxpool.Pool
	Audit *audit.PGRecorder
}

var validSources = map[string]bool{
	SourceAgent: true, SourceEmail: true, SourceWebhook: true,
	SourceSlack: true, SourceSheet: true, SourceManual: true,
}

func (s *Service) q(ctx context.Context) *intakedb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return intakedb.New(tx)
	}
	return intakedb.New(s.Pool)
}

func (s *Service) Submit(ctx context.Context, in SubmitInput) (*Payload, error) {
	if !validSources[in.Source] {
		return nil, fmt.Errorf("%w: %s", ErrInvalidSource, in.Source)
	}
	if strings.TrimSpace(in.RawText) == "" {
		return nil, fmt.Errorf("raw_text required")
	}

	if in.ProjectID == nil || *in.ProjectID == uuid.Nil {
		return nil, ErrProjectIDRequired
	}
	if in.RawPayload == nil {
		in.RawPayload = map[string]any{}
	}
	payloadJSON, _ := json.Marshal(in.RawPayload)

	var srcRef *string
	if in.SourceRef != "" {
		srcRef = &in.SourceRef
	}
	var subBy *string
	if in.SubmittedBy != "" {
		subBy = &in.SubmittedBy
	}

	row, err := s.q(ctx).InsertIntake(ctx, intakedb.InsertIntakeParams{
		Source:      in.Source,
		SourceRef:   srcRef,
		SubmittedBy: subBy,
		RawText:     in.RawText,
		RawPayload:  payloadJSON,
		ProjectID:   *in.ProjectID,
	})
	if err != nil {
		return nil, fmt.Errorf("insert intake: %w", err)
	}

	p := toPayload(intakedb.GetIntakeRow(row))

	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorType:  audit.ActorSystem,
			Action:     "intake.submitted",
			EntityType: "intake_payload",
			EntityID:   &p.ID,
			NewValues:  map[string]any{"source": in.Source},
		})
	}
	return &p, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Payload, error) {
	row, err := s.q(ctx).GetIntake(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get intake: %w", err)
	}
	p := toPayload(row)
	return &p, nil
}

func (s *Service) UpdateClassification(ctx context.Context, id uuid.UUID, type_, severity string, confidence float64, reasoning string) (*Payload, error) {
	err := s.q(ctx).UpdateClassification(ctx, intakedb.UpdateClassificationParams{
		ClassifiedType:          &type_,
		ClassifiedSeverity:      &severity,
		ClassifiedConfidence:    float64ToNumeric(confidence),
		ClassificationReasoning: &reasoning,
		NeedsClarification:      confidence < 0.6,
		Status:                  StatusClassified,
		ID:                      id,
	})
	if err != nil {
		return nil, fmt.Errorf("update classification: %w", err)
	}
	return s.Get(ctx, id)
}

func (s *Service) MarkPendingReview(ctx context.Context, id uuid.UUID, title, description, reqSlug string, issueDraft map[string]any, dedup []any, mergeAction string) (*Payload, error) {
	huJSON, _ := json.Marshal(issueDraft)
	dedupJSON, _ := json.Marshal(dedup)

	err := s.q(ctx).MarkPendingReview(ctx, intakedb.MarkPendingReviewParams{
		ProposedTitle:       &title,
		ProposedDescription: &description,
		ProposedReqSlug:     &reqSlug,
		ProposedHuDraft:     huJSON,
		DedupCandidates:     dedupJSON,
		MergeAction:         &mergeAction,
		Status:              StatusPendingReview,
		ID:                  id,
	})
	if err != nil {
		return nil, fmt.Errorf("mark pending review: %w", err)
	}
	return s.Get(ctx, id)
}

func (s *Service) Approve(ctx context.Context, id uuid.UUID, reviewerID uuid.UUID) (*Payload, error) {
	p, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.Status != StatusPendingReview {
		return nil, fmt.Errorf("%w: status=%s", ErrInvalidStatus, p.Status)
	}
	err = s.q(ctx).ApproveIntake(ctx, intakedb.ApproveIntakeParams{
		Status:     StatusApproved,
		ReviewerID: &reviewerID,
		ID:         id,
	})
	if err != nil {
		return nil, fmt.Errorf("approve: %w", err)
	}
	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorType:  audit.ActorUser,
			Action:     "intake.approved",
			EntityType: "intake_payload",
			EntityID:   &id,
			ActorID:    &reviewerID,
		})
	}
	return s.Get(ctx, id)
}

func (s *Service) Reject(ctx context.Context, id, reviewerID uuid.UUID, reason string) (*Payload, error) {
	p, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.Status == StatusCommitted {
		return nil, fmt.Errorf("%w: cannot reject committed", ErrInvalidStatus)
	}
	err = s.q(ctx).RejectIntake(ctx, intakedb.RejectIntakeParams{
		Status:          StatusRejected,
		ReviewerID:      &reviewerID,
		RejectionReason: &reason,
		ID:              id,
	})
	if err != nil {
		return nil, fmt.Errorf("reject: %w", err)
	}
	return s.Get(ctx, id)
}

func (s *Service) LinkCommitted(ctx context.Context, id uuid.UUID, reqID, issueID *uuid.UUID) (*Payload, error) {
	p, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.Status != StatusApproved {
		return nil, fmt.Errorf("%w: must be approved, got %s", ErrInvalidStatus, p.Status)
	}
	err = s.q(ctx).LinkCommitted(ctx, intakedb.LinkCommittedParams{
		Status:           StatusCommitted,
		CommittedReqID:   reqID,
		CommittedIssueID: issueID,
		ID:               id,
	})
	if err != nil {
		return nil, fmt.Errorf("link committed: %w", err)
	}
	return s.Get(ctx, id)
}

func (s *Service) ListPending(ctx context.Context, limit int) ([]Payload, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	rows, err := s.q(ctx).ListPendingIntakes(ctx, int32(limit))
	if err != nil {
		return nil, fmt.Errorf("list pending: %w", err)
	}

	out := make([]Payload, 0, len(rows))
	for _, r := range rows {
		out = append(out, toPayload(intakedb.GetIntakeRow(r)))
	}
	return out, nil
}

func toPayload(r intakedb.GetIntakeRow) Payload {
	return Payload{
		ID:                   r.ID,
		Source:               r.Source,
		SourceRef:            r.SourceRef,
		SubmittedBy:          r.SubmittedBy,
		RawText:              r.RawText,
		RawPayload:           json.RawMessage(r.RawPayload),
		Status:               r.Status,
		ClassifiedType:       r.ClassifiedType,
		ClassifiedSeverity:   r.ClassifiedSeverity,
		ClassifiedConfidence: numericToFloat64Ptr(r.ClassifiedConfidence),
		ClassificationReason: r.ClassificationReasoning,
		NeedsClarification:   r.NeedsClarification,
		ProposedTitle:        r.ProposedTitle,
		ProposedDescription:  r.ProposedDescription,
		ProposedReqSlug:      r.ProposedReqSlug,
		ProposedHUDraft:      json.RawMessage(r.ProposedHuDraft),
		DedupCandidates:      json.RawMessage(r.DedupCandidates),
		MergeAction:          r.MergeAction,
		ReviewerID:           r.ReviewerID,
		ReviewedAt:           timestamptzToPtr(r.ReviewedAt),
		RejectionReason:      r.RejectionReason,
		CommittedREQ:         r.CommittedReqID,
		CommittedHU:          r.CommittedIssueID,
		FailureReason:        r.FailureReason,
		CreatedAt:            r.CreatedAt,
		UpdatedAt:            r.UpdatedAt,
	}
}

func numericToFloat64Ptr(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	f, err := n.Float64Value()
	if err != nil {
		return nil
	}
	return &f.Float64
}

func float64ToNumeric(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	if err := n.Scan(f); err != nil {
		return pgtype.Numeric{Valid: false}
	}
	return n
}

func timestamptzToPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}
