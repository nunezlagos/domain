package mcpserver

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"

	knowsvc "nunezlagos/domain/internal/service/knowledge"
)

func (d *Deps) handleKnowledgeSave(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Knowledge == nil {
		return mcp.NewToolResultError("knowledge service no configurado"), nil
	}
	args := req.GetArguments()
	slug, _ := args["project_slug"].(string)
	title, _ := args["title"].(string)
	body, _ := args["body"].(string)
	if slug == "" || title == "" || body == "" {
		return mcp.NewToolResultError("project_slug, title y body son requeridos"), nil
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	userID, _ := uuid.Parse(d.Principal.UserID)
	proj, err := d.Projects.GetBySlug(ctx, orgID, slug)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", slug)), nil
	}
	var tags []string
	if v, ok := args["tags"].([]any); ok {
		for _, t := range v {
			if s, ok := t.(string); ok {
				tags = append(tags, s)
			}
		}
	}
	source, _ := args["source"].(string)
	sourceURL, _ := args["source_url"].(string)
	doc, chunks, err := d.Knowledge.Save(ctx, knowsvc.SaveInput{
		OrganizationID: orgID, ProjectID: proj.ID, CreatedBy: &userID,
		Title: title, Body: body, Source: source, SourceURL: sourceURL, Tags: tags,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("save: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"id":           doc.ID.String(),
		"chunks_count": len(chunks),
		"created_at":   doc.CreatedAt,
	})
}

func (d *Deps) handleKnowledgeSearch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Knowledge == nil {
		return mcp.NewToolResultError("knowledge service no configurado"), nil
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
	results, err := d.Knowledge.SearchHybrid(ctx, orgID, query, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"results": results,
		"count":   len(results),
	})
}

func (d *Deps) handleKnowledgeGet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Knowledge == nil {
		return mcp.NewToolResultError("knowledge service no configurado"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("id invalido (UUID)"), nil
	}
	doc, chunks, err := d.Knowledge.Get(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"document": doc,
		"chunks":   chunks,
	})
}
