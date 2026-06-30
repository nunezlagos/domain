// issue-09.6 — resume engine (de-006): retoma un flow_run desde su cursor,
// skip steps ya completados, pausa ante replay_unsafe steps.
package flowrunner

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/api/ctxkeys"
	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/service/flow"
)

// ResumeInput parámetros para reanudar un flow_run existente.
type ResumeInput struct {
	RunID       uuid.UUID
	WorkerID    string
	TriggeredBy *uuid.UUID
}

// ResumeRun reanuda un flow_run desde su último checkpoint.
//  1. Carga el run + su cursor
//  2. Determina steps completados desde cursor["completed"]
//  3. Si current_step es replay_unsafe → paused_awaiting_human
//  4. Sino, ejecuta steps desde el primero NO completado
func (r *Runner) ResumeRun(ctx context.Context, in ResumeInput) (*RunResult, error) {
	var (
		flowID        uuid.UUID
		status        string
		cursorRaw     []byte
		outputsRaw    []byte
		inputsRaw     []byte
		recoveryCount int
		versionID     *uuid.UUID
	)
	err := r.Pool.QueryRow(ctx, `
		SELECT flow_id, status, cursor, outputs, inputs, recovery_count, flow_version_id
		FROM flow_runs WHERE id = $1 FOR UPDATE`, in.RunID,
	).Scan(&flowID, &status, &cursorRaw, &outputsRaw, &inputsRaw, &recoveryCount, &versionID)
	if err != nil {
		return nil, fmt.Errorf("load run: %w", err)
	}

	if status != "running" && status != "pending" {
		return nil, fmt.Errorf("cannot resume run %s: status=%s", in.RunID, status)
	}

	f, err := r.Flows.GetByID(ctx, flowID)
	if err != nil {
		return nil, fmt.Errorf("flow not found: %w", err)
	}

	orgID := ctxkeys.OrgID(ctx)


	if versionID != nil {
		if spec, ok := r.loadRunVersionSpec(ctx, *versionID); ok {
			f.Spec = *spec
		}
	}

	cursor := map[string]any{}
	if len(cursorRaw) > 0 {
		_ = json.Unmarshal(cursorRaw, &cursor)
	}
	stepOutputs := map[string]any{}
	if len(outputsRaw) > 0 {
		_ = json.Unmarshal(outputsRaw, &stepOutputs)
	}
	inputs := map[string]any{}
	if len(inputsRaw) > 0 {
		_ = json.Unmarshal(inputsRaw, &inputs)
	}

	completedSet := extractCompletedIDs(cursor)
	currentStepID, _ := cursor["current_step"].(string)


	_, _ = r.Pool.Exec(ctx,
		`UPDATE flow_runs SET status = 'running', worker_id = $1,
		 last_heartbeat_at = NOW(), recovery_count = recovery_count + 1
		 WHERE id = $2`,
		in.WorkerID, in.RunID,
	)

	if r.Audit != nil {
		_ = r.Audit.Record(ctx, audit.Event{
			OrganizationID: &orgID,
			Action:         "flow_run.resumed_after_crash",
			EntityType:     "flow_run",
			EntityID:       &in.RunID,
			NewValues:      map[string]any{"flow_slug": f.Slug, "recovery_count": recoveryCount + 1},
		})
	}


	if currentStepID != "" {
		for _, s := range f.Spec.Steps {
			if s.ID == currentStepID && !IsReplaySafe(s.ReplaySafe) {
				_, _ = r.Pool.Exec(ctx,
					`UPDATE flow_runs SET status = 'paused_awaiting_human',
					 worker_id = NULL WHERE id = $1`, in.RunID)
				return &RunResult{
					RunID:  in.RunID,
					Status: StatusAwaitHuman,
					Error:  fmt.Sprintf("step '%s' is not replay-safe; requires manual action", currentStepID),
				}, nil
			}
		}
	}


	finalErr := ""
	stepByID := map[string]*flow.Step{}
	for i := range f.Spec.Steps {
		stepByID[f.Spec.Steps[i].ID] = &f.Spec.Steps[i]
	}

	idx := 0
	for idx < len(f.Spec.Steps) {
		if !completedSet[f.Spec.Steps[idx].ID] {
			break
		}
		idx++
	}

LOOP:
	for idx < len(f.Spec.Steps) {
		step := f.Spec.Steps[idx]
		cursor["current_step"] = step.ID
		_ = r.persistCursor(ctx, in.RunID, cursor)

		ctxStep := ctx
		if step.TimeoutS > 0 {
			var cancel context.CancelFunc
			ctxStep, cancel = context.WithTimeout(ctx, time.Duration(step.TimeoutS)*time.Second)
			defer cancel()
		}

		var out any
		var stepErr error
		backoff := 200 * time.Millisecond
		maxBackoff := 30 * time.Second
		if step.MaxBackoffS > 0 {
			maxBackoff = time.Duration(step.MaxBackoffS) * time.Second
		}
		maxAttempts := step.Retries + 1
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			out, stepErr = r.executeStep(ctxStep, in.RunID, &step, inputs, stepOutputs, orgID, in.TriggeredBy)
			if stepErr == nil {
				break
			}
			if !isTransientError(stepErr) {
				break
			}
			if attempt < maxAttempts {
				if r.Metrics != nil {
					r.Metrics.FlowStepRetriesTotal.WithLabelValues(f.Slug, step.ID).Inc()
				}

				bt := time.NewTimer(backoff)
				select {
				case <-ctxStep.Done():
					bt.Stop()
					stepErr = ctxStep.Err()
					break
				case <-bt.C:
				}
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
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
				next, ok := stepByID[step.OnError]
				if !ok {
					finalErr = fmt.Sprintf("step '%s' on_error '%s': %s",
						step.ID, step.OnError, ErrUnknownStepRef.Error())
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


		completedList, _ := cursor["completed"].([]any)
		completedList = append(completedList, step.ID)
		cursor["completed"] = completedList
		_ = r.persistCursor(ctx, in.RunID, cursor)

		idx++
	}

	statusFinal := StatusCompleted
	if finalErr != "" {
		statusFinal = StatusFailed
	}
	finishedAt := time.Now().UTC()
	outputsJSON, _ := json.Marshal(stepOutputs)

	_, _ = r.Pool.Exec(ctx,
		`UPDATE flow_runs SET status = $1, outputs = $2, error = $3, finished_at = $4
		 WHERE id = $5`, statusFinal, outputsJSON, nullStr(finalErr), finishedAt, in.RunID)

	if r.Emitter != nil {
		r.Emitter.EmitFlowRunFinished(ctx, orgID, in.RunID, f.Slug, statusFinal)
	}

	if r.Audit != nil {
		_ = r.Audit.Record(ctx, audit.Event{
			OrganizationID: &orgID,
			ActorID:        in.TriggeredBy,
			ActorType:      audit.ActorSystem,
			Action:         "flow.run_" + statusFinal,
			EntityType:     "flow_run",
			EntityID:       &in.RunID,
			NewValues: map[string]any{
				"flow_slug": f.Slug, "resumed": true,
				"steps_executed": len(stepOutputs),
			},
		})
	}

	return &RunResult{
		RunID: in.RunID, Status: statusFinal, Outputs: stepOutputs, Error: finalErr,
		StartedAt: time.Now(), FinishedAt: finishedAt,
	}, nil
}

// extractCompletedIDs extrae un set de step IDs completados desde el cursor.
func extractCompletedIDs(cursor map[string]any) map[string]bool {
	completedSet := map[string]bool{}
	if raw, ok := cursor["completed"].([]any); ok {
		for _, c := range raw {
			if s, ok := c.(string); ok {
				completedSet[s] = true
			}
		}
	}
	return completedSet
}
