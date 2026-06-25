package mcpserver

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"

	skillsvc "nunezlagos/domain/internal/service/skill"
)

func (d *Deps) handleSkillList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Skills == nil {
		return mcp.NewToolResultError("skill service no configurado"), nil
	}
	args := req.GetArguments()
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	f := skillsvc.ListFilter{}
	if v, _ := args["type"].(string); v != "" {
		f.SkillType = v
	}
	if v, _ := args["tag"].(string); v != "" {
		f.Tag = v
	}
	if v, ok := args["limit"].(float64); ok {
		f.Limit = int(v)
	}
	out, err := d.Skills.List(ctx, orgID, f)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"results": out, "count": len(out)})
}

func (d *Deps) handleSkillSearch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Skills == nil {
		return mcp.NewToolResultError("skill service no configurado"), nil
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
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	results, err := d.Skills.SearchHybrid(ctx, orgID, query, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"results": results, "count": len(results)})
}

func (d *Deps) handleSkillGet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Skills == nil {
		return mcp.NewToolResultError("skill service no configurado"), nil
	}
	args := req.GetArguments()
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	if idStr, _ := args["id"].(string); idStr != "" {
		id, err := uuid.Parse(idStr)
		if err != nil {
			return mcp.NewToolResultError("id invalido"), nil
		}
		sk, err := d.Skills.GetByID(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get: %v", err)), nil
		}
		return toolResultJSON(sk)
	}
	slug, _ := args["slug"].(string)
	if slug == "" {
		return mcp.NewToolResultError("id o slug requerido"), nil
	}
	sk, err := d.Skills.GetBySlug(ctx, orgID, slug)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get: %v", err)), nil
	}
	return toolResultJSON(sk)
}
