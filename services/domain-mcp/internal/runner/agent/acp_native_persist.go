package agentrunner

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/audit"
	agentsvc "nunezlagos/domain/internal/service/agent"
)

// completeNativeRun persiste un agent_run nativo terminado (status=completed) y
// devuelve el RunResult. El turno ACP es one-shot, así que insertamos ya
// finalizado en vez de pasar por el estado running como el tool-loop.
func (r *Runner) completeNativeRun(ctx context.Context, orgID uuid.UUID, in RunInput, ro runOpts, agent *agentsvc.Agent, text string) (*RunResult, error) {
	now := time.Now().UTC()
	inputsJSON, _ := json.Marshal(map[string]any{
		"user_prompt": in.UserPrompt,
		"variables":   in.Variables,
	})
	outputJSON, _ := json.Marshal(map[string]any{"text": text})
	metadataJSON := buildRunMetadata(ro, "acp_native")

	var runID uuid.UUID
	err := r.Pool.QueryRow(ctx,
		`INSERT INTO agent_runs (agent_id, user_id, flow_run_id, status, inputs, metadata, outputs, iterations, started_at, finished_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
		 RETURNING id`,
		agent.ID, in.UserID, ro.flowRunID, StatusCompleted, inputsJSON, metadataJSON, outputJSON, 1, now,
	).Scan(&runID)
	if err != nil {
		return nil, fmt.Errorf("create native run: %w", err)
	}

	r.appendLog(ctx, runID, 1, "final", map[string]any{
		"status": StatusCompleted, "output": text, "mode": "acp_native",
	}, 0, 0, 0)
	r.emitNativeFinished(ctx, orgID, runID, agent, in.UserID)

	return &RunResult{
		RunID: runID, Status: StatusCompleted, Output: text, Iterations: 1,
		StartedAt: now, FinishedAt: now,
	}, nil
}

// emitNativeFinished dispara el evento outbound + audit del run nativo.
func (r *Runner) emitNativeFinished(ctx context.Context, orgID, runID uuid.UUID, agent *agentsvc.Agent, userID *uuid.UUID) {
	if r.Emitter != nil {
		r.Emitter.EmitAgentRunFinished(ctx, orgID, runID, agent.Slug, StatusCompleted, 0, 0)
	}
	if r.Audit != nil {
		_ = r.Audit.Record(ctx, audit.Event{
			OrganizationID: &orgID,
			ActorID:        userID,
			ActorType:      audit.ActorUser,
			Action:         "agent.run_" + StatusCompleted,
			EntityType:     "agent_run",
			EntityID:       &runID,
			NewValues:      map[string]any{"agent_slug": agent.Slug, "mode": "acp_native"},
		})
	}
}
