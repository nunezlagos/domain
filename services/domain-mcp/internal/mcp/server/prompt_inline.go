package mcpserver

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
)

func (d *Deps) handlePromptGet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Prompts == nil {
		return mcp.NewToolResultError("prompt service no configurado"), nil
	}
	args := req.GetArguments()
	slug, _ := args["slug"].(string)
	if slug == "" {
		return mcp.NewToolResultError("slug requerido"), nil
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	var projectID *uuid.UUID
	if ps, _ := args["project_slug"].(string); ps != "" {
		proj, err := d.Projects.GetBySlug(ctx, orgID, ps)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", ps)), nil
		}
		projectID = &proj.ID
	}
	p, err := d.Prompts.GetActive(ctx, orgID, projectID, slug)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get_active: %v", err)), nil
	}
	return toolResultJSON(p)
}

func (d *Deps) handlePromptSearch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Prompts == nil {
		return mcp.NewToolResultError("prompt service no configurado"), nil
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
	results, err := d.Prompts.Search(ctx, orgID, query, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"results": results,
		"count":   len(results),
	})
}

func (d *Deps) handlePromptRender(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Prompts == nil {
		return mcp.NewToolResultError("prompt service no configurado"), nil
	}
	args := req.GetArguments()
	slug, _ := args["slug"].(string)
	if slug == "" {
		return mcp.NewToolResultError("slug requerido"), nil
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	var projectID *uuid.UUID
	if ps, _ := args["project_slug"].(string); ps != "" {
		proj, err := d.Projects.GetBySlug(ctx, orgID, ps)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", ps)), nil
		}
		projectID = &proj.ID
	}
	p, err := d.Prompts.GetActive(ctx, orgID, projectID, slug)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get_active: %v", err)), nil
	}
	body := p.Body
	vars, _ := args["variables"].(map[string]any)
	for k, v := range vars {
		body = stringsReplaceAll(body, "{{"+k+"}}", fmt.Sprint(v))
	}
	return toolResultJSON(map[string]any{
		"slug":    p.Slug,
		"version": p.Version,
		"body":    body,
	})
}

// stringsReplaceAll wrapper para evitar import al package strings desde server.go
// (ya existe en otros files, dejo wrapper local).
func stringsReplaceAll(s, old, new string) string {
	out := ""
	for {
		i := indexOf(s, old)
		if i < 0 {
			return out + s
		}
		out += s[:i] + new
		s = s[i+len(old):]
	}
}

func indexOf(s, sub string) int {
	if len(sub) == 0 {
		return 0
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
