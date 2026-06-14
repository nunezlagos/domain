// issue-12.3 — tools MCP faltantes del catálogo: skill_execute,
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
	skillsvc "nunezlagos/domain/internal/service/skill"
)

func registerCatalogTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	return []mcpgo.ServerTool{
		{Tool: toolSkillExecute(), Handler: wrap.Wrap("domain_skill_execute", deps.handleSkillExecute)},
		{Tool: toolAgentCreate(), Handler: wrap.Wrap("domain_agent_create", deps.handleAgentCreate)},
		{Tool: toolFlowCreate(), Handler: wrap.Wrap("domain_flow_create", deps.handleFlowCreate)},
		{Tool: toolCronList(), Handler: wrap.Wrap("domain_cron_list", deps.handleCronList)},
	}
}

func toolSkillExecute() mcp.Tool {
	return mcp.NewTool("domain_skill_execute",
		mcp.WithDescription("Ejecuta un skill por slug con parámetros validados contra su input_schema. Persiste el log de ejecución."),
		mcp.WithString("skill_slug",
			mcp.Description("Slug del skill a ejecutar"),
			mcp.Required(),
		),
		mcp.WithObject("parameters",
			mcp.Description("Parámetros del skill (validados contra input_schema)"),
		),
		mcp.WithString("mode",
			mcp.Description("sync (default) | async (retorna execution_id para polling)"),
		),
	)
}

func (d *Deps) handleSkillExecute(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	if d.SkillExecution == nil {
		return mcp.NewToolResultError("skill execution not configured"), nil
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

	// issue-35.1: si Dispatcher está seteado, delegamos.
	if d.Dispatcher != nil {
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

	exec, err := d.SkillExecution.Execute(ctx, skillsvc.ExecuteInput{
		OrganizationID: orgID, SkillID: sk.ID, Parameters: params, Mode: mode,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("execute failed: %v", err)), nil
	}
	out := map[string]any{
		"execution_id": exec.ID,
		"status":       exec.Status,
		"mode":         exec.Mode,
	}
	if exec.Output != nil {
		out["output"] = *exec.Output
	}
	if exec.Error != nil {
		out["error"] = *exec.Error
	}
	return toolResultJSON(out)
}

func toolAgentCreate() mcp.Tool {
	return mcp.NewTool("domain_agent_create",
		mcp.WithDescription("Crea un agent con provider/model/system_prompt y skills asignados."),
		mcp.WithString("slug",
			mcp.Description("Slug único del agent (kebab-case)"),
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
			mcp.Description("Slug único del flow"),
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
		return mcp.NewToolResultError(fmt.Sprintf("spec inválido: %v", err)), nil
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
		mcp.WithDescription("Lista los crons programados de la org con su próxima ejecución."),
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
