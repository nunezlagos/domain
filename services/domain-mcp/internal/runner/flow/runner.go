// Package flowrunner — issue-09.3 state machine + issue-09.6 durable execution.
//
// Run(flowID, inputs) flow lifecycle:
//  1. Resolve flow + validate active
//  2. Crear flow_run con status=pending
//  3. status=running + started_at
//  4. Loop steps secuencial:
//     a. Execute step según type
//     b. Si error → on_error: fail (abort) | continue (next step) | <step_id> (jump)
//     c. Persistir cursor JSONB con step actual + outputs intermedios
//  5. status=completed | failed con outputs/error
//
// Step types implementados:
//   - agent_run    : delega a agentrunner.Run con inputs interpolados
//   - skill_run    : delega a skillrunner.Execute
//   - http_request : HTTP call directo (similar a skill api)
//   - mem_save     : Save observation en project (delegado a observation.Save)
//   - condition    : evalúa expression (simple equality) y branch
//
// Step types stub (pending HUs):
//   - parallel: issue-08.8 paralelización
//   - wait_signal: issue-09.8 external signals
package flowrunner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/api/ctxkeys"
	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/metrics"
	agentrunner "nunezlagos/domain/internal/runner/agent"
	skillrunner "nunezlagos/domain/internal/runner/skill"
	agentsvc "nunezlagos/domain/internal/service/agent"
	"nunezlagos/domain/internal/service/flow"
	"nunezlagos/domain/internal/service/observation"
	skillsvc "nunezlagos/domain/internal/service/skill"
	"nunezlagos/domain/internal/store/txctx"
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
	Pool         *pgxpool.Pool
	Audit        audit.Recorder
	Flows        *flow.Service
	Agents       *agentsvc.Service
	Skills       *skillsvc.Service
	Observations *observation.Service
	AgentRunner  *agentrunner.Runner
	SkillRunner  *skillrunner.Runner
	Emitter      EventEmitter
	Metrics      *metrics.Registry   // nil = no metrics
	SagaExecutor *flow.SagaExecutor  // issue-09.9: compensaciones Saga
	Snapshots    *flow.SnapshotStore // issue-09.11: snapshots de I/O
	Signals      *flow.SignalStore   // issue-09.8: await_signal + wake LISTEN/NOTIFY

	runContexts   map[uuid.UUID]context.CancelFunc
	runContextsMu sync.Mutex
}

type RunInput struct {
	FlowID      uuid.UUID
	TriggeredBy *uuid.UUID
	TriggerType string // "manual" | "cron" | "webhook" | ...
	Inputs      map[string]any


	FlowVersion int
}

type RunResult struct {
	RunID      uuid.UUID
	Status     string
	Outputs    map[string]any
	Error      string
	StartedAt  time.Time
	FinishedAt time.Time
}

// trackRun registra una cancel func para permitir cancelación externa.
func (r *Runner) trackRun(runID uuid.UUID, cancel context.CancelFunc) {
	r.runContextsMu.Lock()
	defer r.runContextsMu.Unlock()
	if r.runContexts == nil {
		r.runContexts = make(map[uuid.UUID]context.CancelFunc)
	}
	r.runContexts[runID] = cancel
}

// untrackRun elimina el registro de cancelación externa.
func (r *Runner) untrackRun(runID uuid.UUID) {
	r.runContextsMu.Lock()
	defer r.runContextsMu.Unlock()
	delete(r.runContexts, runID)
}

// CancelRun cancela un flow_run en ejecución vía su context.
// Retorna error si el run no está siendo ejecutado por este worker.
func (r *Runner) CancelRun(runID uuid.UUID) error {
	r.runContextsMu.Lock()
	cancel, ok := r.runContexts[runID]
	r.runContextsMu.Unlock()
	if !ok {
		return fmt.Errorf("run %s is not tracked on this worker", runID)
	}
	cancel()
	return nil
}

// Run ejecuta el flow síncronamente. Para async/durable usar issue-09.6 worker.
func (r *Runner) Run(ctx context.Context, in RunInput) (*RunResult, error) {
	f, err := r.Flows.GetByID(ctx, in.FlowID)
	if err != nil {
		return nil, ErrFlowNotFound
	}
	if !f.IsActive {
		return nil, ErrFlowInactive
	}



	orgID := ctxkeys.OrgID(ctx)

	trigger := in.TriggerType
	if trigger == "" {
		trigger = "manual"
	}
	if in.Inputs == nil {
		in.Inputs = map[string]any{}
	}
	inputsJSON, _ := json.Marshal(in.Inputs)


	var versionID *uuid.UUID
	if in.FlowVersion > 0 {
		v, spec, verr := r.resolveVersionSpec(ctx, f.ID, in.FlowVersion)
		if verr != nil {
			return nil, fmt.Errorf("flow version %d: %w", in.FlowVersion, verr)
		}
		f.Spec = *spec
		versionID = &v.ID
	} else if pinned := r.pinVersion(ctx, f); pinned != nil {
		versionID = &pinned.ID
	}


	var parentRunID *uuid.UUID
	ancestorSlugs := []string{}
	depth := 0
	if lin, ok := ctx.Value(runLineageKey{}).(*runLineage); ok && lin != nil {
		parentRunID = &lin.RunID
		ancestorSlugs = lin.Slugs
		depth = lin.Depth + 1
	}

	now := time.Now().UTC()
	var runID uuid.UUID
	err = r.Pool.QueryRow(ctx,
		`INSERT INTO flow_runs
		   (flow_id, triggered_by, trigger_type, status, inputs, started_at,
		    flow_version_id, parent_run_id, ancestor_slugs, depth)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING id`,
		f.ID, in.TriggeredBy, trigger, StatusRunning, inputsJSON, now,
		versionID, parentRunID, ancestorSlugs, depth,
	).Scan(&runID)
	if err != nil {
		return nil, fmt.Errorf("create flow_run: %w", err)
	}


	runCtx, runCancel := context.WithCancel(ctx)

	runCtx = context.WithValue(runCtx, runLineageKey{},
		&runLineage{RunID: runID, Slugs: append(append([]string{}, ancestorSlugs...), f.Slug), Depth: depth})
	r.trackRun(runID, runCancel)
	defer r.untrackRun(runID)
	defer runCancel()

	stepOutputs := map[string]any{} // step_id → result
	cursor := map[string]any{}
	finalErr := ""


	heartbeatCancel := StartHeartbeat(ctx, HeartbeatConfig{
		Interval: 30 * time.Second,
		Pool:     r.Pool,
		RunID:    runID,
		Metrics:  r.Metrics,
	})
	defer heartbeatCancel()

	stepByID := map[string]*flow.Step{}
	for i := range f.Spec.Steps {
		stepByID[f.Spec.Steps[i].ID] = &f.Spec.Steps[i]
	}

	completedIDs := []string{}


	idx := 0
LOOP:
	for idx < len(f.Spec.Steps) {
		step := f.Spec.Steps[idx]
		cursor["current_step"] = step.ID
		cursor["completed"] = completedIDs
		_ = r.persistCursor(ctx, runID, cursor)


		if err := runCtx.Err(); err != nil {
			finalErr = fmt.Sprintf("flow run cancelled: %v", err)
			break LOOP
		}

		ctxStep := runCtx
		if step.TimeoutS > 0 {
			var cancel context.CancelFunc
			ctxStep, cancel = context.WithTimeout(runCtx, time.Duration(step.TimeoutS)*time.Second)
			defer cancel()
		}


		stepRowID := r.beginStepRow(ctx, runID, step.ID)
		ctxStep = WithHeartbeater(ctxStep, &StepHeartbeater{
			Store:     &flow.HeartbeatStore{Pool: r.Pool},
			RunID:     runID,
			StepRowID: stepRowID,
			StepKey:   step.ID,
		})


		out, stepErr, attemptErrs, retryCount := r.runStepWithRetry(
			ctxStep, runID, &step, in.Inputs, stepOutputs, orgID, in.TriggeredBy, f.Slug)

		if err := r.completeStepRow(ctx, stepRowID, out, stepErr); err != nil {
			slog.Default().Warn("complete step row", slog.String("step", step.ID), slog.Any("err", err))
		}
		if stepErr != nil {

			policy := step.OnError
			if policy == "" {
				policy = f.Spec.DefaultStepErrorPolicy
			}
			switch policy {
			case "continue", "ignore_and_continue":

				if step.DefaultOnError != nil {
					stepOutputs[step.ID] = step.DefaultOnError
				} else {
					stepOutputs[step.ID] = map[string]any{"error": stepErr.Error()}
				}
				idx++
				continue LOOP
			case "fallback_step":

				fbOut, fbErr := r.execFallback(ctxStep, runID, &step,
					in.Inputs, stepOutputs, orgID, in.TriggeredBy, f.Slug, 1)
				if fbErr != nil {
					finalErr = fmt.Sprintf("step '%s': %v", step.ID, fbErr)
					break LOOP
				}
				out = map[string]any{"result": fbOut, "fallback_used": true,
					"retry_count": retryCount, "errors": attemptErrs}
				stepErr = nil
			case "", "fail", "abort_flow":


				finalErr = fmt.Sprintf("step '%s' (retry_count=%d): %v", step.ID, retryCount, stepErr)
				break LOOP
			default:

				next, ok := stepByID[policy]
				if !ok {
					finalErr = fmt.Sprintf("step '%s' on_error '%s': %s",
						step.ID, policy, ErrUnknownStepRef.Error())
					break LOOP
				}
				stepOutputs[step.ID] = map[string]any{"error": stepErr.Error(), "jumped_to": next.ID}

				for i, s := range f.Spec.Steps {
					if s.ID == next.ID {
						idx = i
						continue LOOP
					}
				}
			}
		}
		stepOutputs[step.ID] = out
		completedIDs = append(completedIDs, step.ID)
		cursor["completed"] = completedIDs
		_ = r.persistCursor(ctx, runID, cursor)

		if step.Compensate != "" && r.SagaExecutor != nil {
			comp := flow.SagaCompensation{
				StepKey:       step.ID,
				CompensateKey: step.Compensate,
			}
			if r.SagaExecutor.Store != nil {
				_ = r.SagaExecutor.Store.RegisterCompensation(ctx, runID, comp)
			}
		}

		if r.Snapshots != nil {
			snapID := uuid.New()
			inputsJSON, _ := json.Marshal(in.Inputs)
			outputsJSON, _ := json.Marshal(out)
			snap := &flow.StepSnapshot{
				ID:         snapID,
				StepID:     snapID,
				RunID:      runID,
				StepKey:    step.ID,
				Input:      inputsJSON,
				Output:     outputsJSON,
				DurationMs: int64(time.Since(now).Milliseconds()),
				CapturedAt: time.Now().UTC(),
			}
			if err := r.Snapshots.SaveSnapshot(ctx, snap, CompressOutput); err != nil {
				slog.Default().Error("save snapshot", slog.String("step", step.ID), slog.Any("err", err))
			}
		}
		idx++
	}

	status := StatusCompleted
	compensationStatus := ""
	if finalErr != "" {
		status = StatusFailed

		if r.SagaExecutor != nil {
			var plan []flow.SagaCompensation
			if r.SagaExecutor.Store != nil {
				plan = r.SagaExecutor.Store.RegisteredCompensations(runID)
			}
			if len(plan) == 0 {

				for _, s := range f.Spec.Steps {
					if s.Compensate != "" {
						plan = append(plan, flow.SagaCompensation{
							StepKey:       s.ID,
							CompensateKey: s.Compensate,
						})
					}
				}
			}
			if len(plan) > 0 {
				err := r.SagaExecutor.ExecuteCompensations(ctx, runID, completedIDs, plan)
				if err != nil {
					compensationStatus = "failed_compensation_failed"
					slog.Default().Error("saga compensation execution failed",
						slog.String("run_id", runID.String()), slog.Any("err", err))
				} else {
					compensationStatus = "failed_compensated"
				}
			}
		}
	}
	if compensationStatus != "" {
		status = compensationStatus
	}
	finishedAt := time.Now().UTC()
	outputsJSON, _ := json.Marshal(stepOutputs)

	_, _ = r.Pool.Exec(ctx,
		`UPDATE flow_runs SET status = $1, outputs = $2, error = $3, finished_at = $4
		 WHERE id = $5`, status, outputsJSON, nullStr(finalErr), finishedAt, runID)

	if r.Emitter != nil {
		r.Emitter.EmitFlowRunFinished(ctx, orgID, runID, f.Slug, status)
	}

	if r.Audit != nil {
		_ = r.Audit.Record(ctx, audit.Event{
			OrganizationID: &orgID,
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
// isTransientError determina si un error es retryable.
// Non-transient: auth, 4xx, validation → fail inmediato.
// Transient: network, 5xx, timeout → retry.
func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	nonTransient := []string{
		"unauthorized", "forbidden", "not found", "invalid",
		"bad request", "HTTP 4", "conflict", "too many requests",
		"not supported", "validation", "ErrStepTypeStub",
		"unknown step type", "step_type_required",
	}
	for _, kw := range nonTransient {
		if strings.Contains(strings.ToLower(msg), kw) {
			return false
		}
	}
	return true
}

func (r *Runner) executeStep(ctx context.Context, runID uuid.UUID, step *flow.Step, inputs, outputs map[string]any,
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
	case flow.StepTypeSubFlow:
		return r.execSubFlow(ctx, step, orgID, userID)
	case flow.StepTypeParallel:
		return r.execParallel(ctx, runID, step, inputs, outputs, orgID, userID)
	case flow.StepTypeWaitSignal:
		return r.execWaitSignal(ctx, runID, step)
	case flow.StepTypeHTTPRequest:
		return nil, fmt.Errorf("%w: %s (issue-09 future)", ErrStepTypeStub, step.Type)
	}
	return nil, fmt.Errorf("unknown step type: %s", step.Type)
}

// execWaitSignal pausa el run hasta recibir la señal externa (issue-09.8).
//
// Config esperado:
//
//	{"signal_name": "approval_received", "timeout_seconds": 86400}
//
// El run pasa a paused_awaiting_signal mientras espera (sin CPU: LISTEN/NOTIFY)
// y vuelve a running al recibirla. Output: payload del signal.
func (r *Runner) execWaitSignal(ctx context.Context, runID uuid.UUID, step *flow.Step) (any, error) {
	if r.Signals == nil {
		return nil, fmt.Errorf("wait_signal: SignalStore not configured")
	}
	name, _ := step.Config["signal_name"].(string)
	if name == "" {
		name, _ = step.Config["signal"].(string)
	}
	if name == "" {
		return nil, fmt.Errorf("wait_signal: signal_name required (validation)")
	}
	timeout := 24 * time.Hour
	if ts, ok := step.Config["timeout_seconds"].(float64); ok && ts > 0 {
		timeout = time.Duration(ts) * time.Second
	}

	if _, err := r.Signals.ExpectSignal(ctx, runID, step.ID, name, timeout); err != nil {
		return nil, fmt.Errorf("wait_signal expect: %w", err)
	}
	r.setRunStatus(ctx, runID, StatusAwaitSig)

	stepKey := step.ID
	sig, err := r.Signals.WaitNotify(ctx, runID, &stepKey, name, timeout)
	r.setRunStatus(ctx, runID, StatusRunning)
	if err != nil {
		return nil, fmt.Errorf("wait_signal %q: %w", name, err)
	}
	_ = r.Signals.CancelExpectation(ctx, runID)

	out := map[string]any{"signal": sig.Name, "delivered_at": sig.DeliveredAt}
	if len(sig.Payload) > 0 {
		var payload any
		if err := json.Unmarshal(sig.Payload, &payload); err == nil {
			out["payload"] = payload
		}
	}
	return out, nil
}

// setRunStatus actualiza el status del run sin tocar finished_at.
func (r *Runner) setRunStatus(ctx context.Context, runID uuid.UUID, status string) {
	if _, err := r.Pool.Exec(ctx,
		`UPDATE flow_runs SET status = $2 WHERE id = $1`, runID, status); err != nil {
		slog.Default().Warn("set run status", slog.String("status", status), slog.Any("err", err))
	}
}

// subflowCtxKey rastrea la cadena de slugs ancestrales para detectar
// referencias circulares (issue-09.5 escenario 6).
type subflowCtxKey struct{}

// runLineageKey propaga la identidad del run padre a los sub-flows que
// lanza, para persistir parent_run_id/ancestor_slugs/depth (issue-09.5).
type runLineageKey struct{}

type runLineage struct {
	RunID uuid.UUID
	Slugs []string // ancestros + slug propio
	Depth int
}

// maxSubflowDepth — máximo 5 niveles de anidamiento (spec issue-09.5).
const maxSubflowDepth = 5

// maxSubflowOutputBytes limita el output que un sub-flow devuelve al padre.
const maxSubflowOutputBytes = 1 << 20 // 1MB

// execSubFlow ejecuta un flow anidado como step.
//
// Config esperado:
//
//	{"flow_slug": "email-notifier", "input": {...}}
//
// Output: {"flow_run_id": "...", "status": "completed", "outputs": {...}}.
// Si el sub-flow falla, retorna error con el mensaje (parent aplica on_error policy).
func (r *Runner) execSubFlow(ctx context.Context, step *flow.Step, orgID uuid.UUID, userID *uuid.UUID) (any, error) {
	flowSlug, _ := step.Config["flow_slug"].(string)
	if flowSlug == "" {
		return nil, fmt.Errorf("sub_flow: flow_slug required")
	}
	inputs, _ := step.Config["input"].(map[string]any)
	if inputs == nil {
		inputs = map[string]any{}
	}


	chain, _ := ctx.Value(subflowCtxKey{}).([]string)
	for _, s := range chain {
		if s == flowSlug {
			return nil, fmt.Errorf("circular sub-flow reference detected: %s",
				formatChain(append(chain, flowSlug)))
		}
	}
	if len(chain) >= maxSubflowDepth {
		return nil, fmt.Errorf("sub-flow depth exceeded (%d): %s", maxSubflowDepth, formatChain(chain))
	}
	childCtx := context.WithValue(ctx, subflowCtxKey{}, append(chain, flowSlug))


	childFlow, err := r.Flows.GetBySlug(ctx, orgID, flowSlug)
	if err != nil {
		return nil, fmt.Errorf("sub-flow %q not found", flowSlug)
	}

	res, err := r.Run(childCtx, RunInput{
		FlowID:      childFlow.ID,
		TriggeredBy: userID,
		TriggerType: "subflow",
		Inputs:      inputs,
	})
	if err != nil {
		return nil, fmt.Errorf("sub-flow %q failed to start: %w", flowSlug, err)
	}
	if res.Status != StatusCompleted {
		return map[string]any{
			"flow_run_id": res.RunID,
			"status":      res.Status,
			"error":       res.Error,
		}, fmt.Errorf("sub-flow %q ended with status %s: %s", flowSlug, res.Status, res.Error)
	}

	if raw, err := json.Marshal(res.Outputs); err == nil && len(raw) > maxSubflowOutputBytes {
		return nil, fmt.Errorf("sub-flow %q output exceeds %d bytes (validation)", flowSlug, maxSubflowOutputBytes)
	}
	return map[string]any{
		"flow_run_id": res.RunID,
		"status":      res.Status,
		"outputs":     res.Outputs,
	}, nil
}

// execParallel ejecuta N branches concurrentemente, espera a todos, retorna
// el array de resultados (en el orden declarado, no de completion).
//
// Config esperado:
//
//	{"branches": [<step1>, <step2>, ...]}
//
// Si CUALQUIER branch falla, el step parallel falla con el primer error
// y los demás branches reciben ctx cancel.
//
// Output: {"results": [...], "errors": [...]} ambos arrays paralelos a branches.
func (r *Runner) execParallel(parentCtx context.Context, runID uuid.UUID, step *flow.Step,
	inputs, outputs map[string]any, orgID uuid.UUID, userID *uuid.UUID) (any, error) {

	branchesRaw, ok := step.Config["branches"].([]any)
	if !ok || len(branchesRaw) == 0 {
		return nil, fmt.Errorf("parallel: branches[] requerido")
	}
	maxBranches := 32
	if len(branchesRaw) > maxBranches {
		return nil, fmt.Errorf("parallel: max %d branches, got %d", maxBranches, len(branchesRaw))
	}

	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	type result struct {
		idx int
		val any
		err error
	}
	resCh := make(chan result, len(branchesRaw))

	for i, b := range branchesRaw {
		branchMap, ok := b.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("parallel branch[%d]: must be object", i)
		}
		branchStep := mapToStep(branchMap)
		go func(idx int, st *flow.Step) {
			val, err := r.executeStep(ctx, runID, st, inputs, outputs, orgID, userID)
			resCh <- result{idx: idx, val: val, err: err}
		}(i, branchStep)
	}

	results := make([]any, len(branchesRaw))
	errs := make([]string, len(branchesRaw))
	var firstErr error
	for i := 0; i < len(branchesRaw); i++ {
		r := <-resCh
		results[r.idx] = r.val
		if r.err != nil {
			errs[r.idx] = r.err.Error()
			if firstErr == nil {
				firstErr = r.err
				cancel() // cancela el resto al primer error
			}
		}
	}

	out := map[string]any{"results": results, "errors": errs}
	if firstErr != nil {
		return out, fmt.Errorf("parallel: branch failed: %w", firstErr)
	}
	return out, nil
}

// mapToStep convierte un map (de Config branches) a un flow.Step.
func mapToStep(m map[string]any) *flow.Step {
	step := &flow.Step{
		Config: map[string]any{},
	}
	if v, ok := m["id"].(string); ok {
		step.ID = v
	}
	if v, ok := m["type"].(string); ok {
		step.Type = v
	}
	if v, ok := m["config"].(map[string]any); ok {
		step.Config = v
	} else if v, ok := m["params"].(map[string]any); ok {
		step.Config = v
	}
	return step
}

func formatChain(slugs []string) string {
	if len(slugs) == 0 {
		return ""
	}
	out := slugs[0]
	for _, s := range slugs[1:] {
		out += " → " + s
	}
	return out
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



	var obs *observation.Observation
	err = txctx.WithOrgTx(ctx, r.Pool, orgID, func(tx pgx.Tx) error {
		txCtx := txctx.WithTxContext(ctx, tx)
		var saveErr error
		obs, saveErr = r.Observations.Save(txCtx, observation.SaveInput{
			OrganizationID:  orgID,
			ProjectID:       projectID,
			CreatedBy:       userID,
			Content:         content,
			ObservationType: obsType,
		})
		return saveErr
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"observation_id": obs.ID.String()}, nil
}

// execCondition evalúa expresión simple "field == value" o "field != value".
// Config:
//
//	{ "left": "{{inputs.status}}", "op": "==", "right": "ok", "if_true": "...", "if_false": "..." }
//
// Devuelve {branch: "if_true"|"if_false"} para que steps siguientes lo usen.
func (r *Runner) execCondition(step *flow.Step, inputs, outputs map[string]any) (any, error) {
	left, _ := step.Config["left"].(string)
	right, _ := step.Config["right"].(string)
	op, _ := step.Config["op"].(string)
	if op == "" {
		op = "=="
	}

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

// persistStepOutput inserta/actualiza flow_run_steps con el output de un step.
// issue-09.6: también genera la idempotency key para el step.
func (r *Runner) persistStepOutput(ctx context.Context, runID uuid.UUID, stepID string, output any, stepErr error) error {
	outputsJSON, _ := json.Marshal(output)
	var errStr *string
	if stepErr != nil {
		s := stepErr.Error()
		errStr = &s
	}
	compressed, _, _ := CompressOutput(output)
	idemKey := StepIDempotencyKey(runID, stepID)

	_, err := r.Pool.Exec(ctx, `
		INSERT INTO flow_run_steps
			(flow_run_id, step_key, status, inputs, outputs, error, output_compressed,
			 started_at, completed_at, last_heartbeat_at)
		VALUES ($1, $2, $3, '{}', $4, $5, $6, NOW(), NOW(), NOW())`,
		runID, stepID, "completed", outputsJSON, errStr, compressed,
	)
	if err != nil {
		return fmt.Errorf("insert step output: %w", err)
	}

	_ = idemKey // issue-09.6 de-008: la key está disponible para tracing / downstream
	return nil
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
