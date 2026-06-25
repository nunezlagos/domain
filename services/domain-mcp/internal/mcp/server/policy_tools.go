// REQ-43 — tools MCP de policies:
//   - domain_policy_get extiende para resolver jerarquico:
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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/store/txctx"
	policysvc "nunezlagos/domain/internal/service/policy"
	projsvc "nunezlagos/domain/internal/service/project"
	projectpolicysvc "nunezlagos/domain/internal/service/projectpolicy"
)

type policiesReader interface {
	GetBySlug(ctx context.Context, slug string) (*policysvc.Policy, error)
	List(ctx context.Context, kind string) ([]policysvc.Policy, error)
}

type projectPoliciesStore interface {
	GetBySlug(ctx context.Context, orgID, projectID uuid.UUID, slug string) (*projectpolicysvc.Policy, error)
	Update(ctx context.Context, orgID, id uuid.UUID, in projectpolicysvc.UpdateInput, userID *uuid.UUID) (*projectpolicysvc.Policy, error)
	Create(ctx context.Context, in projectpolicysvc.CreateInput) (*projectpolicysvc.Policy, error)
	List(ctx context.Context, orgID, projectID uuid.UUID, kind string) ([]*projectpolicysvc.Policy, error)
	Delete(ctx context.Context, orgID, id uuid.UUID) error
}

type projectGetter interface {
	GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*projsvc.Project, error)
}

type policyHandlers struct {
	policies        policiesReader
	projectPolicies projectPoliciesStore
	projects        projectGetter
	pool            *pgxpool.Pool
	principal       *apikey.Principal
}

func (h *policyHandlers) q(ctx context.Context) interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
} {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return tx
	}
	return h.pool
}

func registerPolicyTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	h := &policyHandlers{
		policies:        deps.Policies,
		projectPolicies: deps.ProjectPolicies,
		projects:        deps.Projects,
		pool:            deps.Pool,
		principal:       deps.Principal,
	}
	rls := func(fn mcpgo.ToolHandlerFunc) mcpgo.ToolHandlerFunc {
		return withOrgTxHandler(&deps, fn)
	}
	return []mcpgo.ServerTool{
		{Tool: toolPolicyGet(), Handler: wrap.Wrap("domain_policy_get", rls(h.handlePolicyGet))},
		{Tool: toolPolicyList(), Handler: wrap.Wrap("domain_policy_list", h.handlePolicyList)},
		{Tool: toolPlatformPolicyCreate(), Handler: wrap.Wrap("domain_platform_policy_create", rls(h.handlePlatformPolicyCreate))},
		{Tool: toolPlatformPolicyEdit(), Handler: wrap.Wrap("domain_platform_policy_edit", rls(h.handlePlatformPolicyEdit))},
		{Tool: toolProjectPolicySet(), Handler: wrap.Wrap("domain_project_policy_set", rls(h.handleProjectPolicySet))},
		{Tool: toolProjectPolicyList(), Handler: wrap.Wrap("domain_project_policy_list", rls(h.handleProjectPolicyList))},
		{Tool: toolProjectPolicyDelete(), Handler: wrap.Wrap("domain_project_policy_delete", rls(h.handleProjectPolicyDelete))},
		{Tool: toolProjectPolicyImport(), Handler: wrap.Wrap("domain_project_policy_import_from_text", rls(h.handleProjectPolicyImport))},
	}
}

func toolPolicyGet() mcp.Tool {
	return mcp.NewTool("domain_policy_get",
		mcp.WithDescription("Obtiene una policy resolviendo jerarquicamente: si pasas project_slug y existe una project_policy con ese slug, la devuelve (con flag scope='project'). Si no, fallback a platform_policies (scope='platform'). Llamar ANTES de tocar codigo del dominio."),
		mcp.WithString("slug",
			mcp.Description("Slug de la policy (ej: 'go', 'db', 'testing', 'git_workflow', 'tech_stack')"),
			mcp.Required(),
		),
		mcp.WithString("project_slug",
			mcp.Description("Slug del proyecto en cuyo contexto se busca la policy. Si se omite, solo busca platform."),
		),
	)
}

func (h *policyHandlers) handlePolicyGet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.policies == nil {
		return mcp.NewToolResultError("policy service not configured"), nil
	}
	args := req.GetArguments()
	slug, _ := args["slug"].(string)
	if slug == "" {
		return mcp.NewToolResultError("slug es requerido"), nil
	}

	if projSlug, _ := args["project_slug"].(string); projSlug != "" && h.projects != nil && h.projectPolicies != nil && h.principal != nil {
		orgID, perr := uuid.Parse(h.principal.OrganizationID)
		if perr == nil {
			proj, perr := h.projects.GetBySlug(ctx, orgID, projSlug)
			if perr == nil && proj != nil {
				pol, perr := h.projectPolicies.GetBySlug(ctx, orgID, proj.ID, slug)
				if perr == nil && pol != nil {
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
						if base, berr := h.policies.GetBySlug(ctx, slug); berr == nil && base != nil {
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

	p, err := h.policies.GetBySlug(ctx, slug)
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
		mcp.WithDescription("Lista las platform policies activas (slug + nombre + kind + version). Util para descubrir que rules existen antes de pedirlas con domain_policy_get."),
		mcp.WithString("kind",
			mcp.Description("Filtrar por kind (ej: 'convention','security_rule','architecture','sdd_workflow'); vacio = todas"),
		),
	)
}

func (h *policyHandlers) handlePolicyList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.policies == nil {
		return mcp.NewToolResultError("policy service not configured"), nil
	}
	args := req.GetArguments()
	kind, _ := args["kind"].(string)
	policies, err := h.policies.List(ctx, kind)
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

func toolProjectPolicySet() mcp.Tool {
	return mcp.NewTool("domain_project_policy_set",
		mcp.WithDescription("Crea o actualiza una policy especifica de un proyecto (override o extension de la platform_policy del mismo slug). source='llm_generated' si fue auto-generada por el LLM aprendiendo del proyecto."),
		mcp.WithString("project_slug", mcp.Description("Proyecto al que pertenece"), mcp.Required()),
		mcp.WithString("slug", mcp.Description("Slug de la policy (mismo namespace que platform — si coincide, este aplica para el proyecto)"), mcp.Required()),
		mcp.WithString("name", mcp.Description("Nombre legible"), mcp.Required()),
		mcp.WithString("kind", mcp.Description("convention|security_rule|architecture|sdd_workflow|observability|migration_rule|linter_config|agent_protocol|git_workflow|tech_stack|test_strategy"), mcp.Required()),
		mcp.WithString("body_md", mcp.Description("Cuerpo Markdown"), mcp.Required()),
		mcp.WithBoolean("override_platform", mcp.Description("Si true, reemplaza la platform_policy del mismo slug. Si false (default), el LLM ve ambas concatenadas.")),
		mcp.WithString("source", mcp.Description("manual|llm_generated|seed_imported|dashboard. Default: manual")),
	)
}

func (h *policyHandlers) handleProjectPolicySet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if h.projectPolicies == nil || h.projects == nil {
		return mcp.NewToolResultError("project_policy service not configured"), nil
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
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

	proj, perr := h.projects.GetBySlug(ctx, orgID, projSlug)
	if perr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", projSlug)), nil
	}

	existing, _ := h.projectPolicies.GetBySlug(ctx, orgID, proj.ID, slug)
	if existing != nil {
		upd := projectpolicysvc.UpdateInput{
			Name:             &name,
			Kind:             &kind,
			BodyMD:           &body,
			OverridePlatform: &override,
		}
		userID, _ := uuid.Parse(h.principal.UserID)
		updated, err := h.projectPolicies.Update(ctx, orgID, existing.ID, upd, &userID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("update failed: %v", err)), nil
		}
		return toolResultJSON(updated)
	}

	created, err := h.projectPolicies.Create(ctx, projectpolicysvc.CreateInput{
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
		mcp.WithString("kind", mcp.Description("Filtrar por kind. Vacio = todas")),
	)
}

func (h *policyHandlers) handleProjectPolicyList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if h.projectPolicies == nil || h.projects == nil {
		return mcp.NewToolResultError("project_policy service not configured"), nil
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	args := req.GetArguments()
	projSlug, _ := args["project_slug"].(string)
	kind, _ := args["kind"].(string)
	if projSlug == "" {
		return mcp.NewToolResultError("project_slug requerido"), nil
	}
	proj, perr := h.projects.GetBySlug(ctx, orgID, projSlug)
	if perr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", projSlug)), nil
	}
	list, err := h.projectPolicies.List(ctx, orgID, proj.ID, kind)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"policies": list, "total": len(list)})
}

func toolProjectPolicyDelete() mcp.Tool {
	return mcp.NewTool("domain_project_policy_delete",
		mcp.WithDescription("Soft-delete una project_policy. La policy queda inactiva — el resolver caera al platform fallback."),
		mcp.WithString("id", mcp.Description("UUID de la project_policy"), mcp.Required()),
	)
}

func toolProjectPolicyImport() mcp.Tool {
	return mcp.NewTool("domain_project_policy_import_from_text",
		mcp.WithDescription("Importa un AGENTS.md / CLAUDE.md / .cursorrules / openspec / etc. del repo como project_policy del proyecto, con source='seed_imported'. El LLM lee el archivo con su tool Read y pasa el body aqui. Util para que domain herede lo que el repo ya documenta SIN PISAR nada del archivo original — solo persiste una copia importada como policy versionada."),
		mcp.WithString("project_slug", mcp.Description("Proyecto al que pertenece"), mcp.Required()),
		mcp.WithString("source_path", mcp.Description("Path relativo en el repo del archivo origen (ej. AGENTS.md, .claude/CLAUDE.md). Se usa para construir slug y name."), mcp.Required()),
		mcp.WithString("body_md", mcp.Description("Contenido completo del archivo (lo que devolvio tu tool Read)"), mcp.Required()),
		mcp.WithString("kind", mcp.Description("Tipo (default 'convention'). Valores: convention|architecture|sdd_workflow|git_workflow|tech_stack|test_strategy|agent_protocol")),
	)
}

func slugFromSourcePath(p string) string {
	cleaned := strings.ReplaceAll(p, "/", "-")
	cleaned = strings.TrimPrefix(cleaned, ".")
	cleaned = strings.ReplaceAll(cleaned, ".md", "")
	cleaned = strings.ToLower(cleaned)
	return "imported-" + cleaned
}

func (h *policyHandlers) handleProjectPolicyImport(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if h.projectPolicies == nil || h.projects == nil {
		return mcp.NewToolResultError("project_policy service not configured"), nil
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
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

	proj, perr := h.projects.GetBySlug(ctx, orgID, projSlug)
	if perr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", projSlug)), nil
	}

	slug := slugFromSourcePath(sourcePath)
	name := "Imported: " + sourcePath

	existing, _ := h.projectPolicies.GetBySlug(ctx, orgID, proj.ID, slug)
	if existing != nil {
		userID, _ := uuid.Parse(h.principal.UserID)
		upd := projectpolicysvc.UpdateInput{
			Name:   &name,
			Kind:   &kind,
			BodyMD: &body,
		}
		updated, err := h.projectPolicies.Update(ctx, orgID, existing.ID, upd, &userID)
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

	created, err := h.projectPolicies.Create(ctx, projectpolicysvc.CreateInput{
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

// platformPolicyKinds son los kinds aceptados por el CHECK constraint de
// platform_policies (mig 000045). OJO: project_policies acepta mas kinds
// (agent_protocol, git_workflow, tech_stack, test_strategy) pero la tabla
// global NO — validar aca evita un error SQL opaco.
var platformPolicyKinds = map[string]bool{
	"convention": true, "security_rule": true, "architecture": true,
	"sdd_workflow": true, "observability": true, "migration_rule": true,
	"linter_config": true,
}

func toolPlatformPolicyCreate() mcp.Tool {
	return mcp.NewTool("domain_platform_policy_create",
		mcp.WithDescription("Crea una platform_policy GLOBAL (sin scope de proyecto). Las platform_policies aplican a TODOS los proyectos por defecto (se leen siempre en el system prompt), a diferencia de project_policies que son overrides opt-in. Es el equivalente desde MCP del seeder. Usar para reglas de plataforma transversales."),
		mcp.WithString("slug", mcp.Description("Slug unico (kebab-case)"), mcp.Required()),
		mcp.WithString("name", mcp.Description("Nombre legible"), mcp.Required()),
		mcp.WithString("kind", mcp.Description("convention|security_rule|architecture|sdd_workflow|observability|migration_rule|linter_config"), mcp.Required()),
		mcp.WithString("body_md", mcp.Description("Cuerpo Markdown de la regla"), mcp.Required()),
		mcp.WithString("source_file", mcp.Description("Path de origen opcional (informativo)")),
	)
}

func (h *policyHandlers) handlePlatformPolicyCreate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if h.pool == nil {
		return mcp.NewToolResultError("pool not configured"), nil
	}
	args := req.GetArguments()
	slug, _ := args["slug"].(string)
	name, _ := args["name"].(string)
	kind, _ := args["kind"].(string)
	body, _ := args["body_md"].(string)
	if slug == "" || name == "" || kind == "" || body == "" {
		return mcp.NewToolResultError("slug, name, kind y body_md requeridos"), nil
	}
	if !platformPolicyKinds[kind] {
		return mcp.NewToolResultError(fmt.Sprintf("kind '%s' invalido para platform_policies", kind)), nil
	}
	sourceFile, _ := args["source_file"].(string)

	var id uuid.UUID
	var version int
	err := h.q(ctx).QueryRow(ctx,
		`INSERT INTO platform_policies
		   (slug, name, kind, body_md, source_file, is_active, is_user_modified)
		 VALUES ($1,$2,$3,$4,NULLIF($5,''),TRUE,TRUE)
		 RETURNING id, version`,
		slug, name, kind, body, sourceFile,
	).Scan(&id, &version)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("create failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"id": id.String(), "scope": "platform",
		"slug": slug, "name": name, "kind": kind, "version": version,
		"note": "platform_policy global creada. Aplica a todos los proyectos por defecto.",
	})
}

func toolPlatformPolicyEdit() mcp.Tool {
	return mcp.NewTool("domain_platform_policy_edit",
		mcp.WithDescription("Edita una platform_policy global. Identifica por 'id' (UUID) o 'slug' (la activa). Actualiza SOLO los campos provistos (name/kind/body_md). Si cambia body_md, archiva la version anterior en platform_policy_versions y bumpea version. Marca is_user_modified=TRUE."),
		mcp.WithString("id", mcp.Description("UUID de la policy. Preferido. Si se omite, se usa slug.")),
		mcp.WithString("slug", mcp.Description("Slug de la policy activa. Ignorado si se pasa id.")),
		mcp.WithString("name", mcp.Description("Nuevo nombre. Omitir para no cambiar.")),
		mcp.WithString("kind", mcp.Description("Nuevo kind. Omitir para no cambiar. Valores: convention|security_rule|architecture|sdd_workflow|observability|migration_rule|linter_config")),
		mcp.WithString("body_md", mcp.Description("Nuevo cuerpo Markdown. Omitir para no cambiar. Si cambia, se archiva la version previa.")),
	)
}

func (h *policyHandlers) handlePlatformPolicyEdit(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if h.pool == nil {
		return mcp.NewToolResultError("pool not configured"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["id"].(string)
	slug, _ := args["slug"].(string)
	if idStr == "" && slug == "" {
		return mcp.NewToolResultError("id o slug requerido"), nil
	}

	var (
		newName *string
		newKind *string
		newBody *string
	)
	if v, ok := args["name"].(string); ok && v != "" {
		newName = &v
	}
	if v, ok := args["kind"].(string); ok && v != "" {
		if !platformPolicyKinds[v] {
			return mcp.NewToolResultError(fmt.Sprintf("kind '%s' invalido para platform_policies", v)), nil
		}
		newKind = &v
	}
	if v, ok := args["body_md"].(string); ok && v != "" {
		newBody = &v
	}
	if newName == nil && newKind == nil && newBody == nil {
		return mcp.NewToolResultError("nada para actualizar: pasa al menos un campo"), nil
	}

	var (
		curID      uuid.UUID
		curSlug    string
		curVersion int
		curBody    string
		curStruct  []byte
	)
	lookupSQL := `SELECT id, slug, version, body_md, body_structured
	                FROM platform_policies
	               WHERE %s AND is_active = TRUE`
	var lookupArg any
	if idStr != "" {
		parsed, perr := uuid.Parse(idStr)
		if perr != nil {
			return mcp.NewToolResultError("id invalido (UUID requerido)"), nil
		}
		lookupArg = parsed
		lookupSQL = fmt.Sprintf(lookupSQL, "id = $1")
	} else {
		lookupArg = slug
		lookupSQL = fmt.Sprintf(lookupSQL, "slug = $1")
	}
	if err := h.q(ctx).QueryRow(ctx, lookupSQL, lookupArg).
		Scan(&curID, &curSlug, &curVersion, &curBody, &curStruct); err != nil {
		return mcp.NewToolResultError("platform_policy no encontrada"), nil
	}

	bumpVersion := newBody != nil && *newBody != curBody
	if bumpVersion {
		var changedBy *uuid.UUID
		if uid, uerr := uuid.Parse(h.principal.UserID); uerr == nil {
			changedBy = &uid
		}
		if _, err := h.q(ctx).Exec(ctx,
			`INSERT INTO platform_policy_versions
			   (policy_id, version, body_md, body_structured, changed_by)
			 VALUES ($1,$2,$3,$4,$5)`,
			curID, curVersion, curBody, curStruct, changedBy,
		); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("archive version failed: %v", err)), nil
		}
	}

	var (
		outName, outKind string
		outVersion       int
	)
	versionExpr := "version"
	if bumpVersion {
		versionExpr = "version + 1"
	}
	updSQL := `UPDATE platform_policies
	              SET name             = COALESCE($2, name),
	                  kind             = COALESCE($3, kind),
	                  body_md          = COALESCE($4, body_md),
	                  version          = ` + versionExpr + `,
	                  is_user_modified = TRUE
	            WHERE id = $1 AND is_active = TRUE
	          RETURNING name, kind, version`
	if err := h.q(ctx).QueryRow(ctx, updSQL, curID, newName, newKind, newBody).
		Scan(&outName, &outKind, &outVersion); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("edit failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"id": curID.String(), "scope": "platform",
		"slug": curSlug, "name": outName, "kind": outKind,
		"version": outVersion, "version_bumped": bumpVersion, "updated": true,
	})
}

func (h *policyHandlers) handleProjectPolicyDelete(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if h.projectPolicies == nil {
		return mcp.NewToolResultError("project_policy service not configured"), nil
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	args := req.GetArguments()
	idStr, _ := args["id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("id invalido"), nil
	}
	if err := h.projectPolicies.Delete(ctx, orgID, id); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("delete failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"id": id.String(), "deleted": true})
}
