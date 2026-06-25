package mcpserver

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"

	searchsvc "nunezlagos/domain/internal/service/search"
)

func (d *Deps) handleContext(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Timeline == nil {
		return mcp.NewToolResultError("timeline service no configurado"), nil
	}
	args := req.GetArguments()
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	userID, _ := uuid.Parse(d.Principal.UserID)
	var projectID uuid.UUID
	if ps, _ := args["project_slug"].(string); ps != "" {
		proj, err := d.Projects.GetBySlug(ctx, orgID, ps)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", ps)), nil
		}
		projectID = proj.ID
	}
	snap, err := d.Timeline.Context(ctx, orgID, userID, projectID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("context: %v", err)), nil
	}
	return toolResultJSON(snap)
}

func (d *Deps) handleTimeline(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Timeline == nil {
		return mcp.NewToolResultError("timeline service no configurado"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["observation_id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("observation_id invalido"), nil
	}
	before := 3
	after := 3
	if v, ok := args["before"].(float64); ok {
		before = int(v)
	}
	if v, ok := args["after"].(float64); ok {
		after = int(v)
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	entries, err := d.Timeline.Timeline(ctx, orgID, id, before, after)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("timeline: %v", err)), nil
	}
	return toolResultJSON(entries)
}

func (d *Deps) handleGlobalSearch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Search == nil {
		return mcp.NewToolResultError("search service no configurado"), nil
	}
	args := req.GetArguments()
	query, _ := args["query"].(string)
	if query == "" {
		return mcp.NewToolResultError("query requerido"), nil
	}
	limit := 20
	if v, ok := args["limit"].(float64); ok {
		limit = int(v)
	}
	filter := searchsvc.Filter{}
	if et, ok := args["entity_types"].([]any); ok {
		for _, t := range et {
			if s, ok := t.(string); ok {
				filter.EntityTypes = append(filter.EntityTypes, searchsvc.EntityType(s))
			}
		}
	}
	if tg, ok := args["tags"].([]any); ok {
		for _, t := range tg {
			if s, ok := t.(string); ok {
				filter.Tags = append(filter.Tags, s)
			}
		}
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	results, err := d.Search.Search(ctx, orgID, query, limit, filter)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"results": results,
		"count":   len(results),
	})
}
