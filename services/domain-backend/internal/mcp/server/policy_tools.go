// REQ-43 — tools MCP de policies:
//   - domain_policy_get extiende para resolver jerárquico:
//     project_policies → platform_policies (fallback). Si se pasa
//     project_slug, devuelve la del proyecto (si existe) o la global.
//   - domain_policy_list lista platform policies (sin scope).
//   - domain_project_policy_* CRUD scoped a (org, project).
package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	projectpolicysvc "nunezlagos/domain/internal/service/projectpolicy"
)

func registerPolicyTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	// project_policies tiene RLS FORCE — necesita tx con SET LOCAL.
	rls := func(h mcpgo.ToolHandlerFunc) mcpgo.ToolHandlerFunc {
		return withOrgTxHandler(&deps, h)
	}
	return []mcpgo.ServerTool{
		// platform-level (sin scope project)
		{Tool: toolPolicyGet(), Handler: wrap.Wrap("domain_policy_get", rls(deps.handlePolicyGet))},
		{Tool: toolPolicyList(), Handler: wrap.Wrap("domain_policy_list", deps.handlePolicyList)},
		// project-level
		{Tool: toolProjectPolicySet(), Handler: wrap.Wrap("domain_project_policy_set", rls(deps.handleProjectPolicySet))},
		{Tool: toolProjectPolicyList(), Handler: wrap.Wrap("domain_project_policy_list", rls(deps.handleProjectPolicyList))},
		{Tool: toolProjectPolicyDelete(), Handler: wrap.Wrap("domain_project_policy_delete", rls(deps.handleProjectPolicyDelete))},
		{Tool: toolProjectPolicyImport(), Handler: wrap.Wrap("domain_project_policy_import_from_text", rls(deps.handleProjectPolicyImport))},
	}
}

func toolPolicyGet() mcp.Tool {
	return mcp.NewTool("domain_policy_get",
		mcp.WithDescription("Obtiene una policy resolviendo jerárquicamente: si pasás project_slug y existe una project_policy con ese slug, la devuelve (con flag scope='project'). Si no, fallback a platform_policies (scope='platform'). Llamar ANTES de tocar código del dominio."),
		mcp.WithString("slug",
			mcp.Description("Slug de la policy (ej: 'go', 'db', 'testing', 'git_workflow', 'tech_stack')"),
			mcp.Required(),
		),
		mcp.WithString("project_slug",
			mcp.Description("Slug del proyecto en cuyo contexto se busca la policy. Si se omite, solo busca platform."),
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

	// 1. Si hay project_slug y servicios disponibles, intentar project-scope.
	if projSlug, _ := args["project_slug"].(string); projSlug != "" && d.Projects != nil && d.ProjectPolicies != nil && d.Principal != nil {
		orgID, perr := uuid.Parse(d.Principal.OrganizationID)
		if perr == nil {
			proj, perr := d.Projects.GetBySlug(ctx, orgID, projSlug)
			if perr == nil && proj != nil {
				pol, perr := d.ProjectPolicies.GetBySlug(ctx, orgID, proj.ID, slug)
				if perr == nil && pol != nil {
					// Si override_platform=true, devolver solo project.
					// Si false, intentar también la platform y concatenar.
					payload := map[string]any{
						"scope":             "project",
						"project_slug":      projSlug,
						"slug":              pol.Slug,
						"name":              pol.Name,
						"kind":              pol.Kind,
						"version":           pol.Version,
						"body_md":           pol.BodyMD,
						"override_platform": pol.OverridePlatform,
					}
					if !pol.OverridePlatform {
						if base, berr := d.Policies.GetBySlug(ctx, slug); berr == nil && base != nil {
							payload["platform_body_md"] = base.BodyMD
							payload["platform_version"] = base.Version
							payload["scope"] = "merged"
						}
					}
					return toolResultJSON(payload)
				}
			}
		}
	}

	// 2. Fallback platform.
	p, err := d.Policies.GetBySlug(ctx, slug)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("policy '%s' not found", slug)), nil
	}
	return toolResultJSON(map[string]any{
		"scope":   "platform",
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
			mcp.Description("Filtrar por kind (ej: 'convention','security_rule','architecture','sdd_workflow'); vacío = todas"),
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

// --- project-scope tools ---

func toolProjectPolicySet() mcp.Tool {
	return mcp.NewTool("domain_project_policy_set",
		mcp.WithDescription("Crea o actualiza una policy específica de un proyecto (override o extensión de la platform_policy del mismo slug). source='llm_generated' si fue auto-generada por el LLM aprendiendo del proyecto."),
		mcp.WithString("project_slug", mcp.Description("Proyecto al que pertenece"), mcp.Required()),
		mcp.WithString("slug", mcp.Description("Slug de la policy (mismo namespace que platform — si coincide, este aplica para el proyecto)"), mcp.Required()),
		mcp.WithString("name", mcp.Description("Nombre legible"), mcp.Required()),
		mcp.WithString("kind", mcp.Description("convention|security_rule|architecture|sdd_workflow|observability|migration_rule|linter_config|agent_protocol|git_workflow|tech_stack|test_strategy"), mcp.Required()),
		mcp.WithString("body_md", mcp.Description("Cuerpo Markdown"), mcp.Required()),
		mcp.WithBoolean("override_platform", mcp.Description("Si true, reemplaza la platform_policy del mismo slug. Si false (default), el LLM ve ambas concatenadas.")),
		mcp.WithString("source", mcp.Description("manual|llm_generated|seed_imported|dashboard. Default: manual")),
	)
}

func (d *Deps) handleProjectPolicySet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if d.ProjectPolicies == nil || d.Projects == nil {
		return mcp.NewToolResultError("project_policy service not configured"), nil
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	args := req.GetArguments()
	projSlug, _ := args["project_slug"].(string)
	slug, _ := args["slug"].(string)
	name, _ := args["name"].(string)
	kind, _ := args["kind"].(string)
	body, _ := args["body_md"].(string)
	if projSlug == "" || slug == "" || name == "" || kind == "" || body == "" {
		return mcp.NewToolResultError("project_slug, slug, name, kind y body_md son requeridos"), nil
	}
	override, _ := args["override_platform"].(bool)
	source, _ := args["source"].(string)
	if source == "" {
		source = "manual"
	}

	proj, perr := d.Projects.GetBySlug(ctx, orgID, projSlug)
	if perr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", projSlug)), nil
	}

	// Upsert: si existe activa con mismo slug, update; sino, create.
	existing, _ := d.ProjectPolicies.GetBySlug(ctx, orgID, proj.ID, slug)
	if existing != nil {
		upd := projectpolicysvc.UpdateInput{
			Name:             &name,
			Kind:             &kind,
			BodyMD:           &body,
			OverridePlatform: &override,
		}
		userID, _ := uuid.Parse(d.Principal.UserID)
		updated, err := d.ProjectPolicies.Update(ctx, orgID, existing.ID, upd, &userID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("update failed: %v", err)), nil
		}
		return toolResultJSON(updated)
	}

	created, err := d.ProjectPolicies.Create(ctx, projectpolicysvc.CreateInput{
		OrganizationID:   orgID,
		ProjectID:        proj.ID,
		Slug:             slug,
		Name:             name,
		Kind:             kind,
		BodyMD:           body,
		OverridePlatform: override,
		Source:           source,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("create failed: %v", err)), nil
	}
	return toolResultJSON(created)
}

func toolProjectPolicyList() mcp.Tool {
	return mcp.NewTool("domain_project_policy_list",
		mcp.WithDescription("Lista las project_policies activas de un proyecto (opcional filtrar por kind)."),
		mcp.WithString("project_slug", mcp.Description("Proyecto a consultar"), mcp.Required()),
		mcp.WithString("kind", mcp.Description("Filtrar por kind. Vacío = todas")),
	)
}

func (d *Deps) handleProjectPolicyList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if d.ProjectPolicies == nil || d.Projects == nil {
		return mcp.NewToolResultError("project_policy service not configured"), nil
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	args := req.GetArguments()
	projSlug, _ := args["project_slug"].(string)
	kind, _ := args["kind"].(string)
	if projSlug == "" {
		return mcp.NewToolResultError("project_slug requerido"), nil
	}
	proj, perr := d.Projects.GetBySlug(ctx, orgID, projSlug)
	if perr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", projSlug)), nil
	}
	list, err := d.ProjectPolicies.List(ctx, orgID, proj.ID, kind)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"policies": list, "total": len(list)})
}

func toolProjectPolicyDelete() mcp.Tool {
	return mcp.NewTool("domain_project_policy_delete",
		mcp.WithDescription("Soft-delete una project_policy. La policy queda inactiva — el resolver caerá al platform fallback."),
		mcp.WithString("id", mcp.Description("UUID de la project_policy"), mcp.Required()),
	)
}

func toolProjectPolicyImport() mcp.Tool {
	return mcp.NewTool("domain_project_policy_import_from_text",
		mcp.WithDescription("Importa un AGENTS.md / CLAUDE.md / .cursorrules / openspec / etc. del repo como project_policy del proyecto, con source='seed_imported'. El LLM lee el archivo con su tool Read y pasa el body acá. Útil para que domain herede lo que el repo ya documenta SIN PISAR nada del archivo original — solo persiste una copia importada como policy versionada."),
		mcp.WithString("project_slug", mcp.Description("Proyecto al que pertenece"), mcp.Required()),
		mcp.WithString("source_path", mcp.Description("Path relativo en el repo del archivo origen (ej. AGENTS.md, .claude/CLAUDE.md). Se usa para construir slug y name."), mcp.Required()),
		mcp.WithString("body_md", mcp.Description("Contenido completo del archivo (lo que devolvió tu tool Read)"), mcp.Required()),
		mcp.WithString("kind", mcp.Description("Tipo (default 'convention'). Valores: convention|architecture|sdd_workflow|git_workflow|tech_stack|test_strategy|agent_protocol")),
	)
}

func slugFromSourcePath(p string) string {
	// AGENTS.md → imported-agents
	// .claude/CLAUDE.md → imported-claude
	// .cursorrules → imported-cursorrules
	// .windsurf/rules/foo.md → imported-windsurf-rules-foo
	cleaned := strings.ReplaceAll(p, "/", "-")
	cleaned = strings.TrimPrefix(cleaned, ".")
	cleaned = strings.ReplaceAll(cleaned, ".md", "")
	cleaned = strings.ToLower(cleaned)
	return "imported-" + cleaned
}

func (d *Deps) handleProjectPolicyImport(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if d.ProjectPolicies == nil || d.Projects == nil {
		return mcp.NewToolResultError("project_policy service not configured"), nil
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	args := req.GetArguments()
	projSlug, _ := args["project_slug"].(string)
	sourcePath, _ := args["source_path"].(string)
	body, _ := args["body_md"].(string)
	if projSlug == "" || sourcePath == "" || body == "" {
		return mcp.NewToolResultError("project_slug, source_path y body_md son requeridos"), nil
	}
	kind, _ := args["kind"].(string)
	if kind == "" {
		kind = "convention"
	}

	proj, perr := d.Projects.GetBySlug(ctx, orgID, projSlug)
	if perr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", projSlug)), nil
	}

	slug := slugFromSourcePath(sourcePath)
	name := "Imported: " + sourcePath

	// Upsert: si ya hay una imported con mismo slug, update (bumpea version
	// con snapshot a project_policy_versions). Idempotente — re-importar
	// el mismo archivo crea una versión nueva en lugar de duplicar.
	existing, _ := d.ProjectPolicies.GetBySlug(ctx, orgID, proj.ID, slug)
	if existing != nil {
		userID, _ := uuid.Parse(d.Principal.UserID)
		upd := projectpolicysvc.UpdateInput{
			Name:   &name,
			Kind:   &kind,
			BodyMD: &body,
		}
		updated, err := d.ProjectPolicies.Update(ctx, orgID, existing.ID, upd, &userID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("import update failed: %v", err)), nil
		}
		return toolResultJSON(map[string]any{
			"action":      "updated",
			"slug":        slug,
			"source_path": sourcePath,
			"version":     updated.Version,
			"id":          updated.ID.String(),
		})
	}

	created, err := d.ProjectPolicies.Create(ctx, projectpolicysvc.CreateInput{
		OrganizationID: orgID,
		ProjectID:      proj.ID,
		Slug:           slug,
		Name:           name,
		Kind:           kind,
		BodyMD:         body,
		Source:         "seed_imported",
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("import failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"action":      "created",
		"slug":        slug,
		"source_path": sourcePath,
		"version":     created.Version,
		"id":          created.ID.String(),
	})
}

func (d *Deps) handleProjectPolicyDelete(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if d.ProjectPolicies == nil {
		return mcp.NewToolResultError("project_policy service not configured"), nil
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	args := req.GetArguments()
	idStr, _ := args["id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("id inválido"), nil
	}
	if err := d.ProjectPolicies.Delete(ctx, orgID, id); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("delete failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"id": id.String(), "deleted": true})
}
