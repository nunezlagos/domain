package issuebuilder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/service/issuebuilder/issuebuilderdb"
	"nunezlagos/domain/internal/store/txctx"
)

var (
	ErrNotFound          = errors.New("draft not found")
	ErrInvalidMode       = errors.New("invalid mode")
	ErrInvalidStatus     = errors.New("invalid status for operation")
	ErrInvalidAnswer     = errors.New("answer invalid for current step")
	ErrExpired           = errors.New("draft expired")
	ErrUnsupportedMode   = errors.New("mode not yet supported")
	ErrProjectIDRequired = errors.New("project_id required")
)

const (
	ModeFeature  = "feature"
	ModeBugFix   = "bug-fix"
	ModeRefactor = "refactor"
	ModeDoc      = "doc"
	ModeRFC      = "rfc"
)

const (
	StatusInProgress = "in_progress"
	StatusFinished   = "finished"
	StatusCommitted  = "committed"
	StatusExpired    = "expired"
	StatusAbandoned  = "abandoned"
)

type Draft struct {
	ID                    uuid.UUID       `json:"id"`
	OrganizationID        *uuid.UUID      `json:"organization_id,omitempty"`
	CreatedBy             *uuid.UUID      `json:"created_by,omitempty"`
	Mode                  string          `json:"mode"`
	InitialIdea           string          `json:"initial_idea"`
	Answers               json.RawMessage `json:"answers"`
	CurrentStep           int             `json:"current_step"`
	TotalSteps            int             `json:"total_steps"`
	Status                string          `json:"status"`
	PendingClarifications json.RawMessage `json:"pending_clarifications"`
	Preview               json.RawMessage `json:"preview,omitempty"`
	TargetPath            *string         `json:"target_path,omitempty"`
	CommittedAt           *time.Time      `json:"committed_at,omitempty"`
	ExpiresAt             time.Time       `json:"expires_at"`
	CreatedAt             time.Time       `json:"created_at"`
	UpdatedAt             time.Time       `json:"updated_at"`
	IssueID               *uuid.UUID      `json:"issue_id,omitempty"`
	IssueSlug             string          `json:"issue_slug,omitempty"`
}

type Option struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Recommended bool   `json:"recommended,omitempty"`
}

type Question struct {
	Key      string   `json:"key"`
	Prompt   string   `json:"prompt"`
	Options  []Option `json:"options,omitempty"`
	Progress string   `json:"progress"`
}

type Preview struct {
	Files         map[string]string `json:"files"`
	TargetPath    string            `json:"target_path"`
	SuggestedSlug string            `json:"suggested_slug"`
}

type AttachmentService interface {
	InitUpload(ctx context.Context, entityType, entityIDStr, filename, mimeType, createdBy string, size int64) (*AttachmentInitResult, error)
	PromoteEntity(ctx context.Context, fromKind, toKind string, fromID, toID uuid.UUID) (int, error)
}

type AttachmentInitResult struct {
	AttachmentID uuid.UUID `json:"attachment_id"`
	UploadURL    string    `json:"upload_url"`
	Filename     string    `json:"filename"`
}

type RequirementService interface {
	Create(ctx context.Context, slug, title, description, status, priority, parentSlug string, projectID *uuid.UUID) (*MaterializedRequirement, error)
}

type IssueService interface {
	Create(ctx context.Context, slug, title, description, status, priority, reqSlug string) (*MaterializedIssue, error)
}

type MaterializedRequirement struct {
	ID   uuid.UUID `json:"id"`
	Slug string    `json:"slug"`
}

type MaterializedIssue struct {
	ID   uuid.UUID `json:"id"`
	Slug string    `json:"slug"`
}

type Service struct {
	Pool        *pgxpool.Pool
	Audit       *audit.PGRecorder
	Attachments AttachmentService
	DraftTTLHrs int

	ReqSvc   RequirementService
	IssueSvc IssueService
}

func (s *Service) q(ctx context.Context) *issuebuilderdb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return issuebuilderdb.New(tx)
	}
	return issuebuilderdb.New(s.Pool)
}

var ErrAttachmentsNotConfigured = errors.New("attachment service not configured")

const defaultDraftTTLHrs = 24

func (s *Service) ttl() time.Duration {
	if s.DraftTTLHrs > 0 {
		return time.Duration(s.DraftTTLHrs) * time.Hour
	}
	return defaultDraftTTLHrs * time.Hour
}

func toDraft(id uuid.UUID, createdBy *uuid.UUID, mode, initialIdea string, answers []byte, currentStep, totalSteps int32, status string, pendingClarifications []byte, preview []byte, targetPath *string, committedAt *time.Time, expiresAt time.Time, createdAt time.Time, updatedAt time.Time) Draft {
	return Draft{
		ID:                    id,
		CreatedBy:             createdBy,
		Mode:                  mode,
		InitialIdea:           initialIdea,
		Answers:               json.RawMessage(answers),
		CurrentStep:           int(currentStep),
		TotalSteps:            int(totalSteps),
		Status:                status,
		PendingClarifications: json.RawMessage(pendingClarifications),
		Preview:               json.RawMessage(preview),
		TargetPath:            targetPath,
		CommittedAt:           committedAt,
		ExpiresAt:             expiresAt,
		CreatedAt:             createdAt,
		UpdatedAt:             updatedAt,
	}
}

func toDraftFromGet(r issuebuilderdb.GetDraftRow) Draft {
	return toDraft(r.ID, r.CreatedBy, r.Mode, r.InitialIdea, r.Answers, r.CurrentStep, r.TotalSteps, r.Status, r.PendingClarifications, r.Preview, r.TargetPath, r.CommittedAt, r.ExpiresAt, r.CreatedAt, r.UpdatedAt)
}

func toDraftFromList(r issuebuilderdb.ListDraftsRow) Draft {
	return toDraft(r.ID, r.CreatedBy, r.Mode, r.InitialIdea, r.Answers, r.CurrentStep, r.TotalSteps, r.Status, r.PendingClarifications, r.Preview, r.TargetPath, r.CommittedAt, r.ExpiresAt, r.CreatedAt, r.UpdatedAt)
}

func toDraftFromInsert(r issuebuilderdb.InsertDraftRow) Draft {
	return toDraft(r.ID, r.CreatedBy, r.Mode, r.InitialIdea, r.Answers, r.CurrentStep, r.TotalSteps, r.Status, r.PendingClarifications, r.Preview, r.TargetPath, r.CommittedAt, r.ExpiresAt, r.CreatedAt, r.UpdatedAt)
}

func toDraftFromUpdate(r issuebuilderdb.UpdateDraftAfterAnswerRow) Draft {
	return toDraft(r.ID, r.CreatedBy, r.Mode, r.InitialIdea, r.Answers, r.CurrentStep, r.TotalSteps, r.Status, r.PendingClarifications, r.Preview, r.TargetPath, r.CommittedAt, r.ExpiresAt, r.CreatedAt, r.UpdatedAt)
}

func toDraftFromCommit(r issuebuilderdb.CommitDraftRow) Draft {
	return toDraft(r.ID, r.CreatedBy, r.Mode, r.InitialIdea, r.Answers, r.CurrentStep, r.TotalSteps, r.Status, r.PendingClarifications, r.Preview, r.TargetPath, r.CommittedAt, r.ExpiresAt, r.CreatedAt, r.UpdatedAt)
}

func (s *Service) Start(ctx context.Context, mode, initialIdea string, createdBy *uuid.UUID, projectID *uuid.UUID) (*Draft, *Question, error) {
	flow, ok := flowsByMode[mode]
	if !ok {
		return nil, nil, fmt.Errorf("%w: %s", ErrInvalidMode, mode)
	}
	if len(flow) == 0 {
		return nil, nil, fmt.Errorf("%w: %s", ErrUnsupportedMode, mode)
	}
	if strings.TrimSpace(initialIdea) == "" {
		return nil, nil, fmt.Errorf("initial_idea required")
	}

	if projectID == nil || *projectID == uuid.Nil {
		return nil, nil, ErrProjectIDRequired
	}

	expires := time.Now().Add(s.ttl())

	q := s.q(ctx)
	dRow, err := q.InsertDraft(ctx, issuebuilderdb.InsertDraftParams{
		CreatedBy:   createdBy,
		ProjectID:   *projectID,
		Mode:        mode,
		InitialIdea: initialIdea,
		TotalSteps:  int32(len(flow)),
		ExpiresAt:   expires,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("insert draft: %w", err)
	}

	d := toDraftFromInsert(dRow)
	q2, err := s.questionFor(ctx, &d, flow[0])
	if err != nil {
		return nil, nil, err
	}
	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorType:  audit.ActorSystem,
			Action:     "hu_draft.started",
			EntityType: "hu_draft",
			EntityID:   &d.ID,
			NewValues:  map[string]any{"mode": mode},
		})
	}
	return &d, q2, nil
}

func (s *Service) Answer(ctx context.Context, draftID uuid.UUID, rawAnswer any) (*Draft, *Question, error) {
	d, err := s.Get(ctx, draftID)
	if err != nil {
		return nil, nil, err
	}
	if d.Status != StatusInProgress {
		return nil, nil, fmt.Errorf("%w: status=%s", ErrInvalidStatus, d.Status)
	}
	if time.Now().After(d.ExpiresAt) {
		_ = s.markStatus(ctx, draftID, StatusExpired)
		return nil, nil, ErrExpired
	}

	flow := flowsByMode[d.Mode]
	if d.CurrentStep >= len(flow) {
		return nil, nil, fmt.Errorf("draft already past last step")
	}
	step := flow[d.CurrentStep]

	if err := step.Validate(rawAnswer); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidAnswer, err)
	}

	answers := map[string]any{}
	_ = json.Unmarshal(d.Answers, &answers)
	answers[step.Key] = rawAnswer

	newAnswers, _ := json.Marshal(answers)
	nextStep := d.CurrentStep + 1

	optsJSON, _ := json.Marshal(step.options)
	answerJSON, _ := json.Marshal(rawAnswer)
	q := s.q(ctx)
	_ = q.InsertStepLog(ctx, issuebuilderdb.InsertStepLogParams{
		IssueDraftID: draftID,
		StepKey:      step.Key,
		Question:     step.Prompt,
		Options:      optsJSON,
		Answer:       answerJSON,
	})

	newStatus := StatusInProgress
	if nextStep >= len(flow) {
		newStatus = StatusFinished
	}

	dRow, err := q.UpdateDraftAfterAnswer(ctx, issuebuilderdb.UpdateDraftAfterAnswerParams{
		Answers:     newAnswers,
		CurrentStep: int32(nextStep),
		Status:      newStatus,
		ID:          draftID,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("update draft: %w", err)
	}

	updated := toDraftFromUpdate(dRow)

	if newStatus == StatusFinished {
		return &updated, nil, nil
	}
	q3, err := s.questionFor(ctx, &updated, flow[nextStep])
	if err != nil {
		return nil, nil, err
	}
	return &updated, q3, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Draft, error) {
	dRow, err := s.q(ctx).GetDraft(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get draft: %w", err)
	}
	d := toDraftFromGet(dRow)
	return &d, nil
}

func (s *Service) BuildPreview(ctx context.Context, draftID uuid.UUID) (*Preview, error) {
	d, err := s.Get(ctx, draftID)
	if err != nil {
		return nil, err
	}
	if d.Status != StatusFinished && d.Status != StatusCommitted {
		return nil, fmt.Errorf("%w: status=%s", ErrInvalidStatus, d.Status)
	}

	answers := map[string]any{}
	_ = json.Unmarshal(d.Answers, &answers)

	preview, err := renderFeaturePreview(d, answers)
	if err != nil {
		return nil, err
	}

	previewJSON, _ := json.Marshal(preview)
	err = s.q(ctx).UpdateDraftPreview(ctx, issuebuilderdb.UpdateDraftPreviewParams{
		Preview:    previewJSON,
		TargetPath: &preview.TargetPath,
		ID:         draftID,
	})
	if err != nil {
		return nil, fmt.Errorf("update preview: %w", err)
	}
	return preview, nil
}

func (s *Service) AttachToDraft(ctx context.Context, draftID uuid.UUID, filename, mimeType string, size int64) (*AttachmentInitResult, error) {
	if s.Attachments == nil {
		return nil, ErrAttachmentsNotConfigured
	}
	d, err := s.Get(ctx, draftID)
	if err != nil {
		return nil, err
	}
	if d.Status != StatusInProgress && d.Status != StatusFinished {
		return nil, fmt.Errorf("%w: cannot attach to %s draft", ErrInvalidStatus, d.Status)
	}

	createdBy := ""
	if d.CreatedBy != nil {
		createdBy = d.CreatedBy.String()
	}
	res, err := s.Attachments.InitUpload(ctx,
		"hu_draft", draftID.String(),
		filename, mimeType, createdBy, size)
	if err != nil {
		return nil, fmt.Errorf("init upload: %w", err)
	}

	answers := map[string]any{}
	_ = json.Unmarshal(d.Answers, &answers)
	existing, _ := answers["attachments"].([]any)
	existing = append(existing, map[string]any{
		"attachment_id": res.AttachmentID.String(),
		"filename":      filename,
		"mime_type":     mimeType,
		"size":          size,
	})
	answers["attachments"] = existing
	newAnswers, _ := json.Marshal(answers)

	err = s.q(ctx).UpdateDraftAnswers(ctx, issuebuilderdb.UpdateDraftAnswersParams{
		Answers: newAnswers,
		ID:      draftID,
	})
	if err != nil {
		return nil, fmt.Errorf("persist attachment ref: %w", err)
	}

	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorType:  audit.ActorSystem,
			Action:     "hu_draft.attached",
			EntityType: "hu_draft",
			EntityID:   &draftID,
			NewValues: map[string]any{
				"attachment_id": res.AttachmentID.String(),
				"filename":      filename,
			},
		})
	}
	return res, nil
}

func (s *Service) Commit(ctx context.Context, draftID uuid.UUID) (*Draft, error) {
	d, err := s.Get(ctx, draftID)
	if err != nil {
		return nil, err
	}
	if d.Status != StatusFinished {
		return nil, fmt.Errorf("%w: status=%s", ErrInvalidStatus, d.Status)
	}

	var issueID *uuid.UUID
	var issueSlug string
	if s.ReqSvc != nil && s.IssueSvc != nil {
		issueID, issueSlug, err = s.materialize(ctx, d)
		if err != nil {
			return nil, fmt.Errorf("materialize draft: %w", err)
		}
	}

	dRow, err := s.q(ctx).CommitDraft(ctx, issuebuilderdb.CommitDraftParams{
		Status:  StatusCommitted,
		ID:      draftID,
		IssueID: issueID,
	})
	if err != nil {
		return nil, fmt.Errorf("commit draft: %w", err)
	}

	committed := toDraftFromCommit(dRow)
	committed.IssueID = issueID
	committed.IssueSlug = issueSlug

	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorType:  audit.ActorSystem,
			Action:     "hu_draft.committed",
			EntityType: "hu_draft",
			EntityID:   &d.ID,
			NewValues:  map[string]any{"target_path": d.TargetPath},
		})
		if issueID != nil {
			audit.RecordOrLog(ctx, s.Audit, audit.Event{
				ActorType:  audit.ActorSystem,
				Action:     "hu_draft.materialized",
				EntityType: "user_story",
				EntityID:   issueID,
				NewValues: map[string]any{
					"draft_id":   d.ID.String(),
					"issue_slug": issueSlug,
				},
			})
		}
	}
	return &committed, nil
}

func (s *Service) materialize(ctx context.Context, d *Draft) (*uuid.UUID, string, error) {
	answers := map[string]any{}
	_ = json.Unmarshal(d.Answers, &answers)

	reqParent, _ := answers["req_parent"].(string)
	slug, _ := answers["slug"].(string)
	goal, _ := answers["goal"].(string)
	summary, _ := answers["summary"].(string)
	priority, _ := answers["priority"].(string)
	if reqParent == "" || slug == "" {
		return nil, "", fmt.Errorf("answers incompletos: req_parent=%q slug=%q", reqParent, slug)
	}

	reqNum := reqNumberFromSlug(reqParent)
	if reqNum == "" {
		return nil, "", fmt.Errorf("req_parent %q invalido: se espera formato REQ-NN", reqParent)
	}

	reqID, err := s.resolveOrCreateReq(ctx, d, reqParent)
	if err != nil {
		return nil, "", err
	}

	next, err := s.nextIssueOrdinal(ctx, reqID)
	if err != nil {
		return nil, "", err
	}
	issueSlug := cappedIssueSlug(reqNum, next, slug)

	issuePriority := mapPriority(priority)

	title := goal
	if title == "" {
		title = slug
	}
	description := s.issueMarkdownFromPreview(d, answers, summary)

	created, err := s.IssueSvc.Create(ctx, issueSlug, title, description,
		"proposed", issuePriority, reqParent)
	if err != nil {
		return nil, "", fmt.Errorf("crear issue: %w", err)
	}
	return &created.ID, created.Slug, nil
}

func (s *Service) resolveOrCreateReq(ctx context.Context, d *Draft, reqSlug string) (uuid.UUID, error) {
	reqID, err := s.q(ctx).FindRequirementBySlug(ctx, reqSlug)
	if err == nil {
		return reqID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, fmt.Errorf("buscar REQ padre: %w", err)
	}

	projectID := s.draftProjectID(ctx, d.ID)
	created, cerr := s.ReqSvc.Create(ctx, reqSlug, reqSlug, "", "active", "medium", "", projectID)
	if cerr != nil {
		return uuid.Nil, fmt.Errorf("auto-crear REQ padre %q: %w", reqSlug, cerr)
	}
	return created.ID, nil
}

func (s *Service) draftProjectID(ctx context.Context, draftID uuid.UUID) *uuid.UUID {
	pid, err := s.q(ctx).GetDraftProjectID(ctx, draftID)
	if err != nil {
		return nil
	}
	return &pid
}

func (s *Service) nextIssueOrdinal(ctx context.Context, reqID uuid.UUID) (int, error) {
	count, err := s.q(ctx).CountIssuesByReqID(ctx, reqID)
	if err != nil {
		return 0, fmt.Errorf("contar issues del REQ: %w", err)
	}
	return int(count) + 1, nil
}

func (s *Service) issueMarkdownFromPreview(d *Draft, answers map[string]any, summary string) string {
	if len(d.Preview) > 0 {
		var pv Preview
		if err := json.Unmarshal(d.Preview, &pv); err == nil {
			if md, ok := pv.Files["issue.md"]; ok && md != "" {
				return md
			}
		}
	}
	if pv, err := renderFeaturePreview(d, answers); err == nil {
		if md, ok := pv.Files["issue.md"]; ok {
			return md
		}
	}
	return summary
}

func cappedIssueSlug(reqNum string, ordinal int, slug string) string {
	const maxLen = 50
	prefix := fmt.Sprintf("issue-%s.%d-", reqNum, ordinal)
	avail := maxLen - len(prefix)
	if avail <= 0 {
		return strings.TrimRight(fmt.Sprintf("issue-%s.%d", reqNum, ordinal), "-")
	}
	if len(slug) > avail {
		slug = strings.TrimRight(slug[:avail], "-")
	}
	return prefix + slug
}

var reReqNumber = regexp.MustCompile(`^REQ-(\d+)`)

func reqNumberFromSlug(slug string) string {
	m := reReqNumber.FindStringSubmatch(slug)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func mapPriority(p string) string {
	switch p {
	case "alta":
		return "high"
	case "media":
		return "medium"
	case "baja":
		return "low"
	default:
		return "medium"
	}
}

func (s *Service) PromoteAttachmentsToHU(ctx context.Context, draftID, issueID uuid.UUID) (int, error) {
	if s.Attachments == nil {
		return 0, nil
	}
	d, err := s.Get(ctx, draftID)
	if err != nil {
		return 0, err
	}
	if d.Status != StatusCommitted {
		return 0, fmt.Errorf("%w: draft must be committed first (got %s)",
			ErrInvalidStatus, d.Status)
	}
	moved, err := s.Attachments.PromoteEntity(ctx, "hu_draft", "user_story", draftID, issueID)
	if err != nil {
		return 0, err
	}
	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorType:  audit.ActorSystem,
			Action:     "hu_draft.attachments_promoted",
			EntityType: "user_story",
			EntityID:   &issueID,
			NewValues: map[string]any{
				"draft_id":    draftID.String(),
				"moved_count": moved,
			},
		})
	}
	return moved, nil
}

func (s *Service) Abandon(ctx context.Context, draftID uuid.UUID, reason string) error {
	d, err := s.Get(ctx, draftID)
	if err != nil {
		return err
	}
	if d.Status == StatusCommitted {
		return fmt.Errorf("%w: cannot abandon committed", ErrInvalidStatus)
	}
	err = s.q(ctx).AbandonDraft(ctx, issuebuilderdb.AbandonDraftParams{
		Status: StatusAbandoned,
		ID:     draftID,
	})
	if err != nil {
		return fmt.Errorf("abandon draft: %w", err)
	}
	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorType:  audit.ActorSystem,
			Action:     "hu_draft.abandoned",
			EntityType: "hu_draft",
			EntityID:   &draftID,
			NewValues:  map[string]any{"reason": reason},
		})
	}
	return nil
}

func (s *Service) List(ctx context.Context, status string) ([]Draft, error) {
	var statusFilter *string
	if status != "" {
		statusFilter = &status
	}
	rows, err := s.q(ctx).ListDrafts(ctx, statusFilter)
	if err != nil {
		return nil, fmt.Errorf("list drafts: %w", err)
	}
	out := make([]Draft, len(rows))
	for i, r := range rows {
		out[i] = toDraftFromList(r)
	}
	return out, nil
}

func (s *Service) markStatus(ctx context.Context, id uuid.UUID, status string) error {
	return s.q(ctx).MarkDraftStatus(ctx, issuebuilderdb.MarkDraftStatusParams{
		Status: status,
		ID:     id,
	})
}

func (s *Service) questionFor(ctx context.Context, d *Draft, st step) (*Question, error) {
	opts := st.options
	if st.optionsFn != nil {
		dynamic, err := st.optionsFn(ctx, s.Pool, d)
		if err != nil {
			return nil, fmt.Errorf("build options for %s: %w", st.Key, err)
		}
		opts = dynamic
	}
	flow := flowsByMode[d.Mode]
	return &Question{
		Key:      st.Key,
		Prompt:   st.Prompt,
		Options:  opts,
		Progress: fmt.Sprintf("%d/%d", d.CurrentStep+1, len(flow)),
	}, nil
}
