// issue-01.8 — tools MCP de platform policies: las rules SDD/conventions
// viven en BD como source-of-truth y el agente las consulta por slug.
package mcpserver

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"
)

func registerPolicyTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	return []mcpgo.ServerTool{
		{Tool: toolPolicyGet(), Handler: wrap.Wrap("domain_policy_get", deps.handlePolicyGet)},
		{Tool: toolPolicyList(), Handler: wrap.Wrap("domain_policy_list", deps.handlePolicyList)},
	}
}

func toolPolicyGet() mcp.Tool {
	return mcp.NewTool("domain_policy_get",
		mcp.WithDescription("Obtiene una platform policy (rule SDD, convention, guideline) por slug desde la BD — source-of-truth versionado. Consultá la policy del dominio ANTES de tocar código relacionado."),
		mcp.WithString("slug",
			mcp.Description("Slug de la policy (ej: 'go', 'db', 'testing', 'sdd', 'security')"),
			mcp.Required(),
		),
	)
}

func (d *Deps) handlePolicyGet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Policies == nil {
		return mcp.NewToolResultError("policy service not configured"), nil
	}
	args := req.GetArguments()
	slug, _ := args["slug"].(string)
	if slug == "" {
		return mcp.NewToolResultError("slug es requerido"), nil
	}
	p, err := d.Policies.GetBySlug(ctx, slug)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("policy '%s' not found", slug)), nil
	}
	return toolResultJSON(map[string]any{
		"slug":    p.Slug,
		"name":    p.Name,
		"kind":    p.Kind,
		"version": p.Version,
		"body_md": p.BodyMD,
	})
}

func toolPolicyList() mcp.Tool {
	return mcp.NewTool("domain_policy_list",
		mcp.WithDescription("Lista las platform policies activas (slug + nombre + kind + versión). Útil para descubrir qué rules existen antes de pedirlas con domain_policy_get."),
		mcp.WithString("kind",
			mcp.Description("Filtrar por kind (ej: 'rule', 'convention', 'template'); vacío = todas"),
		),
	)
}

func (d *Deps) handlePolicyList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Policies == nil {
		return mcp.NewToolResultError("policy service not configured"), nil
	}
	args := req.GetArguments()
	kind, _ := args["kind"].(string)
	policies, err := d.Policies.List(ctx, kind)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list policies failed: %v", err)), nil
	}
	out := make([]map[string]any, 0, len(policies))
	for _, p := range policies {
		out = append(out, map[string]any{
			"slug": p.Slug, "name": p.Name, "kind": p.Kind, "version": p.Version,
		})
	}
	return toolResultJSON(map[string]any{"policies": out, "total": len(out)})
}
