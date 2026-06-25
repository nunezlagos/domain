package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/service/orchestrator/modes"
	"nunezlagos/domain/internal/service/orchestrator/phases"
)

// ErrFlowNotSeeded indica que la org no tiene el flow sdd-pipeline-v1
// seedeado. El caller debe correr SeedFlowsForOrg primero (cron de
// onboarding, manual bootstrap) antes de invocar el orquestador.
var ErrFlowNotSeeded = errors.New("orchestrator: flow sdd-pipeline-v1 not seeded for org — run SeedFlowsForOrg first")

// Repository es la capa de persistencia del orquestador. La interface
// permite inyectar fakes en tests sin tocar DB.
//
// El service la usa para:
//   - resolver el flow_id a partir del slug canónico + org
//   - crear el flow_run que ata todo el OrchestrateInput a una traza
//     persistible (audit + heartbeat-watcher + orphan-runs-audit lo
//     consultan vía flow_runs.metadata.orchestrator_run_id)
//   - crear los flow_run_steps en pending, uno por PhaseStep del Plan
//   - leer/actualizar steps cuando el cliente IDE reporta phase_result
//     vía MCP (mcp-002)
//   - leer flow_run + steps para responder a domain_flow_status (mcp-004)
type Repository interface {
	GetFlowIDBySlug(ctx context.Context, orgID uuid.UUID, slug string) (uuid.UUID, error)
	CreateFlowRun(ctx context.Context, in FlowRunInsert) error
	CreateFlowRunStep(ctx context.Context, in FlowRunStepInsert) error
	GetFlowRun(ctx context.Context, id uuid.UUID) (*FlowRunRow, error)
	GetFlowRunStep(ctx context.Context, id uuid.UUID) (*FlowRunStepRow, error)
	ListFlowRunSteps(ctx context.Context, flowRunID uuid.UUID) ([]FlowRunStepRow, error)
	MarkStepCompleted(ctx context.Context, stepID uuid.UUID, outputs map[string]any) error
	MarkStepFailed(ctx context.Context, stepID uuid.UUID, errorMsg string) error


	SetFlowRunError(ctx context.Context, flowRunID uuid.UUID, errorMsg string) error
	UpdateFlowRunStatus(ctx context.Context, flowRunID uuid.UUID, status string) error




	UpdateStepInputs(ctx context.Context, stepID uuid.UUID, inputs map[string]any) error




	MarkStepBlocked(ctx context.Context, stepID uuid.UUID, reason string) error
	MarkStepPending(ctx context.Context, stepID uuid.UUID) error
	MarkStepCancelled(ctx context.Context, stepID uuid.UUID) error




	GetAgentTemplateSystemPrompt(ctx context.Context, orgID uuid.UUID, slug string) (string, error)




	GetAgentTemplate(ctx context.Context, orgID uuid.UUID, slug string) (*AgentTemplate, error)
}

// AgentTemplate es la vista de lectura desde agent_templates para Solo
// mode. Coincide con SeedAgentTemplatesForOrg + customizaciones del
// operador (is_user_modified=true). El provider se infiere desde el
// prefijo del Model name por convención (claude-* → anthropic, etc.).
type AgentTemplate struct {
	Slug         string
	Model        string
	Temperature  float32
	MaxTokens    int
	SystemPrompt string


	Metadata map[string]any
}

// SkillThreshold extrae el threshold de metadata (D3). Retorna 0 si
// no está configurado o si Metadata es nil.
func (t *AgentTemplate) SkillThreshold() float64 {
	if t.Metadata == nil {
		return 0
	}
	v, ok := t.Metadata["skill_threshold"]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	}
	return 0
}

// FlowRunRow es la vista de lectura completa de un flow_run.
type FlowRunRow struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	ProjectID      uuid.UUID
	FlowID         uuid.UUID
	Status         string
	ExecMode       string
	Hardspec       bool
	Cursor         map[string]any
	StartedAt      *time.Time
	FinishedAt     *time.Time
}

// FlowRunStepRow es la vista de lectura de un flow_run_step.
type FlowRunStepRow struct {
	ID         uuid.UUID
	FlowRunID  uuid.UUID
	StepKey    string
	Status     string
	Inputs     map[string]any
	Outputs    map[string]any
	Error      string
	Attempt    int
	StartedAt  *time.Time
	CompletedAt *time.Time
}

// FlowRunInsert es el shape que el repo persiste. El service compone
// el metadata JSONB con orchestrator_run_id + mode + raw_text para que
// las auditorías post-mortem puedan reconstruir el contexto sin
// indexar fields nuevos.
type FlowRunInsert struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	ProjectID       uuid.UUID
	FlowID          uuid.UUID
	TriggeredBy     uuid.UUID
	Status          string
	ExecMode        string
	Hardspec        bool
	Inputs          map[string]any
	Metadata        map[string]any
	StartedAt       time.Time
}

// FlowRunStepInsert es un step persistido en pending. Outputs queda
// NULL hasta que el cliente reporte phase_result (mcp-002).
type FlowRunStepInsert struct {
	ID         uuid.UUID
	FlowRunID  uuid.UUID
	StepKey    string
	Status     string
	Inputs     map[string]any
}

// pgRepository implementa Repository contra Postgres. Usa pgx directamente
// porque las queries son específicas del orquestador (no las saca el flow
// service general).
type pgRepository struct {
	pool *pgxpool.Pool
}

// NewPGRepository devuelve un Repository persistido en Postgres.
func NewPGRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

// ErrAgentTemplateNotFound: el slug no está seedeado en la org. El caller
// debe correr SeedAgentTemplatesForOrg primero (catálogo v3).
var ErrAgentTemplateNotFound = errors.New("orchestrator: agent_template not seeded for org")

func (r *pgRepository) GetAgentTemplateSystemPrompt(ctx context.Context, orgID uuid.UUID, slug string) (string, error) {

	_ = orgID
	var prompt string
	err := r.pool.QueryRow(ctx, `
		SELECT system_prompt FROM agent_templates
		WHERE slug = $1
		LIMIT 1`, slug,
	).Scan(&prompt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("%w: slug=%s", ErrAgentTemplateNotFound, slug)
		}
		return "", fmt.Errorf("get agent_template system_prompt: %w", err)
	}
	return prompt, nil
}

func (r *pgRepository) GetAgentTemplate(ctx context.Context, orgID uuid.UUID, slug string) (*AgentTemplate, error) {
	_ = orgID
	var t AgentTemplate
	t.Slug = slug
	var metaJSON []byte
	err := r.pool.QueryRow(ctx, `
		SELECT model, temperature, max_tokens, system_prompt, metadata
		FROM agent_templates
		WHERE slug = $1
		LIMIT 1`, slug,
	).Scan(&t.Model, &t.Temperature, &t.MaxTokens, &t.SystemPrompt, &metaJSON)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: slug=%s", ErrAgentTemplateNotFound, slug)
		}
		return nil, fmt.Errorf("get agent_template: %w", err)
	}
	if len(metaJSON) > 0 {
		_ = json.Unmarshal(metaJSON, &t.Metadata)
	}
	return &t, nil
}

func (r *pgRepository) GetFlowIDBySlug(ctx context.Context, orgID uuid.UUID, slug string) (uuid.UUID, error) {
	_ = orgID
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, `
		SELECT id FROM flows
		WHERE slug = $1 AND deleted_at IS NULL
		LIMIT 1`, slug,
	).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, ErrFlowNotSeeded
		}
		return uuid.Nil, fmt.Errorf("get flow id: %w", err)
	}
	return id, nil
}

func (r *pgRepository) CreateFlowRun(ctx context.Context, in FlowRunInsert) error {
	inputsJSON, err := json.Marshal(in.Inputs)
	if err != nil {
		return fmt.Errorf("marshal inputs: %w", err)
	}
	metadataJSON, err := json.Marshal(in.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	execMode := in.ExecMode
	if execMode == "" {
		execMode = "auto"
	}


	_, err = r.pool.Exec(ctx, `
		INSERT INTO flow_runs
		  (id, flow_id, project_id, triggered_by, trigger_type, status,
		   exec_mode, hardspec, inputs, cursor, started_at)
		VALUES ($1,$2,$3,$4,'manual',$5,$6,$7,$8,$9,$10)`,
		in.ID, in.FlowID, nullUUID(in.ProjectID), nullUUID(in.TriggeredBy),
		in.Status, execMode, in.Hardspec, inputsJSON, metadataJSON, in.StartedAt)
	if err != nil {
		return fmt.Errorf("insert flow_run: %w", err)
	}
	return nil
}

func (r *pgRepository) GetFlowRun(ctx context.Context, id uuid.UUID) (*FlowRunRow, error) {
	var (
		row        FlowRunRow
		cursorRaw  []byte
		startedAt  *time.Time
		finishedAt *time.Time
		projectID  *uuid.UUID
	)

	err := r.pool.QueryRow(ctx, `
		SELECT id, flow_id, project_id, status, COALESCE(exec_mode,'auto'),
		       COALESCE(hardspec,false), cursor, started_at, finished_at
		FROM flow_runs WHERE id = $1`, id,
	).Scan(&row.ID, &row.FlowID, &projectID, &row.Status, &row.ExecMode,
		&row.Hardspec, &cursorRaw, &startedAt, &finishedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrFlowRunNotFound
		}
		return nil, fmt.Errorf("get flow_run: %w", err)
	}
	if projectID != nil {
		row.ProjectID = *projectID
	}
	row.StartedAt = startedAt
	row.FinishedAt = finishedAt
	if len(cursorRaw) > 0 {
		_ = json.Unmarshal(cursorRaw, &row.Cursor)
	}
	return &row, nil
}

func (r *pgRepository) GetFlowRunStep(ctx context.Context, id uuid.UUID) (*FlowRunStepRow, error) {
	var (
		row         FlowRunStepRow
		inputsRaw   []byte
		outputsRaw  []byte
		errorStr    *string
		startedAt   *time.Time
		completedAt *time.Time
	)
	err := r.pool.QueryRow(ctx, `
		SELECT id, flow_run_id, step_key, status, inputs, outputs, error,
		       attempt, started_at, completed_at
		FROM flow_run_steps WHERE id = $1`, id,
	).Scan(&row.ID, &row.FlowRunID, &row.StepKey, &row.Status,
		&inputsRaw, &outputsRaw, &errorStr, &row.Attempt,
		&startedAt, &completedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrFlowRunStepNotFound
		}
		return nil, fmt.Errorf("get flow_run_step: %w", err)
	}
	if len(inputsRaw) > 0 {
		_ = json.Unmarshal(inputsRaw, &row.Inputs)
	}
	if len(outputsRaw) > 0 {
		_ = json.Unmarshal(outputsRaw, &row.Outputs)
	}
	if errorStr != nil {
		row.Error = *errorStr
	}
	row.StartedAt = startedAt
	row.CompletedAt = completedAt
	return &row, nil
}

func (r *pgRepository) ListFlowRunSteps(ctx context.Context, flowRunID uuid.UUID) ([]FlowRunStepRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, flow_run_id, step_key, status, inputs, outputs, error,
		       attempt, started_at, completed_at
		FROM flow_run_steps
		WHERE flow_run_id = $1
		ORDER BY created_at`, flowRunID)
	if err != nil {
		return nil, fmt.Errorf("list flow_run_steps: %w", err)
	}
	defer rows.Close()
	var out []FlowRunStepRow
	for rows.Next() {
		var (
			row         FlowRunStepRow
			inputsRaw   []byte
			outputsRaw  []byte
			errorStr    *string
			startedAt   *time.Time
			completedAt *time.Time
		)
		if err := rows.Scan(&row.ID, &row.FlowRunID, &row.StepKey, &row.Status,
			&inputsRaw, &outputsRaw, &errorStr, &row.Attempt,
			&startedAt, &completedAt); err != nil {
			return nil, fmt.Errorf("scan step: %w", err)
		}
		if len(inputsRaw) > 0 {
			_ = json.Unmarshal(inputsRaw, &row.Inputs)
		}
		if len(outputsRaw) > 0 {
			_ = json.Unmarshal(outputsRaw, &row.Outputs)
		}
		if errorStr != nil {
			row.Error = *errorStr
		}
		row.StartedAt = startedAt
		row.CompletedAt = completedAt
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *pgRepository) MarkStepCompleted(ctx context.Context, stepID uuid.UUID, outputs map[string]any) error {
	outputsJSON, err := json.Marshal(outputs)
	if err != nil {
		return fmt.Errorf("marshal outputs: %w", err)
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE flow_run_steps
		SET status = 'completed',
		    outputs = $2,
		    completed_at = NOW()
		WHERE id = $1
		  AND status IN ('pending','running')`,
		stepID, outputsJSON)
	if err != nil {
		return fmt.Errorf("update step completed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrFlowRunStepNotPending
	}
	return nil
}

func (r *pgRepository) MarkStepFailed(ctx context.Context, stepID uuid.UUID, errorMsg string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE flow_run_steps
		SET status = 'failed',
		    error = $2,
		    completed_at = NOW()
		WHERE id = $1
		  AND status IN ('pending','running','blocked')`,
		stepID, errorMsg)
	if err != nil {
		return fmt.Errorf("update step failed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrFlowRunStepNotPending
	}
	return nil
}

func (r *pgRepository) MarkStepBlocked(ctx context.Context, stepID uuid.UUID, reason string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE flow_run_steps SET status = 'blocked', error = $2
		WHERE id = $1 AND status IN ('pending','running')`,
		stepID, reason)
	if err != nil {
		return fmt.Errorf("mark step blocked: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrFlowRunStepNotPending
	}
	return nil
}

func (r *pgRepository) MarkStepPending(ctx context.Context, stepID uuid.UUID) error {


	tag, err := r.pool.Exec(ctx, `
		UPDATE flow_run_steps SET status = 'pending', error = NULL
		WHERE id = $1 AND status = 'blocked'`,
		stepID)
	if err != nil {
		return fmt.Errorf("mark step pending: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errors.New("orchestrator: step is not blocked (only blocked steps can be reactivated)")
	}
	return nil
}

func (r *pgRepository) MarkStepCancelled(ctx context.Context, stepID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE flow_run_steps SET status = 'cancelled', completed_at = NOW()
		WHERE id = $1 AND status IN ('pending','running','blocked')`,
		stepID)
	if err != nil {
		return fmt.Errorf("mark step cancelled: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrFlowRunStepNotPending
	}
	return nil
}

func (r *pgRepository) UpdateStepInputs(ctx context.Context, stepID uuid.UUID, inputs map[string]any) error {
	inputsJSON, err := json.Marshal(inputs)
	if err != nil {
		return fmt.Errorf("marshal step inputs: %w", err)
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE flow_run_steps SET inputs = $2 WHERE id = $1`,
		stepID, inputsJSON)
	if err != nil {
		return fmt.Errorf("update step inputs: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrFlowRunStepNotFound
	}
	return nil
}

func (r *pgRepository) UpdateFlowRunStatus(ctx context.Context, flowRunID uuid.UUID, status string) error {
	finishedAtClause := ""
	if status == "completed" || status == "failed" || status == "cancelled" {
		finishedAtClause = ", finished_at = NOW()"
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE flow_runs
		SET status = $2`+finishedAtClause+`
		WHERE id = $1`,
		flowRunID, status)
	if err != nil {
		return fmt.Errorf("update flow_run status: %w", err)
	}
	return nil
}

// SetFlowRunError persiste el motivo del fallo en flow_runs.error.
func (r *pgRepository) SetFlowRunError(ctx context.Context, flowRunID uuid.UUID, errorMsg string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE flow_runs SET error = $2 WHERE id = $1`, flowRunID, errorMsg)
	if err != nil {
		return fmt.Errorf("set flow_run error: %w", err)
	}
	return nil
}

func (r *pgRepository) CreateFlowRunStep(ctx context.Context, in FlowRunStepInsert) error {
	inputsJSON, err := json.Marshal(in.Inputs)
	if err != nil {
		return fmt.Errorf("marshal step inputs: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO flow_run_steps
		  (id, flow_run_id, step_key, status, inputs)
		VALUES ($1,$2,$3,$4,$5)`,
		in.ID, in.FlowRunID, in.StepKey, in.Status, inputsJSON)
	if err != nil {
		return fmt.Errorf("insert flow_run_step %s: %w", in.StepKey, err)
	}
	return nil
}

// nullUUID convierte uuid.Nil → nil interface para que pgx persista NULL.
// Usado para triggered_by cuando no hay user_id (cron/system runs).
func nullUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}

// persistPlan toma un PhasePlan ya construido por modes.* y crea los
// rows flow_run + flow_run_steps en DB en ese orden (FK → flow_run debe
// existir primero). El service llama esta función después de validar
// inputs y resolver el flow_id.
//
// metadata.cursor empieza vacío; el dispatcher lo actualiza vía
// MCP cuando avanza fases.
func (s *Service) persistPlan(ctx context.Context, in OrchestrateInput, mode Mode,
	orchestratorRunID, flowID, flowRunID uuid.UUID, plan *modes.PhasePlan, now time.Time,
) error {
	if s.Repo == nil {
		return errors.New("orchestrator: Repo not configured")
	}
	if err := s.Repo.CreateFlowRun(ctx, FlowRunInsert{
		ID:             flowRunID,
		OrganizationID: in.OrganizationID,
		ProjectID:      in.ProjectID,
		FlowID:         flowID,
		TriggeredBy:    in.UserID,
		Status:         "pending",
		ExecMode:       in.ExecMode,
		Hardspec:       in.Hardspec,
		Inputs:         map[string]any{"raw_text": in.RawText},
		Metadata: map[string]any{
			"orchestrator_run_id": orchestratorRunID.String(),
			"mode":                string(mode),
			"raw_text":            in.RawText,
			"express_max_lines":   in.ExpressMaxLines,
		},
		StartedAt: now,
	}); err != nil {
		return err
	}
	expressMax := in.ExpressMaxLines
	if expressMax <= 0 {
		expressMax = 10 // RFC 0006 D1 default
	}
	for _, step := range plan.Steps {
		if err := s.Repo.CreateFlowRunStep(ctx, FlowRunStepInsert{
			ID:        step.ID,
			FlowRunID: flowRunID,
			StepKey:   string(step.Slug),
			Status:    "pending",
			Inputs: map[string]any{
				"agent_template_slug": step.AgentTemplateSlug,
				"system_prompt":       step.SystemPrompt,
				"user_prompt":         step.UserPrompt,
				"suggested_saves":     toAnySuggestedSaves(step.SuggestedSaves),
				"retry_policy":        string(step.RetryPolicy),
				"skill_threshold":     step.SkillThreshold,



				"raw_text": in.RawText,



				"mode":              string(mode),
				"express_max_lines": expressMax,
			},
		}); err != nil {
			return err
		}
	}
	return nil
}

// toAnySuggestedSaves convierte el slice tipado a []map[string]any
// para que el JSONB resultante sea inspeccionable sin acoplar al
// shape de phases.SuggestedSave.
func toAnySuggestedSaves(saves []phases.SuggestedSave) []map[string]any {
	out := make([]map[string]any, len(saves))
	for i, s := range saves {
		out[i] = map[string]any{
			"type":     s.Type,
			"required": s.Required,
			"hint":     s.Hint,
		}
	}
	return out
}
