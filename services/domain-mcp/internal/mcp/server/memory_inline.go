package mcpserver

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"

	obssvc "nunezlagos/domain/internal/service/observation"
	projsvc "nunezlagos/domain/internal/service/project"
)

func (d *Deps) handleMemSave(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	args := req.GetArguments()

	projectSlug, _ := args["project_slug"].(string)
	content, _ := args["content"].(string)
	obsType, _ := args["observation_type"].(string)

	if projectSlug == "" || content == "" {
		return mcp.NewToolResultError("project_slug y content son requeridos"), nil
	}

	orgID, err := uuid.Parse(d.Principal.OrganizationID)
	if err != nil {
		return mcp.NewToolResultError("invalid principal org_id"), nil
	}
	userID, _ := uuid.Parse(d.Principal.UserID)

	proj, err := d.Projects.GetBySlug(ctx, orgID, projectSlug)
	if err != nil {


		proj, err = d.Projects.Create(ctx, projsvc.CreateInput{
			OrganizationID: orgID,
			Name:           projectSlug,
			Slug:           projectSlug,
			ActorID:        userID,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found and auto-create failed: %v", projectSlug, err)), nil
		}
	}

	var tags []string
	if v, ok := args["tags"].([]any); ok {
		for _, t := range v {
			if s, ok := t.(string); ok {
				tags = append(tags, s)
			}
		}
	}
	var metadata map[string]any
	if v, ok := args["metadata"].(map[string]any); ok {
		metadata = v
	}

	obs, err := d.Observations.Save(ctx, obssvc.SaveInput{
		OrganizationID:  orgID,
		ProjectID:       proj.ID,
		CreatedBy:       &userID,
		Content:         content,
		ObservationType: obsType,
		Tags:            tags,
		Metadata:        metadata,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("save failed: %v", err)), nil
	}

	return toolResultJSON(map[string]any{
		"id":         obs.ID.String(),
		"project_id": obs.ProjectID.String(),
		"created_at": obs.CreatedAt,
		"message":    "observation saved",
	})
}

func (d *Deps) handleMemSearch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
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
	rerank, _ := args["rerank"].(bool)
	rerankTopN := 0
	if v, ok := args["rerank_top_n"].(float64); ok {
		rerankTopN = int(v)
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	// SearchHybridReranked degrada al orden BM25/RRF si rerank=false o si el LLM
	// no esta disponible/falla — nunca devuelve error por culpa del rerank.
	results, err := d.Observations.SearchHybridReranked(ctx, orgID, query, limit, obssvc.RerankOptions{
		Enabled: rerank,
		TopN:    rerankTopN,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}
	out := make([]map[string]any, 0, len(results))
	for _, r := range results {
		out = append(out, map[string]any{
			"id":               r.ID.String(),
			"content":          r.Content,
			"observation_type": r.ObservationType,
			"tags":             r.Tags,
			"score":            r.Score,
			"bm25_rank":        r.BM25Rank,
			"vector_rank":      r.VectorRank,
			"created_at":       r.CreatedAt,
		})
	}
	return toolResultJSON(map[string]any{
		"results": out,
		"count":   len(out),
	})
}

func (d *Deps) handleMemContext(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	args := req.GetArguments()
	projectSlug, _ := args["project_slug"].(string)
	if projectSlug == "" {
		return mcp.NewToolResultError("project_slug requerido"), nil
	}
	limit := 20
	if v, ok := args["limit"].(float64); ok {
		limit = int(v)
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	proj, err := d.Projects.GetBySlug(ctx, orgID, projectSlug)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project not found: %v", err)), nil
	}
	obs, err := d.Observations.List(ctx, proj.ID, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list failed: %v", err)), nil
	}
	out := make([]map[string]any, 0, len(obs))
	for _, o := range obs {
		out = append(out, map[string]any{
			"id":               o.ID.String(),
			"content":          o.Content,
			"observation_type": o.ObservationType,
			"tags":             o.Tags,
			"created_at":       o.CreatedAt,
		})
	}
	return toolResultJSON(map[string]any{
		"project_slug": projectSlug,
		"results":      out,
		"count":        len(out),
	})
}

func (d *Deps) handleMemGetObservation(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("invalid id (UUID expected)"), nil
	}
	obs, err := d.Observations.Get(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get failed: %v", err)), nil
	}
	return toolResultJSON(obs)
}
