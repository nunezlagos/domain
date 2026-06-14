package outboundwebhook

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// UsageAlerter es invocado tras cada agent_run completion para evaluar alerts
// configuradas (issue-15.3). Implementaciones: usagealerts.Service.
type UsageAlerter interface {
	EvaluateRunEvent(ctx context.Context, orgID uuid.UUID, costUSD float64, tokensTotal int64) ([]AlertFired, error)
}

// AlertFired info mínima de una alert que disparó.
type AlertFired struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	Metric     string    `json:"metric"`
	Threshold  float64   `json:"threshold"`
	Observed   float64   `json:"observed"`
	Channel    string    `json:"channel"`
	Recipients []string  `json:"recipients,omitempty"`
}

// RunnerEmitter implementa runner/agent.EventEmitter + runner/flow.EventEmitter
// usando un Dispatcher. Es el bridge entre los runners y las subscriptions outbound.
type RunnerEmitter struct {
	Dispatcher *Dispatcher
	Logger     *slog.Logger
	// UsageAlerts (opcional) evalúa alerts post-run y emite eventos usage.alert.fired.
	UsageAlerts UsageAlerter
}

func (e *RunnerEmitter) EmitAgentRunFinished(ctx context.Context, orgID uuid.UUID, runID uuid.UUID, agentSlug, status string, costUSD float64, tokensTotal int64) {
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

	// issue-15.3: evaluar alerts y emitir evento usage.alert.fired por cada match.
	if e.UsageAlerts != nil {
		fired, err := e.UsageAlerts.EvaluateRunEvent(ctx, orgID, costUSD, tokensTotal)
		if err != nil && e.Logger != nil {
			e.Logger.WarnContext(ctx, "usage_alerts evaluate failed",
				slog.String("error", err.Error()))
		}
		for _, a := range fired {
			alertData, _ := json.Marshal(map[string]any{
				"alert_id":  a.ID,
				"name":      a.Name,
				"metric":    a.Metric,
				"threshold": a.Threshold,
				"observed":  a.Observed,
				"channel":   a.Channel,
				"run_id":    runID,
			})
			alertEv := Event{
				ID:         uuid.New(),
				Type:       "usage.alert_fired",
				OccurredAt: time.Now().UTC(),
				Data:       alertData,
			}
			_ = e.Dispatcher.Emit(ctx, orgID, alertEv)
		}
	}
}

// EmitEntityEvent emite un evento genérico de entidad (ow-002:
// observation.created, invite.created, etc.). Best-effort.
func (e *RunnerEmitter) EmitEntityEvent(ctx context.Context, orgID uuid.UUID, eventType string, data map[string]any) {
	raw, err := json.Marshal(data)
	if err != nil {
		return
	}
	ev := Event{
		ID:         uuid.New(),
		Type:       eventType,
		OccurredAt: time.Now().UTC(),
		Data:       raw,
	}
	if err := e.Dispatcher.Emit(ctx, orgID, ev); err != nil && e.Logger != nil {
		e.Logger.WarnContext(ctx, "entity event emit failed",
			slog.String("event_type", eventType),
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
