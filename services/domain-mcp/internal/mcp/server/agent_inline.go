package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"

	"nunezlagos/domain/internal/dispatch"
)

// runAgentDispatch (issue-35.1 phase 5): la ejecucion de agents desde
// MCP se delega EXCLUSIVAMENTE al dispatcher unificado. El path legacy
// (AgentRunner.Run directo) fue eliminado: 1 sola implementacion
// compartida por cron, webhook y MCP.
func (d *Deps) runAgentDispatch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Agents == nil {
		return mcp.NewToolResultError("agent service no configurado"), nil
	}
	if d.Dispatcher == nil {
		return mcp.NewToolResultError("dispatcher no configurado"), nil
	}
	args := req.GetArguments()
	slug, _ := args["agent_slug"].(string)
	input, _ := args["input"].(string)
	if slug == "" || input == "" {
		return mcp.NewToolResultError("agent_slug e input son requeridos"), nil
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	userID, _ := uuid.Parse(d.Principal.UserID)
	ag, err := d.Agents.GetBySlug(ctx, orgID, slug)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("agent '%s' not found", slug)), nil
	}
	var vars map[string]any
	if v, ok := args["variables"].(map[string]any); ok {
		vars = v
	}
	if vars == nil {
		vars = map[string]any{}
	}
	vars["input"] = input

	inputsRaw, _ := json.Marshal(vars)

	var meta map[string]any
	if flowRunIDStr, _ := args["flow_run_id"].(string); flowRunIDStr != "" {
		if fid, err := uuid.Parse(flowRunIDStr); err == nil {
			phaseSlug, _ := args["phase_slug"].(string)
			fc := &dispatch.FlowRunContext{FlowRunID: fid, PhaseSlug: phaseSlug}
			meta = fc.InjectIntoMetadata(nil)
		}
	}

	res, dispatchErr := d.Dispatcher.Dispatch(ctx, dispatch.Request{
		OrgID: orgID, Source: dispatch.SourceMCP, TargetType: dispatch.TargetAgent,
		TargetID: ag.ID, Inputs: inputsRaw, TriggeredBy: &userID, Metadata: meta,
	})
	if dispatchErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("dispatch failed: %v", dispatchErr)), nil
	}
	return toolResultJSON(map[string]any{
		"run_id": res.RunID.String(),
		"status": res.Status,
		"output": string(res.Output),
	})
}

func (d *Deps) handleAgentRunLogs(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Pool == nil {
		return mcp.NewToolResultError("pool no configurado"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["run_id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("run_id invalido"), nil
	}

	var exists bool
	err = d.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM agent_runs WHERE id = $1)`, id).Scan(&exists)
	if err != nil || !exists {
		return mcp.NewToolResultError("not found"), nil
	}

	rows, err := d.Pool.Query(ctx,
		`SELECT id, iteration, event_type, payload, tokens_input, tokens_output,
		        latency_ms, occurred_at
		 FROM agent_run_logs WHERE agent_run_id = $1
		 ORDER BY iteration ASC, occurred_at ASC LIMIT 500`, id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("query: %v", err)), nil
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var (
			logID      int64
			iteration  int
			eventType  string
			payloadRaw []byte
			tokensIn   int
			tokensOut  int
			latencyMS  int
			occurredAt any
		)
		if err := rows.Scan(&logID, &iteration, &eventType, &payloadRaw,
			&tokensIn, &tokensOut, &latencyMS, &occurredAt); err != nil {
			continue
		}
		var payload any
		if len(payloadRaw) > 0 {
			_ = json.Unmarshal(payloadRaw, &payload)
		}
		out = append(out, map[string]any{
			"id":            logID,
			"iteration":     iteration,
			"event_type":    eventType,
			"payload":       payload,
			"tokens_input":  tokensIn,
			"tokens_output": tokensOut,
			"latency_ms":    latencyMS,
			"occurred_at":   occurredAt,
		})
	}
	return toolResultJSON(map[string]any{"logs": out, "count": len(out)})
}

func (d *Deps) handleAgentList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Agents == nil {
		return mcp.NewToolResultError("agent service no configurado"), nil
	}
	args := req.GetArguments()
	limit := 50
	if v, ok := args["limit"].(float64); ok {
		limit = int(v)
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	out, err := d.Agents.List(ctx, orgID, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"results": out, "count": len(out)})
}

func (d *Deps) handleAgentGet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Agents == nil {
		return mcp.NewToolResultError("agent service no configurado"), nil
	}
	args := req.GetArguments()
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	if idStr, _ := args["id"].(string); idStr != "" {
		id, err := uuid.Parse(idStr)
		if err != nil {
			return mcp.NewToolResultError("id invalido"), nil
		}
		ag, err := d.Agents.GetByID(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get: %v", err)), nil
		}
		return toolResultJSON(ag)
	}
	slug, _ := args["slug"].(string)
	if slug == "" {
		return mcp.NewToolResultError("id o slug requerido"), nil
	}
	ag, err := d.Agents.GetBySlug(ctx, orgID, slug)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get: %v", err)), nil
	}
	return toolResultJSON(ag)
}
