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
	"github.com/jackc/pgx/v5"
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

// Step en el DAG del flow.
type Step struct {
	ID          string         `json:"id" yaml:"id"`                               // identificador único en el flow
	Type        string         `json:"type" yaml:"type"`                           // ver validStepTypes
	Config      map[string]any `json:"config" yaml:"config"`                       // params específicos por type
	DependsOn   []string       `json:"depends_on,omitempty" yaml:"depends_on,omitempty"` // step ids de los que depende (DAG edge)
	OnError     string         `json:"on_error,omitempty" yaml:"on_error,omitempty"`     // "fail" (default) | "continue" | step_id
	Retries     int            `json:"retries,omitempty" yaml:"retries,omitempty"`
	MaxBackoffS int            `json:"max_backoff_s,omitempty" yaml:"max_backoff_s,omitempty"` // issue-09.4 backoff cap en segundos (default 30s)
	TimeoutS    int            `json:"timeout_s,omitempty" yaml:"timeout_s,omitempty"`
	ReplaySafe  *bool          `json:"replay_safe,omitempty" yaml:"replay_safe,omitempty"` // issue-09.6: nil=true (safe to re-run on resume)
	Compensate  string         `json:"compensate,omitempty" yaml:"compensate,omitempty"`  // issue-09.9: referencia a skill/step de compensación
}

// Spec del flow (deserializado del JSONB).
type Spec struct {
	Version int    `json:"version" yaml:"version"`
	Steps   []Step `json:"steps" yaml:"steps"`
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
	// Verificar on_error referencias
	for _, step := range s.Steps {
		if step.OnError != "" && step.OnError != "fail" && step.OnError != "continue" {
			if !ids[step.OnError] {
				return fmt.Errorf("%w: step '%s' on_error references unknown step '%s'",
					ErrSpecInvalid, step.ID, step.OnError)
			}
		}
	}
	// Validar DAG: depends_on referencias + detección de ciclos
	if err := ValidateDAG(s.Steps); err != nil {
		return err
	}
	return nil
}

type Flow struct {
	ID                  uuid.UUID
	OrganizationID      uuid.UUID
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
}

type Service struct {
	Pool  *pgxpool.Pool
	Audit audit.Recorder
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

	var f Flow
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO flows
		   (organization_id, slug, name, description, spec, deterministic_replay)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, organization_id, slug, name, COALESCE(description,''),
		           spec, is_active, deterministic_replay, seed_managed, seed_version,
		           is_user_modified, created_at, updated_at`,
		in.OrganizationID, in.Slug, in.Name, nullStr(in.Description), specJSON,
		in.DeterministicReplay,
	).Scan(&f.ID, &f.OrganizationID, &f.Slug, &f.Name, &f.Description,
		&specJSONRaw{&f.Spec}, &f.IsActive, &f.DeterministicReplay,
		&f.SeedManaged, &f.SeedVersion, &f.IsUserModified, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "flows_organization_id_slug_key") ||
			strings.Contains(err.Error(), "duplicate key") {
			return nil, ErrSlugTaken
		}
		return nil, fmt.Errorf("insert flow: %w", err)
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
			OrganizationID: &in.OrganizationID,
			ActorID:        &in.ActorID,
			ActorType:      audit.ActorUser,
			Action:         "flow.created",
			EntityType:     "flow",
			EntityID:       &f.ID,
			NewValues:      map[string]any{"slug": f.Slug, "steps_count": len(f.Spec.Steps)},
		})
	}
	return &f, nil
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

	var f Flow
	err = s.Pool.QueryRow(ctx,
		`UPDATE flows SET name = $2, description = $3, spec = $4, is_active = $5,
		    is_user_modified = $6
		 WHERE id = $1 AND deleted_at IS NULL
		 RETURNING id, organization_id, slug, name, COALESCE(description,''),
		           spec, is_active, deterministic_replay, seed_managed, seed_version,
		           is_user_modified, created_at, updated_at`,
		id, name, nullStr(desc), specJSON, isActive, userMod,
	).Scan(&f.ID, &f.OrganizationID, &f.Slug, &f.Name, &f.Description,
		&specJSONRaw{&f.Spec}, &f.IsActive, &f.DeterministicReplay,
		&f.SeedManaged, &f.SeedVersion, &f.IsUserModified, &f.CreatedAt, &f.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update flow: %w", err)
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
			OrganizationID: &f.OrganizationID,
			ActorID:        &in.ActorID,
			ActorType:      audit.ActorUser,
			Action:         "flow.updated",
			EntityType:     "flow",
			EntityID:       &f.ID,
		})
	}
	return &f, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Flow, error) {
	return s.queryOne(ctx, `WHERE id = $1 AND deleted_at IS NULL`, id)
}

func (s *Service) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*Flow, error) {
	return s.queryOne(ctx,
		`WHERE organization_id = $1 AND slug = $2 AND deleted_at IS NULL`, orgID, slug)
}

func (s *Service) List(ctx context.Context, orgID uuid.UUID, limit int) ([]Flow, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.Pool.Query(ctx,
		`SELECT id, organization_id, slug, name, COALESCE(description,''),
		        spec, is_active, deterministic_replay, seed_managed, seed_version,
		        is_user_modified, created_at, updated_at
		 FROM flows WHERE organization_id = $1 AND deleted_at IS NULL
		 ORDER BY created_at DESC LIMIT $2`, orgID, limit)
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}
	defer rows.Close()
	var out []Flow
	for rows.Next() {
		var f Flow
		if err := rows.Scan(&f.ID, &f.OrganizationID, &f.Slug, &f.Name, &f.Description,
			&specJSONRaw{&f.Spec}, &f.IsActive, &f.DeterministicReplay,
			&f.SeedManaged, &f.SeedVersion, &f.IsUserModified, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Service) SoftDelete(ctx context.Context, id, actorID uuid.UUID) error {
	prev, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx,
		`UPDATE flows SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("soft delete: %w", err)
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
			OrganizationID: &prev.OrganizationID,
			ActorID:        &actorID,
			ActorType:      audit.ActorUser,
			Action:         "flow.deleted",
			EntityType:     "flow",
			EntityID:       &id,
		})
	}
	return nil
}

func (s *Service) queryOne(ctx context.Context, where string, args ...any) (*Flow, error) {
	var f Flow
	q := `SELECT id, organization_id, slug, name, COALESCE(description,''),
	        spec, is_active, deterministic_replay, seed_managed, seed_version,
	        is_user_modified, created_at, updated_at
	      FROM flows ` + where
	err := s.Pool.QueryRow(ctx, q, args...).Scan(
		&f.ID, &f.OrganizationID, &f.Slug, &f.Name, &f.Description,
		&specJSONRaw{&f.Spec}, &f.IsActive, &f.DeterministicReplay,
		&f.SeedManaged, &f.SeedVersion, &f.IsUserModified, &f.CreatedAt, &f.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	return &f, nil
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
	OrganizationID uuid.UUID
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
	var r RunRow
	err := s.Pool.QueryRow(ctx, `
		SELECT id, organization_id, flow_id, status, COALESCE(error,''),
		       started_at, finished_at, triggered_by, trigger_type
		FROM flow_runs WHERE id = $1`, id,
	).Scan(&r.ID, &r.OrganizationID, &r.FlowID, &r.Status, &r.Error,
		&r.StartedAt, &r.FinishedAt, &r.TriggeredBy, &r.TriggerType)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRunNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get run: %w", err)
	}
	return &r, nil
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
	_, err = s.Pool.Exec(ctx,
		`UPDATE flow_runs SET status = 'paused', worker_id = NULL WHERE id = $1 AND status = $2`,
		id, run.Status)
	if err != nil {
		return fmt.Errorf("pause run: %w", err)
	}
	return nil
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
	_, err = s.Pool.Exec(ctx,
		`UPDATE flow_runs SET status = 'running' WHERE id = $1 AND (status = 'paused' OR status = 'paused_awaiting_human' OR status = 'paused_awaiting_signal' OR status = 'paused_error')`,
		id)
	if err != nil {
		return fmt.Errorf("resume run: %w", err)
	}
	return nil
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
	now := time.Now().UTC()
	_, err = s.Pool.Exec(ctx,
		`UPDATE flow_runs SET status = 'cancelled', worker_id = NULL, finished_at = $1, error = COALESCE(error, '') || ' cancelled by user' WHERE id = $2`,
		now, id)
	if err != nil {
		return fmt.Errorf("cancel run: %w", err)
	}
	return nil
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
	where := "WHERE organization_id = $1"
	args := []any{f.OrgID}
	argIdx := 2
	if f.FlowID != nil {
		where += fmt.Sprintf(" AND flow_id = $%d", argIdx)
		args = append(args, *f.FlowID)
		argIdx++
	}
	var total int
	err := s.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM flow_runs `+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count runs: %w", err)
	}
	where += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, f.Limit, f.Offset)
	rows, err := s.Pool.Query(ctx, `
		SELECT id, organization_id, flow_id, status, COALESCE(error,''),
		       started_at, finished_at, triggered_by, trigger_type
		FROM flow_runs `+where, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list runs: %w", err)
	}
	defer rows.Close()
	var out []RunRow
	for rows.Next() {
		var r RunRow
		if err := rows.Scan(&r.ID, &r.OrganizationID, &r.FlowID, &r.Status, &r.Error,
			&r.StartedAt, &r.FinishedAt, &r.TriggeredBy, &r.TriggerType); err != nil {
			return nil, 0, fmt.Errorf("scan: %w", err)
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}
