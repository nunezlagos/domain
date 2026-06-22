// issue-12.3 — tools MCP faltantes del catalogo: skill_execute,
// agent_create, flow_create y cron_list.
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"nunezlagos/domain/internal/dispatch"
	agentsvc "nunezlagos/domain/internal/service/agent"
	flowsvc "nunezlagos/domain/internal/service/flow"
	projsvc "nunezlagos/domain/internal/service/project"
)

func registerCatalogTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	// rls wrappea handlers que tocan tablas con RLS FORCE (projects desde
	// migration 000101). Abre tx + SET LOCAL app.current_org_id/user_id.
	rls := func(h mcpgo.ToolHandlerFunc) mcpgo.ToolHandlerFunc {
		return withOrgTxHandler(&deps, h)
	}
	tools := []mcpgo.ServerTool{
		{Tool: toolSkillExecute(), Handler: wrap.Wrap("domain_skill_execute", deps.runSkillDispatch)},
		{Tool: toolAgentCreate(), Handler: wrap.Wrap("domain_agent_create", deps.handleAgentCreate)},
		{Tool: toolFlowCreate(), Handler: wrap.Wrap("domain_flow_create", deps.handleFlowCreate)},
		{Tool: toolCronList(), Handler: wrap.Wrap("domain_cron_list", deps.handleCronList)},
		// REQ-28.2: gestion de projects desde MCP, con asociacion a client.
		{Tool: toolProjectCreate(), Handler: wrap.Wrap("domain_project_create", rls(deps.handleProjectCreate))},
		{Tool: toolProjectUpdate(), Handler: wrap.Wrap("domain_project_update", rls(deps.handleProjectUpdate))},
	}
	// Clients (mandantes): CRUD + restore + set_status para consultoras.
	tools = append(tools, registerClientTools(wrap, deps)...)
	return tools
}

// toolProjectCreate (REQ-28.2): crea un project, opcionalmente asociado a un
// client (mandante) via client_slug. Si client_slug se omite, el project queda
// como "interno" (client_id NULL).
func toolProjectCreate() mcp.Tool {
	return mcp.NewTool("domain_project_create",
		mcp.WithDescription("Crea un project. Si client_slug se especifica, lo asocia al mandante correspondiente (consultoras gestionando proyectos por cliente)."),
		mcp.WithString("slug",
			mcp.Description("Slug unico per-org (kebab-case)"),
			mcp.Required(),
		),
		mcp.WithString("name",
			mcp.Description("Nombre del project"),
			mcp.Required(),
		),
		mcp.WithString("description",
			mcp.Description("Descripcion opcional"),
		),
		mcp.WithString("repository_url",
			mcp.Description("URL del repositorio asociado (opcional)"),
		),
		mcp.WithString("client_slug",
			mcp.Description("Opcional: slug del client (mandante) en la misma org al que pertenece este project."),
		),
	)
}

func (d *Deps) handleProjectCreate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	if d.Projects == nil {
		return mcp.NewToolResultError("project service not configured"), nil
	}
	args := req.GetArguments()
	slug, _ := args["slug"].(string)
	name, _ := args["name"].(string)
	if slug == "" || name == "" {
		return mcp.NewToolResultError("slug y name son requeridos"), nil
	}
	orgID, err := uuid.Parse(d.Principal.OrganizationID)
	if err != nil {
		return mcp.NewToolResultError("invalid principal org_id"), nil
	}
	userID, _ := uuid.Parse(d.Principal.UserID)

	desc, _ := args["description"].(string)
	repoURL, _ := args["repository_url"].(string)
	clientSlug, _ := args["client_slug"].(string)

	p, err := d.Projects.Create(ctx, projsvc.CreateInput{
		OrganizationID: orgID,
		Name:           name,
		Slug:           slug,
		Description:    desc,
		RepositoryURL:  repoURL,
		ClientSlug:     clientSlug,
		ActorID:        userID,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("create project failed: %v", err)), nil
	}
	out := map[string]any{
		"id": p.ID, "slug": p.Slug, "name": p.Name,
	}
	if p.ClientSlug != "" {
		out["client_slug"] = p.ClientSlug
		out["client_name"] = p.ClientName
	}
	return toolResultJSON(out)
}

// toolProjectUpdate (REQ-28.2): patch + opcionalmente cambiar el client.
// client_slug == "" → unset (proyecto pasa a interno); slug → reasigna.
func toolProjectUpdate() mcp.Tool {
	return mcp.NewTool("domain_project_update",
		mcp.WithDescription("Actualiza name/description/repository_url y opcionalmente reasigna el client (mandante). Pasar client_slug='' (string vacio) para desasignar."),
		mcp.WithString("slug",
			mcp.Description("Slug actual del project a actualizar"),
			mcp.Required(),
		),
		mcp.WithString("name",
			mcp.Description("Nuevo nombre (opcional)"),
		),
		mcp.WithString("description",
			mcp.Description("Nueva descripcion (opcional)"),
		),
		mcp.WithString("repository_url",
			mcp.Description("Nuevo repository_url (opcional)"),
		),
		mcp.WithString("client_slug",
			mcp.Description("Opcional: slug del nuevo client. String vacio '' = desasignar."),
		),
	)
}

func (d *Deps) handleProjectUpdate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	if d.Projects == nil {
		return mcp.NewToolResultError("project service not configured"), nil
	}
	args := req.GetArguments()
	slug, _ := args["slug"].(string)
	if slug == "" {
		return mcp.NewToolResultError("slug es requerido"), nil
	}
	orgID, err := uuid.Parse(d.Principal.OrganizationID)
	if err != nil {
		return mcp.NewToolResultError("invalid principal org_id"), nil
	}
	userID, _ := uuid.Parse(d.Principal.UserID)

	prev, err := d.Projects.GetBySlug(ctx, orgID, slug)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", slug)), nil
	}

	upd := projsvc.UpdateInput{ActorID: userID}
	if v, ok := args["name"].(string); ok {
		upd.Name = &v
	}
	if v, ok := args["description"].(string); ok {
		upd.Description = &v
	}
	if v, ok := args["repository_url"].(string); ok {
		upd.RepositoryURL = &v
	}
	// client_slug: solo lo agregamos si fue provisto (key presente). El
	// MCP framework devuelve "" si la key no vino → distinguimos chequeando
	// la presencia en el map.
	if raw, ok := args["client_slug"]; ok {
		if s, ok := raw.(string); ok {
			upd.ClientSlug = &s
		}
	}

	p, err := d.Projects.Update(ctx, prev.ID, upd)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("update project failed: %v", err)), nil
	}
	out := map[string]any{
		"id": p.ID, "slug": p.Slug, "name": p.Name,
	}
	if p.ClientSlug != "" {
		out["client_slug"] = p.ClientSlug
		out["client_name"] = p.ClientName
	}
	return toolResultJSON(out)
}

func toolSkillExecute() mcp.Tool {
	return mcp.NewTool("domain_skill_execute",
		mcp.WithDescription("Ejecuta un skill por slug con parametros validados contra su input_schema. Persiste el log de ejecucion."),
		mcp.WithString("skill_slug",
			mcp.Description("Slug del skill a ejecutar"),
			mcp.Required(),
		),
		mcp.WithObject("parameters",
			mcp.Description("Parametros del skill (validados contra input_schema)"),
		),
		mcp.WithString("mode",
			mcp.Description("sync (default) | async (retorna execution_id para polling)"),
		),
	)
}

// runSkillDispatch (issue-35.1 phase 5): la ejecucion de skills desde
// MCP se delega EXCLUSIVAMENTE al dispatcher unificado. El path legacy
// (SkillExecution.Execute directo) fue eliminado: 1 sola
// implementacion para cron, webhook y MCP.
func (d *Deps) runSkillDispatch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	if d.Dispatcher == nil {
		return mcp.NewToolResultError("dispatcher no configurado"), nil
	}
	if d.Skills == nil {
		return mcp.NewToolResultError("skill service no configurado"), nil
	}
	args := req.GetArguments()
	slug, _ := args["skill_slug"].(string)
	if slug == "" {
		return mcp.NewToolResultError("skill_slug es requerido"), nil
	}
	orgID, err := uuid.Parse(d.Principal.OrganizationID)
	if err != nil {
		return mcp.NewToolResultError("invalid principal org_id"), nil
	}
	sk, err := d.Skills.GetBySlug(ctx, orgID, slug)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("skill '%s' not found", slug)), nil
	}
	params, _ := args["parameters"].(map[string]any)
	mode, _ := args["mode"].(string)

	inputsRaw, _ := json.Marshal(params)
	res, dispatchErr := d.Dispatcher.Dispatch(ctx, dispatch.Request{
		OrgID: orgID, Source: dispatch.SourceMCP, TargetType: dispatch.TargetSkill,
		TargetID: sk.ID, Inputs: inputsRaw,
	})
	out := map[string]any{
		"execution_id": res.RunID.String(),
		"status":       res.Status,
		"mode":         mode,
	}
	if len(res.Output) > 0 {
		out["output"] = string(res.Output)
	}
	if dispatchErr != nil {
		out["error"] = dispatchErr.Error()
	}
	return toolResultJSON(out)
}

func toolAgentCreate() mcp.Tool {
	return mcp.NewTool("domain_agent_create",
		mcp.WithDescription("Crea un agent con provider/model/system_prompt y skills asignados."),
		mcp.WithString("slug",
			mcp.Description("Slug unico del agent (kebab-case)"),
			mcp.Required(),
		),
		mcp.WithString("name",
			mcp.Description("Nombre del agent"),
			mcp.Required(),
		),
		mcp.WithString("provider",
			mcp.Description("Provider LLM: anthropic | openai | google | ollama"),
			mcp.Required(),
		),
		mcp.WithString("model",
			mcp.Description("Modelo (ej: claude-sonnet-4-6)"),
			mcp.Required(),
		),
		mcp.WithString("system_prompt",
			mcp.Description("System prompt del agent"),
		),
		mcp.WithArray("skills",
			mcp.Description("Slugs de skills a asignar"),
			mcp.Items(map[string]any{"type": "string"}),
		),
	)
}

func (d *Deps) handleAgentCreate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	args := req.GetArguments()
	slug, _ := args["slug"].(string)
	name, _ := args["name"].(string)
	provider, _ := args["provider"].(string)
	model, _ := args["model"].(string)
	if slug == "" || name == "" || provider == "" || model == "" {
		return mcp.NewToolResultError("slug, name, provider y model son requeridos"), nil
	}
	orgID, err := uuid.Parse(d.Principal.OrganizationID)
	if err != nil {
		return mcp.NewToolResultError("invalid principal org_id"), nil
	}
	userID, _ := uuid.Parse(d.Principal.UserID)

	var skills []string
	if v, ok := args["skills"].([]any); ok {
		for _, s := range v {
			if str, ok := s.(string); ok {
				skills = append(skills, str)
			}
		}
	}
	sysPrompt, _ := args["system_prompt"].(string)

	ag, err := d.Agents.Create(ctx, agentsvc.CreateInput{
		OrganizationID: orgID, Slug: slug, Name: name,
		Provider: provider, Model: model, SystemPrompt: sysPrompt,
		SkillsSlugs: skills, ActorID: userID,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("create agent failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"agent_id": ag.ID, "slug": ag.Slug, "provider": ag.Provider, "model": ag.Model,
	})
}

func toolFlowCreate() mcp.Tool {
	return mcp.NewTool("domain_flow_create",
		mcp.WithDescription("Crea un flow con su spec DAG (steps validados: tipos, ciclos, error policies)."),
		mcp.WithString("slug",
			mcp.Description("Slug unico del flow"),
			mcp.Required(),
		),
		mcp.WithString("name",
			mcp.Description("Nombre del flow"),
			mcp.Required(),
		),
		mcp.WithObject("spec",
			mcp.Description(`Spec del flow: {"version":1,"steps":[{"id":"s1","type":"skill_run","config":{...}}]}`),
			mcp.Required(),
		),
	)
}

func (d *Deps) handleFlowCreate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	args := req.GetArguments()
	slug, _ := args["slug"].(string)
	name, _ := args["name"].(string)
	specRaw, ok := args["spec"].(map[string]any)
	if slug == "" || name == "" || !ok {
		return mcp.NewToolResultError("slug, name y spec son requeridos"), nil
	}
	orgID, err := uuid.Parse(d.Principal.OrganizationID)
	if err != nil {
		return mcp.NewToolResultError("invalid principal org_id"), nil
	}
	userID, _ := uuid.Parse(d.Principal.UserID)

	specJSON, _ := json.Marshal(specRaw)
	var spec flowsvc.Spec
	if err := json.Unmarshal(specJSON, &spec); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("spec invalido: %v", err)), nil
	}

	fl, err := d.Flows.Create(ctx, flowsvc.CreateInput{
		OrganizationID: orgID, Slug: slug, Name: name, Spec: spec, ActorID: userID,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("create flow failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"flow_id": fl.ID, "slug": fl.Slug, "steps": len(fl.Spec.Steps),
	})
}

func toolCronList() mcp.Tool {
	return mcp.NewTool("domain_cron_list",
		mcp.WithDescription("Lista los crons programados de la org con su proxima ejecucion."),
	)
}

func (d *Deps) handleCronList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	if d.Crons == nil {
		return mcp.NewToolResultError("cron service not configured"), nil
	}
	orgID, err := uuid.Parse(d.Principal.OrganizationID)
	if err != nil {
		return mcp.NewToolResultError("invalid principal org_id"), nil
	}
	crons, err := d.Crons.List(ctx, orgID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list crons failed: %v", err)), nil
	}
	out := make([]map[string]any, 0, len(crons))
	for _, c := range crons {
		out = append(out, map[string]any{
			"id": c.ID, "slug": c.Slug, "name": c.Name,
			"cron_expression": c.CronExpression, "timezone": c.Timezone,
			"target_type": c.TargetType, "enabled": c.Enabled,
			"next_run_at": c.NextRunAt, "last_run_at": c.LastRunAt,
		})
	}
	return toolResultJSON(map[string]any{"crons": out, "total": len(out)})
}
