// Package intake — issue-04.8 unified requirement intake pipeline.
//
// MVP scope: Submit + transitions + Approve/Reject/Commit. LLM classification,
// embedding dedup y external sync quedan como futuras extensiones del pipeline
// (no incluidas en este commit inicial). El estado y los hooks de transición
// están listos para que un worker async procese los pasos pendientes.
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
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
)

var (
	ErrNotFound      = errors.New("intake not found")
	ErrInvalidStatus = errors.New("invalid status for operation")
	ErrInvalidSource = errors.New("invalid source")
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

// Submit acepta un payload crudo desde cualquier origen. Devuelve el intake
// recién creado en status=received. El procesamiento posterior (classify,
// dedupe, structure) lo hace un worker async leyendo de status="received".
func (s *Service) Submit(ctx context.Context, in SubmitInput) (*Payload, error) {
	if !validSources[in.Source] {
		return nil, fmt.Errorf("%w: %s", ErrInvalidSource, in.Source)
	}
	if strings.TrimSpace(in.RawText) == "" {
		return nil, fmt.Errorf("raw_text required")
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

	var p Payload
	// ISSUE-21.6 Fase D clean Round 3: organization_id se omite del INSERT
	// (single-org, nullable post-000145).
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO issue_intake_payloads (source, source_ref, submitted_by,
		                             raw_text, raw_payload)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, source, source_ref, submitted_by, raw_text, raw_payload,
		          status, classified_type, classified_severity, classified_confidence,
		          classification_reasoning, needs_clarification, proposed_title,
		          proposed_description, proposed_req_slug, proposed_hu_draft,
		          dedup_candidates, merge_action, reviewer_id, reviewed_at,
		          rejection_reason, committed_req_id, committed_issue_id, failure_reason,
		          created_at, updated_at`,
		in.Source, srcRef, subBy, in.RawText, payloadJSON,
	).Scan(scanPayloadCols(&p)...)
	if err != nil {
		return nil, fmt.Errorf("insert intake: %w", err)
	}

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
	var p Payload
	// ISSUE-21.6: organization_id omitido del SELECT (dropeado en Fase C).
	err := s.Pool.QueryRow(ctx, `
		SELECT id, source, source_ref, submitted_by, raw_text, raw_payload,
		       status, classified_type, classified_severity, classified_confidence,
		       classification_reasoning, needs_clarification, proposed_title,
		       proposed_description, proposed_req_slug, proposed_hu_draft,
		       dedup_candidates, merge_action, reviewer_id, reviewed_at,
		       rejection_reason, committed_req_id, committed_issue_id, failure_reason,
		       created_at, updated_at
		FROM issue_intake_payloads WHERE id = $1`, id,
	).Scan(scanPayloadCols(&p)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get intake: %w", err)
	}
	return &p, nil
}

// UpdateClassification persiste el resultado del paso classify.
func (s *Service) UpdateClassification(ctx context.Context, id uuid.UUID, type_, severity string, confidence float64, reasoning string) (*Payload, error) {
	_, err := s.Pool.Exec(ctx, `
		UPDATE issue_intake_payloads
		SET classified_type = $1, classified_severity = $2, classified_confidence = $3,
		    classification_reasoning = $4, needs_clarification = $5,
		    status = $6, updated_at = now()
		WHERE id = $7`,
		type_, severity, confidence, reasoning, confidence < 0.6, StatusClassified, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update classification: %w", err)
	}
	return s.Get(ctx, id)
}

// MarkPendingReview transiciona a pending_review (después de structuring).
func (s *Service) MarkPendingReview(ctx context.Context, id uuid.UUID, title, description, reqSlug string, issueDraft map[string]any, dedup []any, mergeAction string) (*Payload, error) {
	huJSON, _ := json.Marshal(issueDraft)
	dedupJSON, _ := json.Marshal(dedup)

	_, err := s.Pool.Exec(ctx, `
		UPDATE issue_intake_payloads
		SET proposed_title = $1, proposed_description = $2, proposed_req_slug = $3,
		    proposed_hu_draft = $4, dedup_candidates = $5, merge_action = $6,
		    status = $7, updated_at = now()
		WHERE id = $8`,
		title, description, reqSlug, huJSON, dedupJSON, mergeAction,
		StatusPendingReview, id,
	)
	if err != nil {
		return nil, fmt.Errorf("mark pending review: %w", err)
	}
	return s.Get(ctx, id)
}

// Approve marca como aprobado (precondición a commit).
func (s *Service) Approve(ctx context.Context, id uuid.UUID, reviewerID uuid.UUID) (*Payload, error) {
	p, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.Status != StatusPendingReview {
		return nil, fmt.Errorf("%w: status=%s", ErrInvalidStatus, p.Status)
	}
	_, err = s.Pool.Exec(ctx, `
		UPDATE issue_intake_payloads SET status = $1, reviewer_id = $2, reviewed_at = now(),
		                            updated_at = now()
		WHERE id = $3`,
		StatusApproved, reviewerID, id,
	)
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

// Reject marca como rechazado con razón.
func (s *Service) Reject(ctx context.Context, id, reviewerID uuid.UUID, reason string) (*Payload, error) {
	p, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.Status == StatusCommitted {
		return nil, fmt.Errorf("%w: cannot reject committed", ErrInvalidStatus)
	}
	_, err = s.Pool.Exec(ctx, `
		UPDATE issue_intake_payloads SET status = $1, reviewer_id = $2, reviewed_at = now(),
		                            rejection_reason = $3, updated_at = now()
		WHERE id = $4`,
		StatusRejected, reviewerID, reason, id,
	)
	if err != nil {
		return nil, fmt.Errorf("reject: %w", err)
	}
	return s.Get(ctx, id)
}

// LinkCommitted asocia el intake con el REQ/HU creado.
func (s *Service) LinkCommitted(ctx context.Context, id uuid.UUID, reqID, issueID *uuid.UUID) (*Payload, error) {
	p, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.Status != StatusApproved {
		return nil, fmt.Errorf("%w: must be approved, got %s", ErrInvalidStatus, p.Status)
	}
	_, err = s.Pool.Exec(ctx, `
		UPDATE issue_intake_payloads SET status = $1, committed_req_id = $2,
		                            committed_issue_id = $3, updated_at = now()
		WHERE id = $4`,
		StatusCommitted, reqID, issueID, id,
	)
	if err != nil {
		return nil, fmt.Errorf("link committed: %w", err)
	}
	return s.Get(ctx, id)
}

// ListPending devuelve intakes en cualquier status no-terminal.
func (s *Service) ListPending(ctx context.Context, limit int) ([]Payload, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	// ISSUE-21.6: organization_id omitido del SELECT (dropeado en Fase C).
	rows, err := s.Pool.Query(ctx, `
		SELECT id, source, source_ref, submitted_by, raw_text, raw_payload,
		       status, classified_type, classified_severity, classified_confidence,
		       classification_reasoning, needs_clarification, proposed_title,
		       proposed_description, proposed_req_slug, proposed_hu_draft,
		       dedup_candidates, merge_action, reviewer_id, reviewed_at,
		       rejection_reason, committed_req_id, committed_issue_id, failure_reason,
		       created_at, updated_at
		FROM issue_intake_payloads
		WHERE status NOT IN ('committed','rejected','failed')
		ORDER BY created_at ASC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list pending: %w", err)
	}
	defer rows.Close()

	var out []Payload
	for rows.Next() {
		var p Payload
		if err := rows.Scan(scanPayloadCols(&p)...); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func scanPayloadCols(p *Payload) []any {
	// ISSUE-21.6 Fase D clean Round 3: OrganizationID removido del scan
	// (la columna ya no se selecciona en INSERT/SELECT).
	return []any{
		&p.ID, &p.Source, &p.SourceRef, &p.SubmittedBy,
		&p.RawText, &p.RawPayload, &p.Status,
		&p.ClassifiedType, &p.ClassifiedSeverity, &p.ClassifiedConfidence,
		&p.ClassificationReason, &p.NeedsClarification,
		&p.ProposedTitle, &p.ProposedDescription, &p.ProposedReqSlug,
		&p.ProposedHUDraft, &p.DedupCandidates, &p.MergeAction,
		&p.ReviewerID, &p.ReviewedAt, &p.RejectionReason,
		&p.CommittedREQ, &p.CommittedHU, &p.FailureReason,
		&p.CreatedAt, &p.UpdatedAt,
	}
}
