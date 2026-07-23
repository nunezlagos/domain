// DOMAINSERV-93 A — tool MCP de administración de proyectos:
// domain_project_delete (soft-delete con guard HasData). B (domain_project_merge)
// se descopó: el servicio projectmerge.Service quedó obsoleto contra el schema
// actual (skills/flows/agents/crons perdieron project_id en la migración 000160)
// — su reescritura es un ticket aparte.
package mcpserver

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
)

func toolProjectDelete() mcp.Tool {
	return mcp.NewTool("domain_project_delete",
		mcp.WithDescription("Soft-delete de un proyecto. Guard: rechaza si el proyecto tiene contenido (observations/tickets/knowledge/skills/policies/prompts/workflows) salvo force=true. Reversible (deleted_at)."),
		mcp.WithString("slug",
			mcp.Description("Slug del proyecto a borrar"),
			mcp.Required(),
		),
		mcp.WithBoolean("force",
			mcp.Description("Si true, borra aunque el proyecto tenga datos. Default false."),
		),
	)
}

func (h *catalogHandlers) handleProjectDelete(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.projects == nil {
		return mcp.NewToolResultError("project service no configurado"), nil
	}
	args := req.GetArguments()
	slug, _ := args["slug"].(string)
	if slug == "" {
		return mcp.NewToolResultError("slug es requerido"), nil
	}
	force, _ := args["force"].(bool)

	orgID, err := uuid.Parse(h.principal.OrganizationID)
	if err != nil {
		return mcp.NewToolResultError("invalid principal org_id"), nil
	}
	proj, err := h.projects.GetBySlug(ctx, orgID, slug)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", slug)), nil
	}

	hasData, err := h.projects.HasData(ctx, proj.ID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("check project data: %v", err)), nil
	}
	if hasData && !force {
		return mcp.NewToolResultError(fmt.Sprintf(
			"project '%s' tiene contenido; pasá force=true para borrarlo igual (soft-delete, reversible)", slug)), nil
	}

	var actorID uuid.UUID
	if uid, uerr := uuid.Parse(h.principal.UserID); uerr == nil {
		actorID = uid
	}
	if err := h.projects.SoftDelete(ctx, proj.ID, actorID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("soft-delete: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"deleted":  true,
		"slug":     slug,
		"had_data": hasData,
		"forced":   force,
	})
}
