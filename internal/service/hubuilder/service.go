// Package hubuilder — HU-04.7 interactive HU spec wizard.
//
// Implementación inicial mínima: solo mode=feature con state machine de 8 pasos.
// Otros modes (bug-fix, refactor, doc, rfc) marcados unsupported — futuras
// extensiones agregan flows en flow_*.go.
package hubuilder

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

// Errores estables del wizard.
var (
	ErrNotFound          = errors.New("draft not found")
	ErrInvalidMode       = errors.New("invalid mode")
	ErrInvalidStatus     = errors.New("invalid status for operation")
	ErrInvalidAnswer     = errors.New("answer invalid for current step")
	ErrExpired           = errors.New("draft expired")
	ErrUnsupportedMode   = errors.New("mode not yet supported")
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

// Draft refleja una fila de hu_drafts.
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

// Service orquesta el wizard. Stateless; depende de pgxpool y registry steps.
type Service struct {
	Pool         *pgxpool.Pool
	Audit        *audit.PGRecorder
	DraftTTLHrs  int // Default 24
}

const defaultDraftTTLHrs = 24

func (s *Service) ttl() time.Duration {
	if s.DraftTTLHrs > 0 {
		return time.Duration(s.DraftTTLHrs) * time.Hour
	}
	return defaultDraftTTLHrs * time.Hour
}

// Start inicia un nuevo wizard. Crea draft y devuelve primera pregunta.
func (s *Service) Start(ctx context.Context, mode, initialIdea string, createdBy *uuid.UUID) (*Draft, *Question, error) {
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

	expires := time.Now().Add(s.ttl())

	var d Draft
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO hu_drafts (created_by, mode, initial_idea, total_steps, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, organization_id, created_by, mode, initial_idea, answers,
		          current_step, total_steps, status, pending_clarifications,
		          preview, target_path, committed_at, expires_at, created_at, updated_at`,
		createdBy, mode, initialIdea, len(flow), expires,
	).Scan(&d.ID, &d.OrganizationID, &d.CreatedBy, &d.Mode, &d.InitialIdea, &d.Answers,
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
		_ = s.Audit.Record(ctx, audit.Event{
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

	// Log step
	optsJSON, _ := json.Marshal(step.options)
	answerJSON, _ := json.Marshal(rawAnswer)
	_, _ = s.Pool.Exec(ctx,
		`INSERT INTO hu_draft_steps_log (draft_id, step_key, question, options, answer)
		 VALUES ($1, $2, $3, $4, $5)`,
		draftID, step.Key, step.Prompt, optsJSON, answerJSON,
	)

	// Determine new status
	newStatus := StatusInProgress
	if nextStep >= len(flow) {
		newStatus = StatusFinished
	}

	err = s.Pool.QueryRow(ctx, `
		UPDATE hu_drafts
		SET answers = $1, current_step = $2, status = $3, updated_at = now()
		WHERE id = $4
		RETURNING id, organization_id, created_by, mode, initial_idea, answers,
		          current_step, total_steps, status, pending_clarifications,
		          preview, target_path, committed_at, expires_at, created_at, updated_at`,
		newAnswers, nextStep, newStatus, draftID,
	).Scan(&d.ID, &d.OrganizationID, &d.CreatedBy, &d.Mode, &d.InitialIdea, &d.Answers,
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
		SELECT id, organization_id, created_by, mode, initial_idea, answers,
		       current_step, total_steps, status, pending_clarifications,
		       preview, target_path, committed_at, expires_at, created_at, updated_at
		FROM hu_drafts WHERE id = $1`, id,
	).Scan(&d.ID, &d.OrganizationID, &d.CreatedBy, &d.Mode, &d.InitialIdea, &d.Answers,
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
		`UPDATE hu_drafts SET preview = $1, target_path = $2, updated_at = now()
		 WHERE id = $3`,
		previewJSON, preview.TargetPath, draftID,
	)
	return preview, nil
}

// Commit marca el draft como committed. NO escribe archivos; eso es trabajo
// del agente que consume el preview (Edit/Write). El Commit registra audit
// y bloquea Answer posterior.
func (s *Service) Commit(ctx context.Context, draftID uuid.UUID) (*Draft, error) {
	d, err := s.Get(ctx, draftID)
	if err != nil {
		return nil, err
	}
	if d.Status != StatusFinished {
		return nil, fmt.Errorf("%w: status=%s", ErrInvalidStatus, d.Status)
	}

	err = s.Pool.QueryRow(ctx, `
		UPDATE hu_drafts SET status = $1, committed_at = now(), updated_at = now()
		WHERE id = $2
		RETURNING id, organization_id, created_by, mode, initial_idea, answers,
		          current_step, total_steps, status, pending_clarifications,
		          preview, target_path, committed_at, expires_at, created_at, updated_at`,
		StatusCommitted, draftID,
	).Scan(&d.ID, &d.OrganizationID, &d.CreatedBy, &d.Mode, &d.InitialIdea, &d.Answers,
		&d.CurrentStep, &d.TotalSteps, &d.Status, &d.PendingClarifications,
		&d.Preview, &d.TargetPath, &d.CommittedAt, &d.ExpiresAt, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("commit draft: %w", err)
	}

	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
			ActorType:  audit.ActorSystem,
			Action:     "hu_draft.committed",
			EntityType: "hu_draft",
			EntityID:   &d.ID,
			NewValues:  map[string]any{"target_path": d.TargetPath},
		})
	}
	return d, nil
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
		`UPDATE hu_drafts SET status = $1, updated_at = now() WHERE id = $2`,
		StatusAbandoned, draftID,
	)
	if err != nil {
		return fmt.Errorf("abandon draft: %w", err)
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
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
	q := `SELECT id, organization_id, created_by, mode, initial_idea, answers,
	             current_step, total_steps, status, pending_clarifications,
	             preview, target_path, committed_at, expires_at, created_at, updated_at
	      FROM hu_drafts`
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
		if err := rows.Scan(&d.ID, &d.OrganizationID, &d.CreatedBy, &d.Mode, &d.InitialIdea, &d.Answers,
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
		`UPDATE hu_drafts SET status = $1, updated_at = now() WHERE id = $2`, status, id,
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
