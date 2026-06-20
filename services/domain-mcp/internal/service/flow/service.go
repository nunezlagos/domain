// Package flow — REQ-09 flow_system CRUD (issue-09.1 + 09.2).
//
// Un flow tiene un spec JSONB que define los steps en orden topológico.
// Step types soportados (issue-09.2):
//   - agent_run: ejecuta un agent por slug con inputs interpolados
//   - skill_run: ejecuta un skill por slug con args
//   - http_request: HTTP call (similar a skill type=api pero standalone)
//   - mem_save: persiste una observation en el project
//   - condition: branch if/then/else basado en expression simple
//   - parallel: ejecuta children en paralelo, espera todos
//   - wait_signal: pausa hasta que llegue un signal external (issue-09.8)
//
// La ejecución vive en internal/runner/flow (issue-09.3 state machine + 09.6 durable).
package flow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
)

var (
	ErrSlugInvalid     = errors.New("slug must be lowercase ascii, digits, dashes")
	ErrSlugTaken       = errors.New("slug already taken in this organization")
	ErrNameRequired    = errors.New("name required")
	ErrSpecInvalid     = errors.New("invalid flow spec")
	ErrNotFound        = errors.New("flow not found")
	ErrRunNotFound     = errors.New("flow run not found")
	ErrRunTerminal     = errors.New("flow run is in a terminal state")
	ErrInvalidPause    = errors.New("flow run is not running")
	ErrInvalidResume   = errors.New("flow run is not paused")
	ErrInvalidCancel   = errors.New("flow run is already terminal")
)

var reSlug = regexp.MustCompile(`^[a-z][a-z0-9-]{0,98}[a-z0-9]$|^[a-z]$`)
var reNonSlug = regexp.MustCompile(`[^a-z0-9-]+`)
var reMultiDash = regexp.MustCompile(`-{2,}`)

const (
	StepTypeAgentRun    = "agent_run"
	StepTypeSkillRun    = "skill_run"
	StepTypeHTTPRequest = "http_request"
	StepTypeMemSave     = "mem_save"
	StepTypeCondition   = "condition"
	StepTypeParallel    = "parallel"
	StepTypeWaitSignal  = "wait_signal"
	StepTypeSubFlow     = "sub_flow"
)

var validStepTypes = map[string]bool{
	StepTypeAgentRun: true, StepTypeSkillRun: true, StepTypeHTTPRequest: true,
	StepTypeMemSave: true, StepTypeCondition: true, StepTypeParallel: true,
	StepTypeWaitSignal: true, StepTypeSubFlow: true,
}

// StepRetryPolicy configura reintentos por step (issue-09.4).
// (RetryPolicy a secas ya existe en saga.go para compensaciones.)
type StepRetryPolicy struct {
	MaxRetries     int      `json:"max_retries" yaml:"max_retries"`
	Backoff        string   `json:"backoff,omitempty" yaml:"backoff,omitempty"` // "exponential" (default) | "fixed"
	InitialDelayMs int      `json:"initial_delay_ms,omitempty" yaml:"initial_delay_ms,omitempty"`
	FixedDelayMs   int      `json:"fixed_delay_ms,omitempty" yaml:"fixed_delay_ms,omitempty"`
	RetryOn        []string `json:"retry_on,omitempty" yaml:"retry_on,omitempty"` // vacío = todos los errores
}

// MaxRetriesCap — límite duro de reintentos por step (issue-09.4).
const MaxRetriesCap = 10

// Step en el DAG del flow.
type Step struct {
	ID          string         `json:"id" yaml:"id"`                               // identificador único en el flow
	Type        string         `json:"type" yaml:"type"`                           // ver validStepTypes
	Config      map[string]any `json:"config" yaml:"config"`                       // params específicos por type
	DependsOn   []string       `json:"depends_on,omitempty" yaml:"depends_on,omitempty"` // step ids de los que depende (DAG edge)
	OnError     string         `json:"on_error,omitempty" yaml:"on_error,omitempty"`     // "fail"/"abort_flow" (default) | "continue"/"ignore_and_continue" | "fallback_step" | step_id
	Retries     int            `json:"retries,omitempty" yaml:"retries,omitempty"`
	MaxBackoffS int            `json:"max_backoff_s,omitempty" yaml:"max_backoff_s,omitempty"` // issue-09.4 backoff cap en segundos (default 30s)
	TimeoutS    int            `json:"timeout_s,omitempty" yaml:"timeout_s,omitempty"`
	ReplaySafe  *bool          `json:"replay_safe,omitempty" yaml:"replay_safe,omitempty"` // issue-09.6: nil=true (safe to re-run on resume)
	Compensate  string         `json:"compensate,omitempty" yaml:"compensate,omitempty"`  // issue-09.9: referencia a skill/step de compensación

	// issue-09.4: retry policy rica (tiene precedencia sobre Retries legacy)
	Retry *StepRetryPolicy `json:"retry,omitempty" yaml:"retry,omitempty"`
	// DefaultOnError reemplaza el resultado cuando on_error=ignore_and_continue.
	DefaultOnError map[string]any `json:"default_on_error,omitempty" yaml:"default_on_error,omitempty"`
	// FallbackStep se ejecuta en lugar del step cuando on_error=fallback_step.
	FallbackStep *Step `json:"fallback_step,omitempty" yaml:"fallback_step,omitempty"`
}

// Spec del flow (deserializado del JSONB).
type Spec struct {
	Version int    `json:"version" yaml:"version"`
	Steps   []Step `json:"steps" yaml:"steps"`
	// DefaultStepErrorPolicy aplica cuando un step no declara on_error
	// (issue-09.4 escenario 8). La política del step tiene prioridad.
	DefaultStepErrorPolicy string `json:"default_step_error_policy,omitempty" yaml:"default_step_error_policy,omitempty"`
}

// Validate verifica el spec antes de persistirlo: step ids únicos, types
// válidos, on_error references válidos.
func (s Spec) Validate() error {
	if s.Version <= 0 {
		return fmt.Errorf("%w: spec.version required > 0", ErrSpecInvalid)
	}
	if len(s.Steps) == 0 {
		return fmt.Errorf("%w: at least 1 step required", ErrSpecInvalid)
	}
	ids := map[string]bool{}
	for i, step := range s.Steps {
		if step.ID == "" {
			return fmt.Errorf("%w: step[%d].id required", ErrSpecInvalid, i)
		}
		if ids[step.ID] {
			return fmt.Errorf("%w: duplicate step.id '%s'", ErrSpecInvalid, step.ID)
		}
		ids[step.ID] = true
		if !validStepTypes[step.Type] {
			return fmt.Errorf("%w: step '%s' type '%s' not valid", ErrSpecInvalid, step.ID, step.Type)
		}
	}
	// Verificar on_error referencias + reglas issue-09.4
	for _, step := range s.Steps {
		if err := validateErrorHandling(step, ids, 0); err != nil {
			return err
		}
	}
	if s.DefaultStepErrorPolicy != "" && !isNamedErrorPolicy(s.DefaultStepErrorPolicy) {
		return fmt.Errorf("%w: default_step_error_policy '%s' not valid",
			ErrSpecInvalid, s.DefaultStepErrorPolicy)
	}
	// Validar DAG: depends_on referencias + detección de ciclos
	if err := ValidateDAG(s.Steps); err != nil {
		return err
	}
	return nil
}

// MaxFallbackDepth — profundidad máxima de cadena de fallback_steps (issue-09.4).
const MaxFallbackDepth = 3

func isNamedErrorPolicy(p string) bool {
	switch p {
	case "fail", "abort_flow", "continue", "ignore_and_continue", "fallback_step":
		return true
	}
	return false
}

// validateErrorHandling aplica las reglas de issue-09.4 sobre un step
// (y recursivamente sobre su cadena de fallbacks).
func validateErrorHandling(step Step, ids map[string]bool, fallbackDepth int) error {
	if step.OnError != "" && !isNamedErrorPolicy(step.OnError) {
		if !ids[step.OnError] {
			return fmt.Errorf("%w: step '%s' on_error references unknown step '%s'",
				ErrSpecInvalid, step.ID, step.OnError)
		}
	}
	if step.OnError == "fallback_step" && step.FallbackStep == nil {
		return fmt.Errorf("%w: step '%s' on_error=fallback_step requires fallback_step",
			ErrSpecInvalid, step.ID)
	}
	if step.OnError == "ignore_and_continue" && step.DefaultOnError == nil {
		return fmt.Errorf("%w: step '%s' on_error=ignore_and_continue requires default_on_error",
			ErrSpecInvalid, step.ID)
	}
	maxRetries := step.Retries
	if step.Retry != nil {
		maxRetries = step.Retry.MaxRetries
		if b := step.Retry.Backoff; b != "" && b != "exponential" && b != "fixed" {
			return fmt.Errorf("%w: step '%s' retry.backoff '%s' not valid",
				ErrSpecInvalid, step.ID, b)
		}
	}
	if maxRetries > MaxRetriesCap {
		return fmt.Errorf("%w: step '%s' max_retries %d exceeds cap %d",
			ErrSpecInvalid, step.ID, maxRetries, MaxRetriesCap)
	}
	if step.FallbackStep != nil {
		if fallbackDepth+1 > MaxFallbackDepth {
			return fmt.Errorf("%w: step '%s' fallback chain exceeds %d levels",
				ErrSpecInvalid, step.ID, MaxFallbackDepth)
		}
		fb := *step.FallbackStep
		if fb.ID == "" {
			return fmt.Errorf("%w: step '%s' fallback_step.id required", ErrSpecInvalid, step.ID)
		}
		if !validStepTypes[fb.Type] {
			return fmt.Errorf("%w: fallback step '%s' type '%s' not valid",
				ErrSpecInvalid, fb.ID, fb.Type)
		}
		if err := validateErrorHandling(fb, ids, fallbackDepth+1); err != nil {
			return err
		}
	}
	return nil
}

type Flow struct {
	ID                  uuid.UUID
	Slug                string
	Name                string
	Description         string
	Spec                Spec
	IsActive            bool
	DeterministicReplay bool
	SeedManaged         bool
	SeedVersion         *int
	IsUserModified      bool
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type CreateInput struct {
	OrganizationID      uuid.UUID
	Slug                string
	Name                string
	Description         string
	Spec                Spec
	DeterministicReplay bool
	ActorID             uuid.UUID
}

type UpdateInput struct {
	Name        *string
	Description *string
	Spec        *Spec
	IsActive    *bool
	ActorID     uuid.UUID
	// ExpectedUpdatedAt habilita optimistic locking (issue-09.1): si no
	// coincide con flows.updated_at actual, Update retorna ErrUpdateConflict.
	ExpectedUpdatedAt *time.Time
}

// ErrUpdateConflict — el flow fue modificado por otro actor (412 en API).
var ErrUpdateConflict = errors.New("flow modified concurrently")

type Service struct {
	// Pool — DEPRECATED (HU-28.1). Strangler Fig: callers que construyen
	// &Service{Pool: ...} siguen funcionando; otros archivos del package
	// (saga.go, signals.go, snapshots.go, etc.) aún usan Pool directo y se
	// migrarán en HUs futuras.
	Pool  *pgxpool.Pool
	Audit audit.Recorder

	repo Repository
}

// NewService construye el Service con dependencias explícitas.
func NewService(pool *pgxpool.Pool, audit audit.Recorder, repo Repository) *Service {
	if repo == nil && pool != nil {
		repo = NewPgRepository(pool)
	}
	return &Service{Pool: pool, Audit: audit, repo: repo}
}

// repository devuelve el repo inyectado o crea uno on-demand desde Pool
// (compat con struct literal).
func (s *Service) repository() Repository {
	if s.repo != nil {
		return s.repo
	}
	s.repo = NewPgRepository(s.Pool)
	return s.repo
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*Flow, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, ErrNameRequired
	}
	if in.Slug == "" {
		in.Slug = generateSlug(in.Name)
	}
	if !reSlug.MatchString(in.Slug) {
		return nil, ErrSlugInvalid
	}
	if err := in.Spec.Validate(); err != nil {
		return nil, err
	}
	specJSON, _ := json.Marshal(in.Spec)

	f, err := s.repository().InsertFlow(ctx, InsertFlowParams{
		OrganizationID:      in.OrganizationID,
		Slug:                in.Slug,
		Name:                in.Name,
		Description:         in.Description,
		SpecJSON:            specJSON,
		DeterministicReplay: in.DeterministicReplay,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return nil, ErrSlugTaken
		}
		return nil, fmt.Errorf("insert flow: %w", err)
	}
	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			OrganizationID: &in.OrganizationID,
			ActorID:        &in.ActorID,
			ActorType:      audit.ActorUser,
			Action:         "flow.created",
			EntityType:     "flow",
			EntityID:       &f.ID,
			NewValues:      map[string]any{"slug": f.Slug, "steps_count": len(f.Spec.Steps)},
		})
	}
	return f, nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*Flow, error) {
	prev, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	name := prev.Name
	if in.Name != nil {
		name = *in.Name
	}
	desc := prev.Description
	if in.Description != nil {
		desc = *in.Description
	}
	spec := prev.Spec
	if in.Spec != nil {
		if err := in.Spec.Validate(); err != nil {
			return nil, err
		}
		spec = *in.Spec
	}
	isActive := prev.IsActive
	if in.IsActive != nil {
		isActive = *in.IsActive
	}
	userMod := prev.IsUserModified || prev.SeedManaged
	specJSON, _ := json.Marshal(spec)

	// issue-09.1 optimistic locking: la condición updated_at en repo garantiza
	// que no pisamos una modificación concurrente.
	f, err := s.repository().UpdateFlow(ctx, UpdateFlowParams{
		ID:                id,
		Name:              name,
		Description:       desc,
		SpecJSON:          specJSON,
		IsActive:          isActive,
		IsUserModified:    userMod,
		ExpectedUpdatedAt: in.ExpectedUpdatedAt,
	})
	if err != nil {
		return nil, err
	}
	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorID:        &in.ActorID,
			ActorType:      audit.ActorUser,
			Action:         "flow.updated",
			EntityType:     "flow",
			EntityID:       &f.ID,
		})
	}
	return f, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Flow, error) {
	return s.repository().GetFlowByID(ctx, id)
}

func (s *Service) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*Flow, error) {
	return s.repository().GetFlowBySlug(ctx, orgID, slug)
}

func (s *Service) List(ctx context.Context, orgID uuid.UUID, limit int) ([]Flow, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.repository().ListFlows(ctx, orgID, limit)
}

// ListParents devuelve los flows de la org que referencian a slug como
// sub_flow en su spec (issue-09.5, GET /flows/:id/parents).
func (s *Service) ListParents(ctx context.Context, orgID uuid.UUID, slug string) ([]Flow, error) {
	return s.repository().ListParents(ctx, orgID, slug)
}

func (s *Service) SoftDelete(ctx context.Context, id, actorID uuid.UUID) error {
	if _, err := s.GetByID(ctx, id); err != nil {
		return err
	}
	if err := s.repository().SoftDeleteFlow(ctx, id); err != nil {
		return err
	}
	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorID:        &actorID,
			ActorType:      audit.ActorUser,
			Action:         "flow.deleted",
			EntityType:     "flow",
			EntityID:       &id,
		})
	}
	return nil
}

// specJSONRaw helper scan: JSONB → Spec
type specJSONRaw struct {
	target *Spec
}

func (s *specJSONRaw) Scan(src any) error {
	if src == nil {
		return nil
	}
	var raw []byte
	switch v := src.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("specJSONRaw: unsupported type %T", src)
	}
	return json.Unmarshal(raw, s.target)
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// generateSlug crea un slug URL-friendly a partir de un nombre.
// Convierte a lowercase, reemplaza espacios y no-ascii por guiones,
// elimina guiones duplicados y trimea.
func generateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = reNonSlug.ReplaceAllString(slug, "-")
	slug = reMultiDash.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 100 {
		slug = slug[:100]
	}
	slug = strings.TrimRight(slug, "-")
	if slug == "" {
		slug = "flow"
	}
	return slug
}

// RunRow represents a row from flow_runs table.
type RunRow struct {
	ID             uuid.UUID
	FlowID         uuid.UUID
	Status         string
	Error          string
	StartedAt     *time.Time
	FinishedAt    *time.Time
	TriggeredBy   *uuid.UUID
	TriggerType   string
}

// GetRun loads a flow run by ID.
func (s *Service) GetRun(ctx context.Context, id uuid.UUID) (*RunRow, error) {
	return s.repository().GetRun(ctx, id)
}

// StepRow es la vista de un step para GET /flow-runs/:id (issue-09.3/09.10).
type StepRow struct {
	ID              uuid.UUID  `json:"id"`
	StepKey         string     `json:"step_key"`
	Status          string     `json:"status"`
	Progress        *float64   `json:"progress,omitempty"`
	ProgressMessage *string    `json:"progress_message,omitempty"`
	Error           *string    `json:"error,omitempty"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
}

// GetRunSteps lista los steps de un run con su progreso.
func (s *Service) GetRunSteps(ctx context.Context, runID uuid.UUID) ([]StepRow, error) {
	return s.repository().GetRunSteps(ctx, runID)
}

// PauseRun transitions a running flow run to paused.
func (s *Service) PauseRun(ctx context.Context, id uuid.UUID) error {
	m := NewFlowStateMachine()
	run, err := s.GetRun(ctx, id)
	if err != nil {
		return err
	}
	if err := m.ValidateTransition(FlowStatus(run.Status), FlowStatusPaused); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidPause, err.Error())
	}
	return s.repository().PauseRun(ctx, id, run.Status)
}

// ResumeRun transitions a paused flow run back to running.
func (s *Service) ResumeRun(ctx context.Context, id uuid.UUID) error {
	m := NewFlowStateMachine()
	run, err := s.GetRun(ctx, id)
	if err != nil {
		return err
	}
	if err := m.ValidateTransition(FlowStatus(run.Status), FlowStatusRunning); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidResume, err.Error())
	}
	return s.repository().ResumeRun(ctx, id)
}

// CancelRun transitions any non-terminal flow run to cancelled.
func (s *Service) CancelRun(ctx context.Context, id uuid.UUID) error {
	m := NewFlowStateMachine()
	run, err := s.GetRun(ctx, id)
	if err != nil {
		return err
	}
	if m.IsTerminal(FlowStatus(run.Status)) {
		return ErrInvalidCancel
	}
	if err := m.ValidateTransition(FlowStatus(run.Status), FlowStatusCancelled); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidCancel, err.Error())
	}
	return s.repository().CancelRun(ctx, id, time.Now().UTC())
}

// RunFilter specifies optional run list filters.
type RunFilter struct {
	OrgID  uuid.UUID
	FlowID *uuid.UUID
	Limit  int
	Offset int
}

// ListRuns lists flow runs with optional filters.
func (s *Service) ListRuns(ctx context.Context, f RunFilter) ([]RunRow, int, error) {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 50
	}
	return s.repository().ListRuns(ctx, f)
}
