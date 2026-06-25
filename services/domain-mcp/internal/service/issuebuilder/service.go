// Package issuebuilder — issue-04.7 interactive HU spec wizard.
//
// Implementación inicial mínima: solo mode=feature con state machine de 8 pasos.
// Otros modes (bug-fix, refactor, doc, rfc) marcados unsupported — futuras
// extensiones agregan flows en flow_*.go.
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
)

// Errores estables del wizard.
var (
	ErrNotFound          = errors.New("draft not found")
	ErrInvalidMode       = errors.New("invalid mode")
	ErrInvalidStatus     = errors.New("invalid status for operation")
	ErrInvalidAnswer     = errors.New("answer invalid for current step")
	ErrExpired           = errors.New("draft expired")
	ErrUnsupportedMode   = errors.New("mode not yet supported")





	ErrProjectIDRequired = errors.New("project_id required")
)

// Mode permitidos. Implementados: feature. Resto reservado.
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

// Draft refleja una fila de issue_drafts.
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


	IssueID *uuid.UUID `json:"issue_id,omitempty"`


	IssueSlug string `json:"issue_slug,omitempty"`
}

// Option representa una opción válida para una pregunta del wizard.
type Option struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Recommended bool   `json:"recommended,omitempty"`
}

// Question es la próxima pregunta a contestar.
type Question struct {
	Key      string   `json:"key"`
	Prompt   string   `json:"prompt"`
	Options  []Option `json:"options,omitempty"`
	Progress string   `json:"progress"`
}

// Preview es el snapshot renderizado pre-commit.
type Preview struct {
	Files         map[string]string `json:"files"`
	TargetPath    string            `json:"target_path"`
	SuggestedSlug string            `json:"suggested_slug"`
}

// AttachmentService es el subset de internal/service/attachment.Service que
// el wizard necesita para colgar imágenes a un draft. Inyectable para tests.
type AttachmentService interface {
	InitUpload(ctx context.Context, entityType, entityIDStr, filename, mimeType, createdBy string, size int64) (*AttachmentInitResult, error)
	PromoteEntity(ctx context.Context, fromKind, toKind string, fromID, toID uuid.UUID) (int, error)
}

// AttachmentInitResult — espejo lite del tipo del attachment service.
// Evita import cycle: internal/service/attachment.InitUploadResult tiene
// shape compatible y satisface esto.
type AttachmentInitResult struct {
	AttachmentID uuid.UUID `json:"attachment_id"`
	UploadURL    string    `json:"upload_url"`
	Filename     string    `json:"filename"`
}

// RequirementService es el subset de internal/service/requirement.Service que
// el Commit necesita para auto-crear el REQ padre si no existe. Interface lite
// para evitar acoplar issuebuilder al paquete requirement (mismo patron que
// AttachmentService). Se inyecta via adapter en main.go.
type RequirementService interface {

	Create(ctx context.Context, slug, title, description, status, priority, parentSlug string, projectID *uuid.UUID) (*MaterializedRequirement, error)
}

// IssueService es el subset de internal/service/issue.Service que el Commit
// necesita para materializar el draft en un issue real.
type IssueService interface {



	Create(ctx context.Context, slug, title, description, status, priority, reqSlug string) (*MaterializedIssue, error)
}

// MaterializedRequirement — espejo lite del requirement creado. Solo expone lo
// que el Commit consume (id, slug) para evitar importar requirement.Requirement.
type MaterializedRequirement struct {
	ID   uuid.UUID `json:"id"`
	Slug string    `json:"slug"`
}

// MaterializedIssue — espejo lite del issue creado. El Commit guarda el ID en
// issue_drafts.issue_id y devuelve el slug al cliente.
type MaterializedIssue struct {
	ID   uuid.UUID `json:"id"`
	Slug string    `json:"slug"`
}

// Service orquesta el wizard. Stateless; depende de pgxpool y registry steps.
type Service struct {
	Pool        *pgxpool.Pool
	Audit       *audit.PGRecorder
	Attachments AttachmentService // opcional; si nil → AttachToDraft falla con ErrAttachmentsNotConfigured
	DraftTTLHrs int               // Default 24




	ReqSvc   RequirementService
	IssueSvc IssueService
}

// ErrAttachmentsNotConfigured se devuelve si AttachToDraft se llama sin
// AttachmentService inyectado.
var ErrAttachmentsNotConfigured = errors.New("attachment service not configured")

const defaultDraftTTLHrs = 24

func (s *Service) ttl() time.Duration {
	if s.DraftTTLHrs > 0 {
		return time.Duration(s.DraftTTLHrs) * time.Hour
	}
	return defaultDraftTTLHrs * time.Hour
}

// Start inicia un nuevo wizard. Crea draft y devuelve primera pregunta.
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

	var d Draft



	err := s.Pool.QueryRow(ctx, `
		INSERT INTO issue_drafts (created_by, project_id, mode, initial_idea, total_steps, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_by, mode, initial_idea, answers,
		          current_step, total_steps, status, pending_clarifications,
		          preview, target_path, committed_at, expires_at, created_at, updated_at`,
		createdBy, projectID, mode, initialIdea, len(flow), expires,
	).Scan(&d.ID, &d.CreatedBy, &d.Mode, &d.InitialIdea, &d.Answers,
		&d.CurrentStep, &d.TotalSteps, &d.Status, &d.PendingClarifications,
		&d.Preview, &d.TargetPath, &d.CommittedAt, &d.ExpiresAt, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, nil, fmt.Errorf("insert draft: %w", err)
	}

	q, err := s.questionFor(ctx, &d, flow[0])
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
	return &d, q, nil
}

// Answer recibe una respuesta para el step actual y avanza.
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
	_, _ = s.Pool.Exec(ctx,
		`INSERT INTO issue_draft_steps_log (draft_id, step_key, question, options, answer)
		 VALUES ($1, $2, $3, $4, $5)`,
		draftID, step.Key, step.Prompt, optsJSON, answerJSON,
	)


	newStatus := StatusInProgress
	if nextStep >= len(flow) {
		newStatus = StatusFinished
	}

	err = s.Pool.QueryRow(ctx, `
		UPDATE issue_drafts
		SET answers = $1, current_step = $2, status = $3, updated_at = now()
		WHERE id = $4
		RETURNING id, created_by, mode, initial_idea, answers,
		          current_step, total_steps, status, pending_clarifications,
		          preview, target_path, committed_at, expires_at, created_at, updated_at`,
		newAnswers, nextStep, newStatus, draftID,
	).Scan(&d.ID, &d.CreatedBy, &d.Mode, &d.InitialIdea, &d.Answers,
		&d.CurrentStep, &d.TotalSteps, &d.Status, &d.PendingClarifications,
		&d.Preview, &d.TargetPath, &d.CommittedAt, &d.ExpiresAt, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, nil, fmt.Errorf("update draft: %w", err)
	}

	if newStatus == StatusFinished {
		return d, nil, nil
	}
	q, err := s.questionFor(ctx, d, flow[nextStep])
	if err != nil {
		return nil, nil, err
	}
	return d, q, nil
}

// Get retrieve un draft por ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Draft, error) {
	var d Draft

	err := s.Pool.QueryRow(ctx, `
		SELECT id, created_by, mode, initial_idea, answers,
		       current_step, total_steps, status, pending_clarifications,
		       preview, target_path, committed_at, expires_at, created_at, updated_at
		FROM issue_drafts WHERE id = $1`, id,
	).Scan(&d.ID, &d.CreatedBy, &d.Mode, &d.InitialIdea, &d.Answers,
		&d.CurrentStep, &d.TotalSteps, &d.Status, &d.PendingClarifications,
		&d.Preview, &d.TargetPath, &d.CommittedAt, &d.ExpiresAt, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get draft: %w", err)
	}
	return &d, nil
}

// BuildPreview renderiza preview de los archivos SDD desde answers.
// Requiere status=finished.
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
	_, _ = s.Pool.Exec(ctx,
		`UPDATE issue_drafts SET preview = $1, target_path = $2, updated_at = now()
		 WHERE id = $3`,
		previewJSON, preview.TargetPath, draftID,
	)
	return preview, nil
}

// AttachToDraft sube una imagen/archivo asociada al draft en curso. Genera
// presigned PUT URL via attachment.Service y registra el attachment_id en
// issue_drafts.answers["attachments"] como []{id, filename, mime_type}.
//
// El cliente (Claude Code / agente IA) sube el archivo a UploadURL y luego
// invoca de nuevo `Answer` con el step "attachments_confirmed" si el flow
// lo requiere. Al commit del draft, PromoteAttachments mueve los
// attachments del entity_type=hu_draft → user_story (HU final).
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
	if _, err := s.Pool.Exec(ctx,
		`UPDATE issue_drafts SET answers = $1, updated_at = now() WHERE id = $2`,
		newAnswers, draftID,
	); err != nil {
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

// Commit cierra el draft. Hace dos cosas:
//
//  1. Si ReqSvc + IssueSvc estan inyectados, MATERIALIZA el draft committeado
//     en sdd_requirements + issues: resuelve (o auto-crea) el REQ padre, deriva
//     un issue slug real issue-NN.M-<slug>, crea el issue con el issue.md del
//     preview como description y guarda el issue_id en issue_drafts.issue_id.
//  2. Marca status=committed (committed_at = now()) y bloquea Answer posterior.
//
// Compatibilidad: si ReqSvc o IssueSvc son nil (clientes que aun no los
// inyectan), se conserva el comportamiento legacy — solo marca committed sin
// escribir en issues/sdd_requirements.
//
// NO escribe archivos en disco; eso lo hace el agente que consume el preview.
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

	err = s.Pool.QueryRow(ctx, `
		UPDATE issue_drafts SET status = $1, committed_at = now(), updated_at = now(), issue_id = $3
		WHERE id = $2
		RETURNING id, created_by, mode, initial_idea, answers,
		          current_step, total_steps, status, pending_clarifications,
		          preview, target_path, committed_at, expires_at, created_at, updated_at`,
		StatusCommitted, draftID, issueID,
	).Scan(&d.ID, &d.CreatedBy, &d.Mode, &d.InitialIdea, &d.Answers,
		&d.CurrentStep, &d.TotalSteps, &d.Status, &d.PendingClarifications,
		&d.Preview, &d.TargetPath, &d.CommittedAt, &d.ExpiresAt, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("commit draft: %w", err)
	}
	d.IssueID = issueID
	d.IssueSlug = issueSlug

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
	return d, nil
}

// materialize resuelve el REQ padre (auto-creandolo si no existe), deriva un
// issue slug real y crea el issue con el issue.md del preview como description.
// Devuelve el issue_id + issue_slug materializados.
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

// resolveOrCreateReq devuelve el id del REQ padre. Si no existe, lo crea via
// ReqSvc (status active) scopeado al project_id del draft.
func (s *Service) resolveOrCreateReq(ctx context.Context, d *Draft, reqSlug string) (uuid.UUID, error) {
	var reqID uuid.UUID
	err := s.Pool.QueryRow(ctx,
		`SELECT id FROM sdd_requirements WHERE slug = $1`, reqSlug,
	).Scan(&reqID)
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

// draftProjectID lee issue_drafts.project_id (no esta en el struct Draft).
func (s *Service) draftProjectID(ctx context.Context, draftID uuid.UUID) *uuid.UUID {
	var pid *uuid.UUID
	_ = s.Pool.QueryRow(ctx,
		`SELECT project_id FROM issue_drafts WHERE id = $1`, draftID,
	).Scan(&pid)
	return pid
}

// nextIssueOrdinal calcula el siguiente correlativo M para issue-NN.M-* dentro
// de un REQ. Cuenta los issues ya colgados de ese req_id y suma 1 (1-based).
func (s *Service) nextIssueOrdinal(ctx context.Context, reqID uuid.UUID) (int, error) {
	var count int
	err := s.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM issues WHERE req_id = $1`, reqID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("contar issues del REQ: %w", err)
	}
	return count + 1, nil
}

// issueMarkdownFromPreview devuelve el issue.md renderizado para usar como
// description del issue. Si el draft ya tiene preview persistido, lo reutiliza;
// si no, lo re-renderiza desde answers.
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

// cappedIssueSlug arma issue-NN.M-<slug> garantizando <= 50 chars (issues.slug
// es VARCHAR(50)). Si excede, trunca el sufijo <slug> por el final y limpia
// guiones colgantes para no romper el patron ^issue-\d+\.\d+(-[a-z0-9-]+)?$.
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

// reReqNumber extrae el numero NN de un slug REQ-NN[-...].
var reReqNumber = regexp.MustCompile(`^REQ-(\d+)`)

// reqNumberFromSlug devuelve el NN de un slug REQ-NN-* ("" si no matchea).
func reqNumberFromSlug(slug string) string {
	m := reReqNumber.FindStringSubmatch(slug)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// mapPriority traduce la prioridad del wizard (es) a la del issue (en).
// issue.Create valida low/medium/high/critical; default medium.
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

// PromoteAttachmentsToHU mueve los attachments vinculados al draft hacia
// la HU final creada por el agente IA tras consumir el preview. Llamado
// por el agente al hacer el write filesystem + INSERT user_story.
// Retorna el count de attachments movidos.
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
				"draft_id":     draftID.String(),
				"moved_count":  moved,
			},
		})
	}
	return moved, nil
}

// Abandon marca el draft como abandonado.
func (s *Service) Abandon(ctx context.Context, draftID uuid.UUID, reason string) error {
	d, err := s.Get(ctx, draftID)
	if err != nil {
		return err
	}
	if d.Status == StatusCommitted {
		return fmt.Errorf("%w: cannot abandon committed", ErrInvalidStatus)
	}
	_, err = s.Pool.Exec(ctx,
		`UPDATE issue_drafts SET status = $1, updated_at = now() WHERE id = $2`,
		StatusAbandoned, draftID,
	)
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

// List devuelve drafts filtrados por status (vacío = todos los activos).
func (s *Service) List(ctx context.Context, status string) ([]Draft, error) {

	q := `SELECT id, created_by, mode, initial_idea, answers,
	             current_step, total_steps, status, pending_clarifications,
	             preview, target_path, committed_at, expires_at, created_at, updated_at
	      FROM issue_drafts`
	args := []any{}
	if status != "" {
		q += ` WHERE status = $1`
		args = append(args, status)
	}
	q += ` ORDER BY created_at DESC LIMIT 100`

	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list drafts: %w", err)
	}
	defer rows.Close()

	var out []Draft
	for rows.Next() {
		var d Draft
		if err := rows.Scan(&d.ID, &d.CreatedBy, &d.Mode, &d.InitialIdea, &d.Answers,
			&d.CurrentStep, &d.TotalSteps, &d.Status, &d.PendingClarifications,
			&d.Preview, &d.TargetPath, &d.CommittedAt, &d.ExpiresAt, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan draft: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Service) markStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE issue_drafts SET status = $1, updated_at = now() WHERE id = $2`, status, id,
	)
	return err
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
