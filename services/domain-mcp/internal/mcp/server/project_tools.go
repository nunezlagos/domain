// Tools MCP de projects: descubrimiento para que el agente sepa que
// project_slug usar en mem_save/search (issue-12.2 follow-up). El
// mem_save ademas auto-crea el project si no existe.
//
// REQ-28.2: extiende domain_project_list para aceptar client_slug como
// filtro opcional. Los tools de create/update viven en catalog_tools.go.
package mcpserver

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"
)

func registerProjectTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {


	rls := func(h mcpgo.ToolHandlerFunc) mcpgo.ToolHandlerFunc {
		return withOrgTxHandler(&deps, h)
	}
	return []mcpgo.ServerTool{
		{Tool: toolProjectList(), Handler: wrap.Wrap("domain_project_list", rls(deps.handleProjectList))},
	}
}

func toolProjectList() mcp.Tool {
	return mcp.NewTool("domain_project_list",
		mcp.WithDescription("Lista los projects de la organizacion (slug + nombre). Usa el slug en domain_mem_save/domain_mem_search; si guardas con un slug nuevo, el project se crea solo. REQ-28.2: filtra por client_slug para ver solo los proyectos asociados a un mandante."),
		mcp.WithString("client_slug",
			mcp.Description("Opcional: filtra projects asociados al client con este slug."),
		),
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
	args := req.GetArguments()
	clientSlug, _ := args["client_slug"].(string)

	var projects []struct {
		Slug        string
		Name        string
		Description string
		ClientSlug  string
		ClientName  string
	}
	if clientSlug != "" {
		list, err := d.Projects.ListFiltered(ctx, orgID, clientSlug)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("list projects failed: %v", err)), nil
		}
		for _, p := range list {
			projects = append(projects, struct {
				Slug, Name, Description, ClientSlug, ClientName string
			}{p.Slug, p.Name, p.Description, p.ClientSlug, p.ClientName})
		}
	} else {
		list, err := d.Projects.List(ctx, orgID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("list projects failed: %v", err)), nil
		}
		for _, p := range list {
			projects = append(projects, struct {
				Slug, Name, Description, ClientSlug, ClientName string
			}{p.Slug, p.Name, p.Description, p.ClientSlug, p.ClientName})
		}
	}

	out := make([]map[string]any, 0, len(projects))
	for _, p := range projects {
		row := map[string]any{
			"slug": p.Slug, "name": p.Name, "description": p.Description,
		}
		if p.ClientSlug != "" {
			row["client_slug"] = p.ClientSlug
			row["client_name"] = p.ClientName
		}
		out = append(out, row)
	}
	return toolResultJSON(map[string]any{"projects": out, "total": len(out)})
}
