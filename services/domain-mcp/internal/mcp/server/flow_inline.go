package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"

	"nunezlagos/domain/internal/dispatch"
)

func (d *Deps) handleFlowList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Flows == nil {
		return mcp.NewToolResultError("flow service no configurado"), nil
	}
	args := req.GetArguments()
	limit := 50
	if v, ok := args["limit"].(float64); ok {
		limit = int(v)
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	out, err := d.Flows.List(ctx, orgID, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"results": out, "count": len(out)})
}

// runFlowDispatch (issue-35.1 phase 5): la ejecucion de flows desde
// MCP se delega EXCLUSIVAMENTE al dispatcher unificado.
func (d *Deps) runFlowDispatch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if d.Dispatcher == nil {
		return mcp.NewToolResultError("dispatcher no configurado"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["flow_id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("flow_id invalido"), nil
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	userID, _ := uuid.Parse(d.Principal.UserID)
	var inputs map[string]any
	if v, ok := args["inputs"].(map[string]any); ok {
		inputs = v
	}
	inputsRaw, _ := json.Marshal(inputs)
	res, dispatchErr := d.Dispatcher.Dispatch(ctx, dispatch.Request{
		OrgID: orgID, Source: dispatch.SourceMCP, TargetType: dispatch.TargetFlow,
		TargetID: id, Inputs: inputsRaw, TriggeredBy: &userID,
	})
	if dispatchErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("dispatch failed: %v", dispatchErr)), nil
	}
	return toolResultJSON(map[string]any{
		"run_id":  res.RunID.String(),
		"status":  res.Status,
		"error":   "",
		"outputs": map[string]any{"raw": string(res.Output)},
	})
}
