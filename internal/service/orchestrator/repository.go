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
	UpdateFlowRunStatus(ctx context.Context, flowRunID uuid.UUID, status string) error
}

// FlowRunRow es la vista de lectura completa de un flow_run.
type FlowRunRow struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	FlowID         uuid.UUID
	Status         string
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
	FlowID          uuid.UUID
	TriggeredBy     uuid.UUID
	Status          string
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

func (r *pgRepository) GetFlowIDBySlug(ctx context.Context, orgID uuid.UUID, slug string) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, `
		SELECT id FROM flows
		WHERE organization_id = $1 AND slug = $2 AND deleted_at IS NULL
		LIMIT 1`, orgID, slug,
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
	_, err = r.pool.Exec(ctx, `
		INSERT INTO flow_runs
		  (id, organization_id, flow_id, triggered_by, trigger_type, status,
		   inputs, cursor, started_at)
		VALUES ($1,$2,$3,$4,'manual',$5,$6,$7,$8)`,
		in.ID, in.OrganizationID, in.FlowID, nullUUID(in.TriggeredBy),
		in.Status, inputsJSON, metadataJSON, in.StartedAt)
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
	)
	err := r.pool.QueryRow(ctx, `
		SELECT id, organization_id, flow_id, status, cursor, started_at, finished_at
		FROM flow_runs WHERE id = $1`, id,
	).Scan(&row.ID, &row.OrganizationID, &row.FlowID, &row.Status,
		&cursorRaw, &startedAt, &finishedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrFlowRunNotFound
		}
		return nil, fmt.Errorf("get flow_run: %w", err)
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
		  AND status IN ('pending','running')`,
		stepID, errorMsg)
	if err != nil {
		return fmt.Errorf("update step failed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrFlowRunStepNotPending
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
		FlowID:         flowID,
		TriggeredBy:    in.UserID,
		Status:         "pending",
		Inputs:         map[string]any{"raw_text": in.RawText},
		Metadata: map[string]any{
			"orchestrator_run_id": orchestratorRunID.String(),
			"mode":                string(mode),
			"raw_text":            in.RawText,
		},
		StartedAt: now,
	}); err != nil {
		return err
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
