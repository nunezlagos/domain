// REQ-44 tools MCP para skills scoped a proyecto. Conviven con los tools
// globales (skill_list/skill_search/skill_get): estos nuevos agregan
// scope = project + fallback automatico a globales (project_id IS NULL).
//
// La migration 000107 ya agrego skills.project_id NULL-able + indexes.
// No tocamos el service skill existente — usamos queries SQL directas
// con RLS-aware q(ctx) (toma tx del context si esta).
package mcpserver

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"nunezlagos/domain/internal/auth/apikey"
	projsvc "nunezlagos/domain/internal/service/project"
	"nunezlagos/domain/internal/store/txctx"
)

type skillProjectGetter interface {
	GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*projsvc.Project, error)
}

type projectSkillHandlers struct {
	projects  skillProjectGetter
	pool      *pgxpool.Pool
	principal *apikey.Principal
}

func (h *projectSkillHandlers) q(ctx context.Context) interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
} {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return tx
	}
	return h.pool
}

func registerProjectSkillTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	h := &projectSkillHandlers{
		projects:  deps.Projects,
		pool:      deps.Pool,
		principal: deps.Principal,
	}
	rls := func(fn mcpgo.ToolHandlerFunc) mcpgo.ToolHandlerFunc {
		return withOrgTxHandler(&deps, fn)
	}
	return []mcpgo.ServerTool{
		{Tool: toolProjectSkillRegister(), Handler: wrap.Wrap("domain_project_skill_register", rls(h.handleProjectSkillRegister))},
		{Tool: toolProjectSkillList(), Handler: wrap.Wrap("domain_project_skill_list", rls(h.handleProjectSkillList))},

		{Tool: toolSkillCreate(), Handler: wrap.Wrap("domain_skill_create", rls(h.handleSkillCreate))},
		{Tool: toolSkillEdit(), Handler: wrap.Wrap("domain_skill_edit", rls(h.handleSkillEdit))},
		{Tool: toolProjectSkillUnlink(), Handler: wrap.Wrap("domain_project_skill_unlink", rls(h.handleProjectSkillUnlink))},
	}
}

func toolProjectSkillRegister() mcp.Tool {
	return mcp.NewTool("domain_project_skill_register",
		mcp.WithDescription("Registra una skill especifica de un proyecto (project_id NOT NULL). Mismo slug puede convivir con una skill global. Util cuando el LLM aprende un patron propio del proyecto y quiere persistirlo."),
		mcp.WithString("project_slug", mcp.Description("Proyecto al que pertenece"), mcp.Required()),
		mcp.WithString("slug", mcp.Description("Slug de la skill (kebab-case)"), mcp.Required()),
		mcp.WithString("name", mcp.Description("Nombre legible"), mcp.Required()),
		mcp.WithString("description", mcp.Description("Descripcion 1-2 lineas. Sirve al matching de skill_search.")),
		mcp.WithString("skill_type", mcp.Description("prompt|code|api|mcp_tool. Default: prompt")),
		mcp.WithString("content", mcp.Description("Cuerpo de la skill (template prompt, codigo, etc).")),
		mcp.WithString("root_path", mcp.Description("Subpath del repo al que aplica la skill en un monorepo (ej. 'services/api'). Vacío/omitido = aplica a todo el proyecto. Usar para skills de stack cuando hay >1 stack en el repo.")),
	)
}

func (h *projectSkillHandlers) handleProjectSkillRegister(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if h.projects == nil || h.pool == nil {
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
	rootPath, _ := args["root_path"].(string)

	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	proj, perr := h.projects.GetBySlug(ctx, orgID, projSlug)
	if perr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", projSlug)), nil
	}

	var id uuid.UUID
	err := h.q(ctx).QueryRow(ctx,
		`INSERT INTO skills
		   (project_id, slug, name, description,
		    skill_type, content, input_schema, output_schema, root_path)
		 VALUES ($1,$2,$3,NULLIF($4,''),$5,NULLIF($6,''),'{}','{}',NULLIF($7,''))
		 RETURNING id`,
		proj.ID, slug, name, desc, skillType, content, rootPath,
	).Scan(&id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("register failed: %v", err)), nil
	}

	if _, lerr := h.q(ctx).Exec(ctx,
		`INSERT INTO project_skills (project_id, skill_id)
		 VALUES ($1, $2) ON CONFLICT (project_id, skill_id) DO NOTHING`,
		proj.ID, id,
	); lerr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("register: skill creada pero no se pudo enlazar: %v", lerr)), nil
	}
	return toolResultJSON(map[string]any{
		"id": id.String(), "scope": "project", "project_slug": projSlug,
		"slug": slug, "name": name, "skill_type": skillType,
		"root_path": rootPath,
		"linked":    true,
	})
}

func toolProjectSkillList() mcp.Tool {
	return mcp.NewTool("domain_project_skill_list",
		mcp.WithDescription("Lista skills disponibles para un proyecto: las propias del proyecto (project_id = proj) Y las globales de la org (project_id IS NULL). Devuelve un flag scope por cada item. include_globals=false para ver solo las del proyecto."),
		mcp.WithString("project_slug", mcp.Description("Proyecto a consultar"), mcp.Required()),
		mcp.WithBoolean("include_globals", mcp.Description("Si true (default), incluye las globales de la org. Si false, solo las del proyecto.")),
	)
}

func (h *projectSkillHandlers) handleProjectSkillList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if h.projects == nil || h.pool == nil {
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

	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	proj, perr := h.projects.GetBySlug(ctx, orgID, projSlug)
	if perr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", projSlug)), nil
	}

	q := `SELECT s.id, s.slug, s.name, COALESCE(s.description,''), s.skill_type,
		    CASE WHEN s.project_id IS NULL THEN 'global' ELSE 'project' END AS scope,
		    COALESCE(s.root_path,'') AS root_path
		   FROM skills s
		   WHERE s.deleted_at IS NULL
		     AND s.proposed = false
		     AND (s.project_id IS NULL OR s.project_id = $1)
		     AND NOT EXISTS (
		       SELECT 1 FROM project_skills ps
		        WHERE ps.skill_id = s.id AND ps.project_id = $1 AND ps.is_enabled = FALSE
		     )`
	if !includeGlobals {
		q += ` AND s.project_id IS NOT NULL`
	}
	q += ` ORDER BY (s.project_id IS NULL) ASC, s.slug ASC`

	rows, err := h.q(ctx).Query(ctx, q, proj.ID)
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
		RootPath    string `json:"root_path"`
	}
	out := make([]item, 0)
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.ID, &it.Slug, &it.Name, &it.Description, &it.SkillType, &it.Scope, &it.RootPath); err != nil {
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

// domain_skill_create crea una skill GLOBAL (project_id IS NULL). Es el
// equivalente desde MCP del seeder: la skill queda disponible para enlazar
// a cualquier proyecto via domain_project_skill_register/link. No la enlaza
// a ningun proyecto: una skill global no es usable hasta tener fila en
// project_skills (regla "no usable si no enlazada").
func toolSkillCreate() mcp.Tool {
	return mcp.NewTool("domain_skill_create",
		mcp.WithDescription("Crea una skill GLOBAL (project_id NULL) disponible para toda la org. No la enlaza a ningun proyecto — una skill global solo es usable cuando se enlaza con domain_project_skill_register. Usar para patrones reusables cross-proyecto. Para una skill propia de un proyecto usar domain_project_skill_register."),
		mcp.WithString("slug", mcp.Description("Slug de la skill (kebab-case)"), mcp.Required()),
		mcp.WithString("name", mcp.Description("Nombre legible"), mcp.Required()),
		mcp.WithString("description", mcp.Description("Descripcion 1-2 lineas. Sirve al matching de skill_search.")),
		mcp.WithString("skill_type", mcp.Description("prompt|code|api|mcp_tool. Default: prompt")),
		mcp.WithString("content", mcp.Description("Cuerpo de la skill (template prompt, codigo, etc).")),
	)
}

func (h *projectSkillHandlers) handleSkillCreate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if h.pool == nil {
		return mcp.NewToolResultError("pool not configured"), nil
	}
	args := req.GetArguments()
	slug, _ := args["slug"].(string)
	name, _ := args["name"].(string)
	if slug == "" || name == "" {
		return mcp.NewToolResultError("slug y name requeridos"), nil
	}
	skillType, _ := args["skill_type"].(string)
	if skillType == "" {
		skillType = "prompt"
	}
	if !validSkillType(skillType) {
		return mcp.NewToolResultError("skill_type invalido: use prompt|code|api|mcp_tool"), nil
	}
	desc, _ := args["description"].(string)
	content, _ := args["content"].(string)

	var id uuid.UUID
	err := h.q(ctx).QueryRow(ctx,
		`INSERT INTO skills
		   (project_id, slug, name, description,
		    skill_type, content, input_schema, output_schema)
		 VALUES (NULL,$1,$2,NULLIF($3,''),$4,NULLIF($5,''),'{}','{}')
		 RETURNING id`,
		slug, name, desc, skillType, content,
	).Scan(&id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("create failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"id": id.String(), "scope": "global",
		"slug": slug, "name": name, "skill_type": skillType,
		"note": "Skill global creada. No es usable hasta enlazarla a un proyecto con domain_project_skill_register.",
	})
}

// domain_skill_edit edita una skill existente (global o de proyecto).
// Identifica por id (UUID) o slug; con slug, resuelve la global primero
// (project_id IS NULL). Actualiza solo los campos provistos (COALESCE);
// los que no se pasan quedan intactos.
func toolSkillEdit() mcp.Tool {
	return mcp.NewTool("domain_skill_edit",
		mcp.WithDescription("Edita una skill existente. Identifica por 'id' (UUID, preciso) o por 'slug' (resuelve la GLOBAL — project_id NULL). Actualiza SOLO los campos provistos (name/description/content/skill_type); los omitidos quedan igual. No cambia el scope ni los enlaces."),
		mcp.WithString("id", mcp.Description("UUID de la skill. Preferido. Si se omite, se usa slug.")),
		mcp.WithString("slug", mcp.Description("Slug de la skill global a editar (project_id NULL). Ignorado si se pasa id.")),
		mcp.WithString("name", mcp.Description("Nuevo nombre. Omitir para no cambiar.")),
		mcp.WithString("description", mcp.Description("Nueva descripcion. Omitir para no cambiar.")),
		mcp.WithString("skill_type", mcp.Description("prompt|code|api|mcp_tool. Omitir para no cambiar.")),
		mcp.WithString("content", mcp.Description("Nuevo cuerpo. Omitir para no cambiar.")),
	)
}

func (h *projectSkillHandlers) handleSkillEdit(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	var target uuid.UUID
	if idStr != "" {
		parsed, perr := uuid.Parse(idStr)
		if perr != nil {
			return mcp.NewToolResultError("id invalido (UUID requerido)"), nil
		}
		err := h.q(ctx).QueryRow(ctx,
			`SELECT id FROM skills WHERE id = $1 AND deleted_at IS NULL`,
			parsed,
		).Scan(&target)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("skill id '%s' no encontrada", idStr)), nil
		}
	} else {
		err := h.q(ctx).QueryRow(ctx,
			`SELECT id FROM skills
			   WHERE slug = $1 AND project_id IS NULL AND deleted_at IS NULL`,
			slug,
		).Scan(&target)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("skill global '%s' no encontrada", slug)), nil
		}
	}

	var (
		name      *string
		desc      *string
		skillType *string
		content   *string
	)
	if v, ok := args["name"].(string); ok && v != "" {
		name = &v
	}
	if v, ok := args["description"].(string); ok && v != "" {
		desc = &v
	}
	if v, ok := args["skill_type"].(string); ok && v != "" {
		if !validSkillType(v) {
			return mcp.NewToolResultError("skill_type invalido: use prompt|code|api|mcp_tool"), nil
		}
		skillType = &v
	}
	if v, ok := args["content"].(string); ok && v != "" {
		content = &v
	}
	if name == nil && desc == nil && skillType == nil && content == nil {
		return mcp.NewToolResultError("nada para actualizar: pasa al menos un campo"), nil
	}

	var (
		outSlug, outName, outType string
		outDesc                   string
	)
	err := h.q(ctx).QueryRow(ctx,
		`UPDATE skills
		   SET name        = COALESCE($2, name),
		       description  = COALESCE($3, description),
		       skill_type   = COALESCE($4, skill_type),
		       content      = COALESCE($5, content)
		 WHERE id = $1 AND deleted_at IS NULL
		 RETURNING slug, name, COALESCE(description,''), skill_type`,
		target, name, desc, skillType, content,
	).Scan(&outSlug, &outName, &outDesc, &outType)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("edit failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"id":          target.String(),
		"slug":        outSlug,
		"name":        outName,
		"description": outDesc,
		"skill_type":  outType,
		"updated":     true,
	})
}

// domain_project_skill_unlink EXCLUYE una skill de un proyecto (modelo hibrido:
// auto + excluibles). Las skills aplican automaticamente; "unlink" registra una
// EXCLUSION (project_skills con is_enabled = FALSE) para que esa skill deje de
// aplicar a ese proyecto. NO borra la skill. Idempotente. Identifica la skill por
// skill_id (UUID) o por skill_slug (resuelve global o del proyecto).
func toolProjectSkillUnlink() mcp.Tool {
	return mcp.NewTool("domain_project_skill_unlink",
		mcp.WithDescription("Excluye una skill de un proyecto: registra una exclusion (project_skills.is_enabled=FALSE) para que esa skill deje de aplicar. Las skills globales aplican automaticamente; esto sirve para desactivar una puntual. NO borra la skill. Idempotente. Identifica la skill por 'skill_id' (UUID) o 'skill_slug' (busca primero la del proyecto, luego la global)."),
		mcp.WithString("project_slug", mcp.Description("Proyecto del que se desenlaza"), mcp.Required()),
		mcp.WithString("skill_id", mcp.Description("UUID de la skill. Preferido. Si se omite, se usa skill_slug.")),
		mcp.WithString("skill_slug", mcp.Description("Slug de la skill. Ignorado si se pasa skill_id.")),
	)
}

func (h *projectSkillHandlers) handleProjectSkillUnlink(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if h.projects == nil || h.pool == nil {
		return mcp.NewToolResultError("projects service / pool not configured"), nil
	}
	args := req.GetArguments()
	projSlug, _ := args["project_slug"].(string)
	skillID, _ := args["skill_id"].(string)
	skillSlug, _ := args["skill_slug"].(string)
	if projSlug == "" {
		return mcp.NewToolResultError("project_slug requerido"), nil
	}
	if skillID == "" && skillSlug == "" {
		return mcp.NewToolResultError("skill_id o skill_slug requerido"), nil
	}

	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	proj, perr := h.projects.GetBySlug(ctx, orgID, projSlug)
	if perr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", projSlug)), nil
	}

	var target uuid.UUID
	if skillID != "" {
		parsed, idErr := uuid.Parse(skillID)
		if idErr != nil {
			return mcp.NewToolResultError("skill_id invalido (UUID requerido)"), nil
		}
		target = parsed
	} else {
		err := h.q(ctx).QueryRow(ctx,
			`SELECT id FROM skills
			   WHERE slug = $1 AND deleted_at IS NULL
			     AND (project_id = $2 OR project_id IS NULL)
			 ORDER BY (project_id IS NULL) ASC
			 LIMIT 1`,
			skillSlug, proj.ID,
		).Scan(&target)
		if err != nil {
			return toolResultJSON(map[string]any{
				"project_slug": projSlug, "skill_slug": skillSlug,
				"unlinked": false, "reason": "skill no encontrada",
			})
		}
	}

	if _, eerr := h.q(ctx).Exec(ctx,
		`INSERT INTO project_skills (project_id, skill_id, is_enabled)
		   VALUES ($1, $2, FALSE)
		 ON CONFLICT (project_id, skill_id) DO UPDATE SET is_enabled = FALSE`,
		proj.ID, target,
	); eerr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("exclude failed: %v", eerr)), nil
	}
	return toolResultJSON(map[string]any{
		"project_slug": projSlug,
		"skill_id":     target.String(),
		"excluded":     true,
		"unlinked":     true,
	})
}

// nota: domain_skill_get y domain_skill_execute existentes resuelven por
// slug global. Si el LLM quiere ejecutar una project-skill, debe pasar
// el id explicito (futuro: extender execute para aceptar project_slug
// y resolver slug → id en el scope correcto).
// validSkillType valida skill_type contra el CHECK del schema (mig 000010):
// prompt|code|api|mcp_tool. Evita filtrar un error SQL crudo al LLM.
func validSkillType(t string) bool {
	switch t {
	case "prompt", "code", "api", "mcp_tool":
		return true
	default:
		return false
	}
}
