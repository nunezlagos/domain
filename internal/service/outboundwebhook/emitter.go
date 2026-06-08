package outboundwebhook

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// RunnerEmitter implementa runner/agent.EventEmitter + runner/flow.EventEmitter
// usando un Dispatcher. Es el bridge entre los runners y las subscriptions outbound.
type RunnerEmitter struct {
	Dispatcher *Dispatcher
	Logger     *slog.Logger
}

func (e *RunnerEmitter) EmitAgentRunFinished(ctx context.Context, orgID uuid.UUID, runID uuid.UUID, agentSlug, status string, costUSD float64) {
	eventType := "agent_run.completed"
	if status != "completed" {
		eventType = "agent_run.failed"
	}
	data, _ := json.Marshal(map[string]any{
		"run_id":     runID,
		"agent_slug": agentSlug,
		"status":     status,
		"cost_usd":   costUSD,
	})
	ev := Event{
		ID:         uuid.New(),
		Type:       eventType,
		OccurredAt: time.Now().UTC(),
		Data:       data,
	}
	if err := e.Dispatcher.Emit(ctx, orgID, ev); err != nil && e.Logger != nil {
		e.Logger.WarnContext(ctx, "agent_run event emit failed",
			slog.String("run_id", runID.String()),
			slog.String("error", err.Error()))
	}
}

func (e *RunnerEmitter) EmitFlowRunFinished(ctx context.Context, orgID, runID uuid.UUID, flowSlug, status string) {
	eventType := "flow_run.completed"
	if status != "completed" {
		eventType = "flow_run.failed"
	}
	data, _ := json.Marshal(map[string]any{
		"run_id":    runID,
		"flow_slug": flowSlug,
		"status":    status,
	})
	ev := Event{
		ID:         uuid.New(),
		Type:       eventType,
		OccurredAt: time.Now().UTC(),
		Data:       data,
	}
	if err := e.Dispatcher.Emit(ctx, orgID, ev); err != nil && e.Logger != nil {
		e.Logger.WarnContext(ctx, "flow_run event emit failed",
			slog.String("run_id", runID.String()),
			slog.String("error", err.Error()))
	}
}
