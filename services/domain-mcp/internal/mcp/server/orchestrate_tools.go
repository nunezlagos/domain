// MCP tools — orquestador SDD (issue-08.10 mcp-001 + mcp-002 + mcp-004).
//
// Tres tools que el cliente IDE (Claude Code, Cline, etc.) usa para
// driver el pipeline SDD del servidor:
//
//   domain_orchestrate(raw_text, mode?, starting_phase?, skip_phases?)
//     Inicia un flow_run + devuelve el plan con prompts pre-construidos.
//
//   domain_orchestrate_phase_result(flow_run_step_id, output, memory_refs_saved)
//     Reporta el resultado de una fase. Valida D5 + handler.Validate.
//     Devuelve status + next step prompt si hay más fases pending.
//
//   domain_flow_status(flow_run_id)
//     Lee el estado completo de un flow_run gobernado por el orquestador.

package mcpserver

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	orchsvc "nunezlagos/domain/internal/service/orchestrator"
	"nunezlagos/domain/internal/service/orchestrator/phases"
)

func toolOrchestrate() mcp.Tool {
	return mcp.NewTool("domain_orchestrate",
		mcp.WithDescription("Inicia un flow del orquestador SDD a partir del prompt del usuario. Devuelve flow_run_id + plan con los steps (system_prompt + user_prompt + suggested_saves) que el cliente IDE debe ejecutar en orden. Reportar cada step terminada con domain_orchestrate_phase_result."),
		mcp.WithString("raw_text",
			mcp.Description("Prompt original del usuario (sin clasificación previa de PromptRouter)."),
			mcp.Required(),
		),
		mcp.WithString("mode",
			mcp.Description("Modo del orquestador: express | lite | full | solo | detect | async. Default: full. Express usa sólo sdd-apply + sdd-verify para cambios ≤10 líneas single-file. Lite corre un subset (sdd-explore + sdd-apply + sdd-verify) para cambios triviales (fix de 1 línea, doc, refactor chico) salteando las fases pesadas."),
		),
		mcp.WithString("starting_phase",
			mcp.Description("Phase slug para reanudar desde una fase específica (p.ej. sdd-design). Si vacío, arranca en sdd-explore."),
		),
		mcp.WithArray("skip_phases",
			mcp.Description("Lista de phase slugs a omitir. El orquestador valida que el DAG resultante sea ejecutable."),
		),
		mcp.WithNumber("express_max_lines",
			mcp.Description("Override del threshold de Express (default 10). Sólo aplica si mode=express."),
		),
		mcp.WithString("project_id",
			mcp.Description("UUID del proyecto de la corrida (de domain_session_bootstrap). Scopea el flow_run y la cadena SDD/TDD al proyecto. Si se omite, la corrida queda sin proyecto."),
		),
		mcp.WithString("exec_mode",
			mcp.Description("Modo de ejecución: auto (corre sin pausar), manual (pausa y pide aprobación tras CADA fase vía domain_orchestrate_confirm), hybrid (pausa solo en fases clave: spec/design/apply/judge). Default: auto. Preguntale al usuario al inicio qué modo quiere."),
		),
	)
}

func toolOrchestratePhaseResult() mcp.Tool {
	return mcp.NewTool("domain_orchestrate_phase_result",
		mcp.WithDescription("Reporta el resultado de una fase del orquestador. Valida el contract D5 (suggested_saves required) + el shape específico del handler. Devuelve status del step + siguiente step pendiente (si hay) con su prompt."),
		mcp.WithString("flow_run_step_id",
			mcp.Description("UUID del step que terminó (lo recibiste en el plan inicial de domain_orchestrate)."),
			mcp.Required(),
		),
		mcp.WithObject("output",
			mcp.Description("Output JSON de la fase. Shape depende del slug del step (ver docs/agents/sdd-pipeline.md)."),
			mcp.Required(),
		),
		mcp.WithArray("memory_refs_saved",
			mcp.Description("Memory refs persistidas vía mem_save durante la fase. Cada item: {type, id}. Requerido para satisfacer suggested_saves con Required=true (D5)."),
		),
		mcp.WithNumber("duration_ms",
			mcp.Description("Duración en milisegundos de la ejecución de la fase en el cliente (opcional, para métricas)."),
		),
	)
}

func toolOrchestrateConfirm() mcp.Tool {
	return mcp.NewTool("domain_orchestrate_confirm",
		mcp.WithDescription("Confirma o rechaza un paso bloqueado por el confirm condicional D1 (RFC 0006). Se invoca cuando domain_orchestrate_phase_result devolvió RequiresConfirm=true. Si confirmed=true, el step queda pending y el cliente puede continuar con su prompt original; si false, el flow_run pasa a failed con razón 'user_rejected_confirm'."),
		mcp.WithString("flow_run_id",
			mcp.Description("UUID del flow_run que tiene un step bloqueado."),
			mcp.Required(),
		),
		mcp.WithBoolean("confirmed",
			mcp.Description("true para desbloquear y continuar; false para rechazar y marcar el flow como failed."),
			mcp.Required(),
		),
	)
}

func toolFlowStatus() mcp.Tool {
	return mcp.NewTool("domain_flow_status",
		mcp.WithDescription("Lee el estado de un flow_run del orquestador SDD: status del run + lista de steps con su status, outputs y previews de prompts. Útil para resumir, retomar tras reconexión, debugging."),
		mcp.WithString("flow_run_id",
			mcp.Description("UUID del flow_run a consultar (devuelto por domain_orchestrate)."),
			mcp.Required(),
		),
	)
}

func (d *Deps) handleOrchestrate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	if d.Orchestrator == nil {
		return mcp.NewToolResultError("orchestrator service not configured"), nil
	}
	orgID, err := uuid.Parse(d.Principal.OrganizationID)
	if err != nil {
		return mcp.NewToolResultError("invalid principal org_id"), nil
	}
	userID, _ := uuid.Parse(d.Principal.UserID)

	rawText, err := req.RequireString("raw_text")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	modeStr := req.GetString("mode", "")
	startingPhase := req.GetString("starting_phase", "")
	expressMax := req.GetInt("express_max_lines", 0)

	projectID := uuid.Nil
	if s := req.GetString("project_id", ""); s != "" {
		p, perr := uuid.Parse(s)
		if perr != nil {
			return mcp.NewToolResultError("invalid project_id"), nil
		}
		projectID = p
	}

	args := req.GetArguments()
	var skipPhases []orchsvc.PhaseSlug
	if raw, ok := args["skip_phases"].([]any); ok {
		for _, p := range raw {
			if s, ok := p.(string); ok {
				skipPhases = append(skipPhases, orchsvc.PhaseSlug(s))
			}
		}
	}

	in := orchsvc.OrchestrateInput{
		OrganizationID:  orgID,
		UserID:          userID,
		ProjectID:       projectID,
		ExecMode:        req.GetString("exec_mode", ""),
		RawText:         rawText,
		Mode:            orchsvc.Mode(modeStr),
		StartingPhase:   orchsvc.PhaseSlug(startingPhase),
		SkipPhases:      skipPhases,
		ExpressMaxLines: expressMax,
	}
	res, err := d.Orchestrator.Run(ctx, in)
	if err != nil {
		return mcp.NewToolResultError("orchestrate: " + err.Error()), nil
	}
	body, _ := json.MarshalIndent(res, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(string(body))}}, nil
}

func (d *Deps) handleOrchestratePhaseResult(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	if d.Orchestrator == nil {
		return mcp.NewToolResultError("orchestrator service not configured"), nil
	}
	stepIDStr, err := req.RequireString("flow_run_step_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	stepID, err := uuid.Parse(stepIDStr)
	if err != nil {
		return mcp.NewToolResultError("invalid flow_run_step_id"), nil
	}
	args := req.GetArguments()
	output, _ := args["output"].(map[string]any)
	durationMS := int64(req.GetInt("duration_ms", 0))

	var refs []phases.MemoryRef
	if raw, ok := args["memory_refs_saved"].([]any); ok {
		for _, item := range raw {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			ref := phases.MemoryRef{}
			if t, ok := m["type"].(string); ok {
				ref.Type = t
			}
			if id, ok := m["id"].(string); ok {
				if parsed, err := uuid.Parse(id); err == nil {
					ref.ID = parsed
				}
			}
			refs = append(refs, ref)
		}
	}

	res, err := d.Orchestrator.RecordPhaseResult(ctx, orchsvc.PhaseResultInput{
		FlowRunStepID:   stepID,
		Output:          output,
		MemoryRefsSaved: refs,
		DurationMS:      durationMS,
	})
	if err != nil {
		// Errores tipados se devuelven con código accionable para que el
		// cliente decida si re-emitir, fallar o pedir input humano.
		return mcp.NewToolResultError("phase_result: " + err.Error()), nil
	}
	body, _ := json.MarshalIndent(res, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(string(body))}}, nil
}

func (d *Deps) handleOrchestrateConfirm(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	if d.Orchestrator == nil {
		return mcp.NewToolResultError("orchestrator service not configured"), nil
	}
	idStr, err := req.RequireString("flow_run_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	flowRunID, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("invalid flow_run_id"), nil
	}
	args := req.GetArguments()
	confirmed, _ := args["confirmed"].(bool)
	res, err := d.Orchestrator.ConfirmContinue(ctx, flowRunID, confirmed)
	if err != nil {
		return mcp.NewToolResultError("confirm: " + err.Error()), nil
	}
	body, _ := json.MarshalIndent(res, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(string(body))}}, nil
}

func (d *Deps) handleFlowStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	if d.Orchestrator == nil {
		return mcp.NewToolResultError("orchestrator service not configured"), nil
	}
	idStr, err := req.RequireString("flow_run_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	flowRunID, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("invalid flow_run_id"), nil
	}
	status, err := d.Orchestrator.GetFlowStatus(ctx, flowRunID)
	if err != nil {
		return mcp.NewToolResultError("flow_status: " + err.Error()), nil
	}
	body, _ := json.MarshalIndent(status, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(string(body))}}, nil
}

// registerOrchestrateTools devuelve los 3 ServerTool del orquestador.
// El caller (Tools() en server.go) los appendea al slice principal.
func registerOrchestrateTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	// Mutators: tope conservador (60/min) como mem_save, agent_run, etc.
	wrap.SetBudget("domain_orchestrate",
		ToolBudget{CallsPerMinute: 60, MaxRetries: 1, RetryBackoff: defaultBudget.RetryBackoff})
	wrap.SetBudget("domain_orchestrate_phase_result",
		ToolBudget{CallsPerMinute: 60, MaxRetries: 1, RetryBackoff: defaultBudget.RetryBackoff})
	wrap.SetBudget("domain_orchestrate_confirm",
		ToolBudget{CallsPerMinute: 60, MaxRetries: 1, RetryBackoff: defaultBudget.RetryBackoff})
	return []mcpgo.ServerTool{
		{Tool: toolOrchestrate(), Handler: wrap.Wrap("domain_orchestrate", deps.handleOrchestrate)},
		{Tool: toolOrchestratePhaseResult(), Handler: wrap.Wrap("domain_orchestrate_phase_result", deps.handleOrchestratePhaseResult)},
		{Tool: toolOrchestrateConfirm(), Handler: wrap.Wrap("domain_orchestrate_confirm", deps.handleOrchestrateConfirm)},
		{Tool: toolFlowStatus(), Handler: wrap.Wrap("domain_flow_status", deps.handleFlowStatus)},
	}
}
