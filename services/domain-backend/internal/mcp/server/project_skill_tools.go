// REQ-44 tools MCP para skills scoped a proyecto. Conviven con los tools
// globales (skill_list/skill_search/skill_get): estos nuevos agregan
// scope = project + fallback automático a globales (project_id IS NULL).
//
// La migration 000107 ya agregó skills.project_id NULL-able + indexes.
// No tocamos el service skill existente — usamos queries SQL directas
// con RLS-aware q(ctx) (toma tx del context si está).
package mcpserver

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"
)

func registerProjectSkillTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	rls := func(h mcpgo.ToolHandlerFunc) mcpgo.ToolHandlerFunc {
		return withOrgTxHandler(&deps, h)
	}
	return []mcpgo.ServerTool{
		{Tool: toolProjectSkillRegister(), Handler: wrap.Wrap("domain_project_skill_register", rls(deps.handleProjectSkillRegister))},
		{Tool: toolProjectSkillList(), Handler: wrap.Wrap("domain_project_skill_list", rls(deps.handleProjectSkillList))},
	}
}

func toolProjectSkillRegister() mcp.Tool {
	return mcp.NewTool("domain_project_skill_register",
		mcp.WithDescription("Registra una skill específica de un proyecto (project_id NOT NULL). Mismo slug puede convivir con una skill global. Útil cuando el LLM aprende un patrón propio del proyecto y quiere persistirlo."),
		mcp.WithString("project_slug", mcp.Description("Proyecto al que pertenece"), mcp.Required()),
		mcp.WithString("slug", mcp.Description("Slug de la skill (kebab-case)"), mcp.Required()),
		mcp.WithString("name", mcp.Description("Nombre legible"), mcp.Required()),
		mcp.WithString("description", mcp.Description("Descripción 1-2 líneas. Sirve al matching de skill_search.")),
		mcp.WithString("skill_type", mcp.Description("prompt|code|api|mcp_tool. Default: prompt")),
		mcp.WithString("content", mcp.Description("Cuerpo de la skill (template prompt, código, etc).")),
	)
}

func (d *Deps) handleProjectSkillRegister(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if d.Projects == nil || d.Pool == nil {
		return mcp.NewToolResultError("projects service / pool not configured"), nil
	}
	args := req.GetArguments()
	projSlug, _ := args["project_slug"].(string)
	slug, _ := args["slug"].(string)
	name, _ := args["name"].(string)
	if projSlug == "" || slug == "" || name == "" {
		return mcp.NewToolResultError("project_slug, slug y name requeridos"), nil
	}
	skillType, _ := args["skill_type"].(string)
	if skillType == "" {
		skillType = "prompt"
	}
	desc, _ := args["description"].(string)
	content, _ := args["content"].(string)

	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	proj, perr := d.Projects.GetBySlug(ctx, orgID, projSlug)
	if perr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", projSlug)), nil
	}

	var id uuid.UUID
	err := d.q(ctx).QueryRow(ctx,
		`INSERT INTO skills
		   (organization_id, project_id, slug, name, description,
		    skill_type, content, input_schema, output_schema)
		 VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,NULLIF($7,''),'{}','{}')
		 RETURNING id`,
		orgID, proj.ID, slug, name, desc, skillType, content,
	).Scan(&id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("register failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"id": id.String(), "scope": "project", "project_slug": projSlug,
		"slug": slug, "name": name, "skill_type": skillType,
	})
}

func toolProjectSkillList() mcp.Tool {
	return mcp.NewTool("domain_project_skill_list",
		mcp.WithDescription("Lista skills disponibles para un proyecto: las propias del proyecto (project_id = proj) Y las globales de la org (project_id IS NULL). Devuelve un flag scope por cada item. include_globals=false para ver solo las del proyecto."),
		mcp.WithString("project_slug", mcp.Description("Proyecto a consultar"), mcp.Required()),
		mcp.WithBoolean("include_globals", mcp.Description("Si true (default), incluye las globales de la org. Si false, solo las del proyecto.")),
	)
}

func (d *Deps) handleProjectSkillList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if d.Projects == nil || d.Pool == nil {
		return mcp.NewToolResultError("projects service / pool not configured"), nil
	}
	args := req.GetArguments()
	projSlug, _ := args["project_slug"].(string)
	if projSlug == "" {
		return mcp.NewToolResultError("project_slug requerido"), nil
	}
	includeGlobals := true
	if v, ok := args["include_globals"].(bool); ok {
		includeGlobals = v
	}

	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	proj, perr := d.Projects.GetBySlug(ctx, orgID, projSlug)
	if perr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", projSlug)), nil
	}

	q := `SELECT id, slug, name, COALESCE(description,''), skill_type,
		    CASE WHEN project_id IS NULL THEN 'global' ELSE 'project' END AS scope
		   FROM skills
		   WHERE organization_id = $1
		     AND deleted_at IS NULL
		     AND proposed = false
		     AND (project_id = $2`
	if includeGlobals {
		q += ` OR project_id IS NULL`
	}
	q += `) ORDER BY (project_id IS NULL) ASC, slug ASC`

	rows, err := d.q(ctx).Query(ctx, q, orgID, proj.ID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list failed: %v", err)), nil
	}
	defer rows.Close()

	type item struct {
		ID          string `json:"id"`
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Description string `json:"description"`
		SkillType   string `json:"skill_type"`
		Scope       string `json:"scope"`
	}
	out := make([]item, 0)
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.ID, &it.Slug, &it.Name, &it.Description, &it.SkillType, &it.Scope); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("scan failed: %v", err)), nil
		}
		out = append(out, it)
	}
	return toolResultJSON(map[string]any{
		"skills":          out,
		"total":           len(out),
		"include_globals": includeGlobals,
	})
}

// nota: domain_skill_get y domain_skill_execute existentes resuelven por
// slug global. Si el LLM quiere ejecutar una project-skill, debe pasar
// el id explícito (futuro: extender execute para aceptar project_slug
// y resolver slug → id en el scope correcto).
var _ context.Context // silenciar import si vacío
