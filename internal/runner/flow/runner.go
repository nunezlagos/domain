// Package flowrunner — HU-09.3 state machine + HU-09.6 durable execution.
//
// Run(flowID, inputs) flow lifecycle:
//   1. Resolve flow + validate active
//   2. Crear flow_run con status=pending
//   3. status=running + started_at
//   4. Loop steps secuencial:
//      a. Execute step según type
//      b. Si error → on_error: fail (abort) | continue (next step) | <step_id> (jump)
//      c. Persistir cursor JSONB con step actual + outputs intermedios
//   5. status=completed | failed con outputs/error
//
// Step types implementados:
//   - agent_run    : delega a agentrunner.Run con inputs interpolados
//   - skill_run    : delega a skillrunner.Execute
//   - http_request : HTTP call directo (similar a skill api)
//   - mem_save     : Save observation en project (delegado a observation.Save)
//   - condition    : evalúa expression (simple equality) y branch
//
// Step types stub (pending HUs):
//   - parallel: HU-08.8 paralelización
//   - wait_signal: HU-09.8 external signals
package flowrunner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
	agentrunner "nunezlagos/domain/internal/runner/agent"
	skillrunner "nunezlagos/domain/internal/runner/skill"
	agentsvc "nunezlagos/domain/internal/service/agent"
	"nunezlagos/domain/internal/service/flow"
	"nunezlagos/domain/internal/service/observation"
	skillsvc "nunezlagos/domain/internal/service/skill"
)

const (
	StatusPending    = "pending"
	StatusRunning    = "running"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
	StatusPaused     = "paused"
	StatusCancelled  = "cancelled"
	StatusAwaitSig   = "paused_awaiting_signal"
	StatusAwaitHuman = "paused_awaiting_human"
)

var (
	ErrFlowNotFound   = errors.New("flow not found")
	ErrFlowInactive   = errors.New("flow is inactive")
	ErrStepTypeStub   = errors.New("step type not implemented yet")
	ErrUnknownStepRef = errors.New("on_error references unknown step")
)

// EventEmitter dispara eventos de dominio post-run.
type EventEmitter interface {
	EmitFlowRunFinished(ctx context.Context, orgID, runID uuid.UUID, flowSlug, status string)
}

type Runner struct {
	Pool        *pgxpool.Pool
	Audit       audit.Recorder
	Flows       *flow.Service
	Agents      *agentsvc.Service
	Skills      *skillsvc.Service
	Observations *observation.Service
	AgentRunner *agentrunner.Runner
	SkillRunner *skillrunner.Runner
	Emitter     EventEmitter
}

type RunInput struct {
	FlowID      uuid.UUID
	TriggeredBy *uuid.UUID
	TriggerType string // "manual" | "cron" | "webhook" | ...
	Inputs      map[string]any
}

type RunResult struct {
	RunID      uuid.UUID
	Status     string
	Outputs    map[string]any
	Error      string
	StartedAt  time.Time
	FinishedAt time.Time
}

// Run ejecuta el flow síncronamente. Para async/durable usar HU-09.6 worker.
func (r *Runner) Run(ctx context.Context, in RunInput) (*RunResult, error) {
	f, err := r.Flows.GetByID(ctx, in.FlowID)
	if err != nil {
		return nil, ErrFlowNotFound
	}
	if !f.IsActive {
		return nil, ErrFlowInactive
	}

	trigger := in.TriggerType
	if trigger == "" {
		trigger = "manual"
	}
	if in.Inputs == nil {
		in.Inputs = map[string]any{}
	}
	inputsJSON, _ := json.Marshal(in.Inputs)

	now := time.Now().UTC()
	var runID uuid.UUID
	err = r.Pool.QueryRow(ctx,
		`INSERT INTO flow_runs
		   (organization_id, flow_id, triggered_by, trigger_type, status, inputs, started_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id`,
		f.OrganizationID, f.ID, in.TriggeredBy, trigger, StatusRunning, inputsJSON, now,
	).Scan(&runID)
	if err != nil {
		return nil, fmt.Errorf("create flow_run: %w", err)
	}

	stepOutputs := map[string]any{} // step_id → result
	cursor := map[string]any{}
	finalErr := ""

	stepByID := map[string]*flow.Step{}
	for i := range f.Spec.Steps {
		stepByID[f.Spec.Steps[i].ID] = &f.Spec.Steps[i]
	}

	// Ejecución secuencial. Loop con jump support.
	idx := 0
LOOP:
	for idx < len(f.Spec.Steps) {
		step := f.Spec.Steps[idx]
		cursor["current_step"] = step.ID
		_ = r.persistCursor(ctx, runID, cursor)

		ctxStep := ctx
		if step.TimeoutS > 0 {
			var cancel context.CancelFunc
			ctxStep, cancel = context.WithTimeout(ctx, time.Duration(step.TimeoutS)*time.Second)
			defer cancel()
		}

		// HU-09.4 retry: si step.Retries > 0, reintentar con backoff exponencial.
		var out any
		var stepErr error
		backoff := 200 * time.Millisecond
		maxAttempts := step.Retries + 1 // 0 retries = 1 attempt
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			out, stepErr = r.executeStep(ctxStep, &step, in.Inputs, stepOutputs, f.OrganizationID, in.TriggeredBy)
			if stepErr == nil {
				break
			}
			if attempt < maxAttempts {
				select {
				case <-ctxStep.Done():
					stepErr = ctxStep.Err()
					break
				case <-time.After(backoff):
				}
				backoff *= 2
			}
		}
		if stepErr != nil {
			switch step.OnError {
			case "continue":
				stepOutputs[step.ID] = map[string]any{"error": stepErr.Error()}
				idx++
				continue LOOP
			case "", "fail":
				finalErr = fmt.Sprintf("step '%s': %v", step.ID, stepErr)
				break LOOP
			default:
				// Jump al step_id especificado
				next, ok := stepByID[step.OnError]
				if !ok {
					finalErr = fmt.Sprintf("step '%s' on_error '%s': %s",
						step.ID, step.OnError, ErrUnknownStepRef.Error())
					break LOOP
				}
				stepOutputs[step.ID] = map[string]any{"error": stepErr.Error(), "jumped_to": next.ID}
				// Encontrar índice del step destino
				for i, s := range f.Spec.Steps {
					if s.ID == next.ID {
						idx = i
						continue LOOP
					}
				}
			}
		}
		stepOutputs[step.ID] = out
		idx++
	}

	status := StatusCompleted
	if finalErr != "" {
		status = StatusFailed
	}
	finishedAt := time.Now().UTC()
	outputsJSON, _ := json.Marshal(stepOutputs)

	_, _ = r.Pool.Exec(ctx,
		`UPDATE flow_runs SET status = $1, outputs = $2, error = $3, finished_at = $4
		 WHERE id = $5`, status, outputsJSON, nullStr(finalErr), finishedAt, runID)

	if r.Emitter != nil {
		r.Emitter.EmitFlowRunFinished(ctx, f.OrganizationID, runID, f.Slug, status)
	}

	if r.Audit != nil {
		_ = r.Audit.Record(ctx, audit.Event{
			OrganizationID: &f.OrganizationID,
			ActorID:        in.TriggeredBy,
			ActorType:      audit.ActorUser,
			Action:         "flow.run_" + status,
			EntityType:     "flow_run",
			EntityID:       &runID,
			NewValues: map[string]any{
				"flow_slug": f.Slug, "trigger": trigger,
				"steps_executed": len(stepOutputs),
			},
		})
	}
	return &RunResult{
		RunID: runID, Status: status, Outputs: stepOutputs, Error: finalErr,
		StartedAt: now, FinishedAt: finishedAt,
	}, nil
}

// executeStep dispatch por step.Type.
func (r *Runner) executeStep(ctx context.Context, step *flow.Step, inputs, outputs map[string]any,
	orgID uuid.UUID, userID *uuid.UUID) (any, error) {

	switch step.Type {
	case flow.StepTypeAgentRun:
		return r.execAgentRun(ctx, step, orgID, userID)
	case flow.StepTypeSkillRun:
		return r.execSkillRun(ctx, step, orgID)
	case flow.StepTypeMemSave:
		return r.execMemSave(ctx, step, orgID, userID)
	case flow.StepTypeCondition:
		return r.execCondition(step, inputs, outputs)
	case flow.StepTypeHTTPRequest, flow.StepTypeParallel, flow.StepTypeWaitSignal:
		return nil, fmt.Errorf("%w: %s (HU-09 future)", ErrStepTypeStub, step.Type)
	}
	return nil, fmt.Errorf("unknown step type: %s", step.Type)
}

func (r *Runner) execAgentRun(ctx context.Context, step *flow.Step, orgID uuid.UUID, userID *uuid.UUID) (any, error) {
	agentSlug, _ := step.Config["agent_slug"].(string)
	input, _ := step.Config["input"].(string)
	if agentSlug == "" || input == "" {
		return nil, fmt.Errorf("agent_run: agent_slug and input required")
	}
	if r.AgentRunner == nil {
		return nil, errors.New("agent_run: AgentRunner not configured")
	}
	ag, err := r.Agents.GetBySlug(ctx, orgID, agentSlug)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %s", agentSlug)
	}
	res, err := r.AgentRunner.Run(ctx, agentrunner.RunInput{
		AgentID: ag.ID, UserID: userID, UserPrompt: input,
	})
	if res == nil {
		return nil, err
	}
	return map[string]any{
		"run_id": res.RunID.String(), "status": res.Status,
		"output": res.Output, "tokens_total": res.TokensInput + res.TokensOutput,
	}, nil
}

func (r *Runner) execSkillRun(ctx context.Context, step *flow.Step, orgID uuid.UUID) (any, error) {
	skillSlug, _ := step.Config["skill_slug"].(string)
	args, _ := step.Config["args"].(map[string]any)
	if skillSlug == "" {
		return nil, errors.New("skill_run: skill_slug required")
	}
	if r.SkillRunner == nil {
		return nil, errors.New("skill_run: SkillRunner not configured")
	}
	sk, err := r.Skills.GetBySlug(ctx, orgID, skillSlug)
	if err != nil {
		return nil, fmt.Errorf("skill not found: %s", skillSlug)
	}
	if err := r.Skills.ValidateInput(ctx, sk.ID, args); err != nil {
		return nil, fmt.Errorf("invalid skill input: %w", err)
	}
	result, err := r.SkillRunner.Execute(ctx, sk, args)
	if err != nil {
		return nil, err
	}
	return map[string]any{"result": result}, nil
}

func (r *Runner) execMemSave(ctx context.Context, step *flow.Step, orgID uuid.UUID, userID *uuid.UUID) (any, error) {
	projectIDStr, _ := step.Config["project_id"].(string)
	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		return nil, fmt.Errorf("mem_save: invalid project_id: %w", err)
	}
	content, _ := step.Config["content"].(string)
	if content == "" {
		return nil, errors.New("mem_save: content required")
	}
	obsType, _ := step.Config["observation_type"].(string)
	if r.Observations == nil {
		return nil, errors.New("mem_save: Observations service not configured")
	}
	obs, err := r.Observations.Save(ctx, observation.SaveInput{
		OrganizationID:  orgID,
		ProjectID:       projectID,
		CreatedBy:       userID,
		Content:         content,
		ObservationType: obsType,
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"observation_id": obs.ID.String()}, nil
}

// execCondition evalúa expresión simple "field == value" o "field != value".
// Config:
//   { "left": "{{inputs.status}}", "op": "==", "right": "ok", "if_true": "...", "if_false": "..." }
// Devuelve {branch: "if_true"|"if_false"} para que steps siguientes lo usen.
func (r *Runner) execCondition(step *flow.Step, inputs, outputs map[string]any) (any, error) {
	left, _ := step.Config["left"].(string)
	right, _ := step.Config["right"].(string)
	op, _ := step.Config["op"].(string)
	if op == "" {
		op = "=="
	}
	// Resolver {{inputs.field}} → valor
	leftResolved := resolveTemplate(left, inputs, outputs)
	rightResolved := resolveTemplate(right, inputs, outputs)
	var branch string
	switch op {
	case "==":
		if leftResolved == rightResolved {
			branch = "if_true"
		} else {
			branch = "if_false"
		}
	case "!=":
		if leftResolved != rightResolved {
			branch = "if_true"
		} else {
			branch = "if_false"
		}
	default:
		return nil, fmt.Errorf("condition: unsupported op '%s'", op)
	}
	return map[string]any{"branch": branch, "left": leftResolved, "right": rightResolved}, nil
}

// resolveTemplate substituye {{inputs.x}} y {{outputs.step_id.field}}.
func resolveTemplate(s string, inputs, outputs map[string]any) string {
	// MVP simple: solo placeholders top-level
	// {{inputs.foo}} → inputs["foo"]
	if val, ok := tryResolve(s, "{{inputs.", inputs); ok {
		return val
	}
	if val, ok := tryResolve(s, "{{outputs.", outputs); ok {
		return val
	}
	return s
}

func tryResolve(s, prefix string, m map[string]any) (string, bool) {
	if len(s) < len(prefix)+2 || s[:len(prefix)] != prefix || s[len(s)-2:] != "}}" {
		return "", false
	}
	key := s[len(prefix) : len(s)-2]
	if v, ok := m[key]; ok {
		return fmt.Sprint(v), true
	}
	return "", false
}

func (r *Runner) persistCursor(ctx context.Context, runID uuid.UUID, cursor map[string]any) error {
	raw, _ := json.Marshal(cursor)
	_, err := r.Pool.Exec(ctx,
		`UPDATE flow_runs SET cursor = $1, last_heartbeat_at = NOW() WHERE id = $2`,
		raw, runID)
	return err
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
