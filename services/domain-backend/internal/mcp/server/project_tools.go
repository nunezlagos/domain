// Tools MCP de projects: descubrimiento para que el agente sepa qué
// project_slug usar en mem_save/search (issue-12.2 follow-up). El
// mem_save además auto-crea el project si no existe.
package mcpserver

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"
)

func registerProjectTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	return []mcpgo.ServerTool{
		{Tool: toolProjectList(), Handler: wrap.Wrap("domain_project_list", deps.handleProjectList)},
	}
}

func toolProjectList() mcp.Tool {
	return mcp.NewTool("domain_project_list",
		mcp.WithDescription("Lista los projects de la organización (slug + nombre). Usá el slug en domain_mem_save/domain_mem_search; si guardás con un slug nuevo, el project se crea solo."),
	)
}

func (d *Deps) handleProjectList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Projects == nil {
		return mcp.NewToolResultError("project service not configured"), nil
	}
	orgID, err := uuid.Parse(d.Principal.OrganizationID)
	if err != nil {
		return mcp.NewToolResultError("invalid principal org_id"), nil
	}
	projects, err := d.Projects.List(ctx, orgID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list projects failed: %v", err)), nil
	}
	out := make([]map[string]any, 0, len(projects))
	for _, p := range projects {
		out = append(out, map[string]any{
			"slug": p.Slug, "name": p.Name, "description": p.Description,
		})
	}
	return toolResultJSON(map[string]any{"projects": out, "total": len(out)})
}
