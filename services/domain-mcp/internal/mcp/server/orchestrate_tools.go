package mcpserver

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"nunezlagos/domain/internal/auth/apikey"
	flowsvc "nunezlagos/domain/internal/service/flow"
	orchsvc "nunezlagos/domain/internal/service/orchestrator"
	"nunezlagos/domain/internal/service/orchestrator/phases"
)

type orchestratorService interface {
	Run(ctx context.Context, in orchsvc.OrchestrateInput) (*orchsvc.OrchestrateResult, error)
	RecordPhaseResult(ctx context.Context, in orchsvc.PhaseResultInput) (*orchsvc.PhaseResultResult, error)
	ConfirmContinue(ctx context.Context, flowRunID uuid.UUID, confirmed bool) (*orchsvc.PhaseResultResult, error)
	GetFlowStatus(ctx context.Context, flowRunID uuid.UUID) (*orchsvc.FlowStatusResponse, error)
	CancelFlow(ctx context.Context, flowRunID uuid.UUID, reason string) (*orchsvc.FlowStatusResponse, error)
}

type orchestrateHandlers struct {
	orchestrator orchestratorService
	principal    *apikey.Principal
	flowToken    *flowsvc.FlowTokenService
}

func toolOrchestrate() mcp.Tool {
	return mcp.NewTool("domain_orchestrate",
		mcp.WithDescription("Inicia un flow del orquestador SDD a partir del prompt del usuario. Devuelve flow_run_id + plan con los steps (system_prompt + user_prompt + suggested_saves) que el cliente IDE debe ejecutar en orden. Reportar cada step terminada con domain_orchestrate_phase_result."),
		mcp.WithString("raw_text",
			mcp.Description("Prompt original del usuario (sin clasificacion previa de PromptRouter)."),
			mcp.Required(),
		),
		mcp.WithString("mode",
			mcp.Description("Modo del orquestador: micro | express | lite | full | solo | detect | async. Default: full. MICRO (nuevo) corre SOLO sdd-apply, SIN sdd-verify y SIN requisito de tests (el commit-gate lo exenta): rutea aqui para ediciones TRIVIALES sin logica testeable — cambiar texto de front, crear/editar un script, doc/markdown/config, 1 archivo. Express usa sdd-apply + sdd-verify para cambios de codigo ≤10 lineas single-file (SI corre tests). Lite corre un subset (sdd-explore + sdd-apply + sdd-verify) para cambios triviales de codigo salteando las fases pesadas. Regla de ruteo: si el cambio NO toca logica (texto/doc/config/script nuevo) → micro; si toca codigo con logica → express/lite/full segun tamano."),
		),
		mcp.WithString("starting_phase",
			mcp.Description("Phase slug para reanudar desde una fase especifica (p.ej. sdd-design). Si vacio, arranca en sdd-explore."),
		),
		mcp.WithArray("skip_phases",
			mcp.Description("Lista de phase slugs a omitir. El orquestador valida que el DAG resultante sea ejecutable."),
		),
		mcp.WithNumber("express_max_lines",
			mcp.Description("Override del threshold de Express (default 10). Solo aplica si mode=express."),
		),
		mcp.WithString("project_id",
			mcp.Description("UUID del proyecto de la corrida (de domain_session_bootstrap). OBLIGATORIO: scopea el flow_run y la cadena SDD/TDD al proyecto (flow_runs.project_id es NOT NULL)."),
			mcp.Required(),
		),
		mcp.WithString("exec_mode",
			mcp.Description("Modo de ejecucion: auto (corre sin pausar), manual (pausa y pide aprobacion tras CADA fase via domain_orchestrate_confirm), hybrid (pausa solo en fases clave: spec/design/apply/judge). Default: auto. Consulte al usuario al inicio que modo quiere."),
		),
		mcp.WithBoolean("hardspec",
			mcp.Description("Reiteracion humana del spec (OBLIGATORIA por defecto): al terminar sdd-spec el flujo pausa para que el desarrollador de el OK o solicite rehacer una parte especifica del spec. La confirmacion queda auditada. Use hardspec=false solo para desactivarla (default true)."),
		),
	)
}

func toolOrchestratePhaseResult() mcp.Tool {
	return mcp.NewTool("domain_orchestrate_phase_result",
		mcp.WithDescription("Reporta el resultado de una fase del orquestador. Valida el contract D5 (suggested_saves required) + el shape especifico del handler. Devuelve status del step + siguiente step pendiente (si hay) con su prompt."),
		mcp.WithString("flow_run_step_id",
			mcp.Description("UUID del step que termino (lo recibiste en el plan inicial de domain_orchestrate)."),
			mcp.Required(),
		),
		mcp.WithObject("output",
			mcp.Description("Output JSON de la fase. Shape depende del slug del step (ver docs/agents/sdd-pipeline.md). Si el shape es inválido (falta un campo requerido), el servidor NO mata el step: queda running (reintentable) y devuelve validation_error para que corrijas y reintentes. REQ-56 issue-56.4."),
			mcp.Required(),
		),
		mcp.WithArray("memory_refs_saved",
			mcp.Description("Memory refs persistidas via mem_save durante la fase. Cada item: {type, id}. Requerido para satisfacer suggested_saves con Required=true (D5, ej. code_reference). Si falta alguno, el servidor NO mata el step: queda running (reintentable) y devuelve missing_required_saves con {type, hint} para que lo persistas y reintentes. REQ-56 issue-56.5."),
		),
		mcp.WithArray("tool_calls",
			mcp.Description("Nombres de las tools domain_* que invocaste durante la fase (ej. [\"domain_verify_start\",\"domain_verify_complete\"]). Si la fase declara required_tool_calls, el servidor RECHAZA el cierre (step sigue running, reintentable) si falta alguna, devolviendo missing_tool_calls. REQ-54."),
		),
		mcp.WithNumber("duration_ms",
			mcp.Description("Duracion en milisegundos de la ejecucion de la fase en el cliente (opcional, para metricas)."),
		),
	)
}

func toolOrchestrateConfirm() mcp.Tool {
	return mcp.NewTool("domain_orchestrate_confirm",
		mcp.WithDescription("Confirma o rechaza un paso bloqueado por el confirm condicional D1 (RFC 0006). Se invoca cuando domain_orchestrate_phase_result devolvio RequiresConfirm=true. Si confirmed=true, el step queda pending y el cliente puede continuar con su prompt original; si false, el flow_run pasa a failed con razon 'user_rejected_confirm'."),
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
		mcp.WithDescription("Lee el estado de un flow_run del orquestador SDD: status del run + lista de steps con su status, outputs y previews de prompts. Util para resumir, retomar tras reconexion, debugging."),
		mcp.WithString("flow_run_id",
			mcp.Description("UUID del flow_run a consultar (devuelto por domain_orchestrate)."),
			mcp.Required(),
		),
	)
}

func toolFlowCancel() mcp.Tool {
	return mcp.NewTool("domain_flow_cancel",
		mcp.WithDescription("Lleva un flow_run a estado terminal 'cancelled' cuando el trabajo ya no aplica (feature retirada, flow huérfano, abort explícito). Cancela también los steps aún pendientes y persiste el motivo para audit trail. Solo cancela flows en estado no-terminal (running/paused/pending); rechaza los ya completed/failed/cancelled."),
		mcp.WithString("flow_run_id",
			mcp.Description("UUID del flow_run a cancelar (de domain_orchestrate o domain_flow_status)."),
			mcp.Required(),
		),
		mcp.WithString("reason",
			mcp.Description("Motivo de la cancelación (se persiste en flow_runs.error para trazabilidad)."),
		),
	)
}

func (h *orchestrateHandlers) handleOrchestrate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	if h.orchestrator == nil {
		return mcp.NewToolResultError("orchestrator service not configured"), nil
	}
	orgID, err := uuid.Parse(h.principal.OrganizationID)
	if err != nil {
		return mcp.NewToolResultError("invalid principal org_id"), nil
	}
	userID, _ := uuid.Parse(h.principal.UserID)

	rawText, err := req.RequireString("raw_text")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	modeStr := req.GetString("mode", "")
	startingPhase := req.GetString("starting_phase", "")
	expressMax := req.GetInt("express_max_lines", 0)

	pidStr := req.GetString("project_id", "")
	if pidStr == "" {
		return mcp.NewToolResultError("project_id es requerido (de domain_session_bootstrap)"), nil
	}
	projectID, perr := uuid.Parse(pidStr)
	if perr != nil {
		return mcp.NewToolResultError("invalid project_id"), nil
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

	hardspec := true
	if v, ok := args["hardspec"].(bool); ok {
		hardspec = v
	}

	in := orchsvc.OrchestrateInput{
		OrganizationID:  orgID,
		UserID:          userID,
		ProjectID:       projectID,
		ExecMode:        req.GetString("exec_mode", ""),
		Hardspec:        hardspec,
		RawText:         rawText,
		Mode:            orchsvc.Mode(modeStr),
		StartingPhase:   orchsvc.PhaseSlug(startingPhase),
		SkipPhases:      skipPhases,
		ExpressMaxLines: expressMax,
	}
	res, err := h.orchestrator.Run(ctx, in)
	if err != nil {
		return mcp.NewToolResultError("orchestrate: " + err.Error()), nil
	}
	body, _ := json.MarshalIndent(trimOrchestrateForTransport(res), "", "  ")
	return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(string(body))}}, nil
}

// trimOrchestrateForTransport reduce el payload de OrchestrateResult para no
// exceder el cap de tool-output del cliente (DOMAINSERV-108): el plan completo
// inlinea SnapshotPrompt + los prompts de TODAS las fases (63-74k chars), lo que
// hacía que el resultado volviera como error y el hook post-orchestrate no
// pudiera extraer el flow_run_id (→ token del gate nunca minteado). Mantiene los
// prompts SOLO de la primera fase; las siguientes llegan on-demand vía
// domain_orchestrate_phase_result / domain_flow_status.
func trimOrchestrateForTransport(res *orchsvc.OrchestrateResult) *orchsvc.OrchestrateResult {
	if res == nil || res.Plan == nil {
		return res
	}
	out := *res
	plan := *res.Plan
	steps := make([]orchsvc.PhaseStepSummary, len(plan.Steps))
	copy(steps, plan.Steps)
	for i := 1; i < len(steps); i++ {
		steps[i].SystemPrompt = ""
		steps[i].UserPrompt = ""
	}
	plan.Steps = steps
	out.Plan = &plan
	return &out
}

func (h *orchestrateHandlers) handleOrchestratePhaseResult(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	if h.orchestrator == nil {
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

	var toolCalls []string
	if raw, ok := args["tool_calls"].([]any); ok {
		for _, item := range raw {
			if s, ok := item.(string); ok && s != "" {
				toolCalls = append(toolCalls, s)
			}
		}
	}

	res, err := h.orchestrator.RecordPhaseResult(ctx, orchsvc.PhaseResultInput{
		FlowRunStepID:   stepID,
		Output:          output,
		MemoryRefsSaved: refs,
		ToolCallsSaved:  toolCalls,
		DurationMS:      durationMS,
	})
	if err != nil {
		return mcp.NewToolResultError("phase_result: " + err.Error()), nil
	}
	body, _ := json.MarshalIndent(res, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(string(body))}}, nil
}

func (h *orchestrateHandlers) handleOrchestrateConfirm(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	if h.orchestrator == nil {
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
	res, err := h.orchestrator.ConfirmContinue(ctx, flowRunID, confirmed)
	if err != nil {
		return mcp.NewToolResultError("confirm: " + err.Error()), nil
	}
	body, _ := json.MarshalIndent(res, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(string(body))}}, nil
}

func (h *orchestrateHandlers) handleFlowStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	if h.orchestrator == nil {
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
	status, err := h.orchestrator.GetFlowStatus(ctx, flowRunID)
	if err != nil {
		return mcp.NewToolResultError("flow_status: " + err.Error()), nil
	}
	body, _ := json.MarshalIndent(status, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(string(body))}}, nil
}

func toolFlowGrantToken() mcp.Tool {
	return mcp.NewTool("domain_flow_grant_token",
		mcp.WithDescription("Genera un token HMAC firmado que autoriza ediciones de código para un flow activo. El token incluye flow_run_id, session_id y expiry TTL. Válido mientras el flow esté en estado running/pending. El cliente guarda este token y lo presenta en domain_flow_validate_token en cada pre-edit."),
		mcp.WithString("flow_run_id",
			mcp.Description("UUID del flow_run activo (devuelto por domain_orchestrate)."),
			mcp.Required(),
		),
		mcp.WithString("session_id",
			mcp.Description("session_id de la sesión del agente (del hook payload)."),
			mcp.Required(),
		),
		mcp.WithArray("allowed_paths",
			mcp.Description("DOMAINSERV-110 batch-mode: globs de paths que este flow autoriza a editar. Si se pasa, el gate pre-edit solo permite ediciones cuyo path matchee uno de estos globs (scope por sub-tarea en multiagent paralelo). Vacío/omitido = sin restricción de path (comportamiento histórico)."),
		),
	)
}

func toolFlowValidateToken() mcp.Tool {
	return mcp.NewTool("domain_flow_validate_token",
		mcp.WithDescription("Valida un token HMAC de flow. Verifica firma, expiry y que el flow siga activo (running/pending). Devuelve {valid, flow_run_id, status, reason}."),
		mcp.WithString("token",
			mcp.Description("Token HMAC firmado (generado por domain_flow_grant_token)."),
			mcp.Required(),
		),
		mcp.WithString("session_id",
			mcp.Description("session_id de la sesión que valida (del hook payload). Debe coincidir con el del token."),
			mcp.Required(),
		),
	)
}

func (h *orchestrateHandlers) handleFlowGrantToken(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	if h.flowToken == nil || !h.flowToken.IsConfigured() {
		return mcp.NewToolResultError("flow token: HMAC secret not configured (set DOMAIN_FLOW_TOKEN_SECRET)"), nil
	}

	flowRunID := req.GetString("flow_run_id", "")
	sessionID := req.GetString("session_id", "")
	if flowRunID == "" || sessionID == "" {
		return mcp.NewToolResultError("flow_run_id and session_id are required"), nil
	}

	// validate flow is active
	if h.orchestrator != nil {
		fid, err := uuid.Parse(flowRunID)
		if err != nil {
			return mcp.NewToolResultError("invalid flow_run_id"), nil
		}
		status, err := h.orchestrator.GetFlowStatus(ctx, fid)
		if err != nil {
			return mcp.NewToolResultError("flow_grant_token: " + err.Error()), nil
		}
		if status.Status != "running" && status.Status != "pending" {
			return mcp.NewToolResultError("flow_grant_token: flow is not active (status=" + status.Status + ")"), nil
		}
	}

	var allowedPaths []string
	if raw, ok := req.GetArguments()["allowed_paths"].([]any); ok {
		for _, p := range raw {
			if s, ok := p.(string); ok && s != "" {
				allowedPaths = append(allowedPaths, s)
			}
		}
	}

	token, err := h.flowToken.GenerateToken(flowRunID, sessionID, h.principal.OrganizationID, allowedPaths...)
	if err != nil {
		return mcp.NewToolResultError("flow_grant_token: " + err.Error()), nil
	}

	body, _ := json.MarshalIndent(map[string]any{
		"token":       token,
		"flow_run_id": flowRunID,
		"session_id":  sessionID,
		"expires_in":  int(flowsvc.FlowTokenTTL.Seconds()),
	}, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(string(body))}}, nil
}

func flowInvalidResult(reason string) *mcp.CallToolResult {
	body, _ := json.MarshalIndent(map[string]any{"valid": false, "reason": reason}, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(string(body))}}
}

func (h *orchestrateHandlers) handleFlowValidateToken(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	if h.flowToken == nil || !h.flowToken.IsConfigured() {
		return mcp.NewToolResultError("flow token: HMAC secret not configured (set DOMAIN_FLOW_TOKEN_SECRET)"), nil
	}

	token := req.GetString("token", "")
	if token == "" {
		return mcp.NewToolResultError("token is required"), nil
	}

	payload, err := h.flowToken.ValidateToken(token)
	if err != nil {
		reason := "invalid"
		if err == flowsvc.ErrTokenExpired {
			reason = "expired"
		}
		return flowInvalidResult(reason), nil
	}

	// DOMAINSERV-98: el token debe pertenecer a la org del principal y a la
	// sesión que valida; corta replay cross-org y cross-sesión antes del
	// check de flow activo.
	if payload.OrgID != h.principal.OrganizationID {
		return flowInvalidResult("org_mismatch"), nil
	}
	if payload.SessionID != req.GetString("session_id", "") {
		return flowInvalidResult("session_mismatch"), nil
	}

	// validate flow is still active server-side (fail-closed: DOMAINSERV-94)
	// active solo pasa a true tras una lectura EXITOSA de status running/pending;
	// error / not-found / parse-fail / orchestrator-nil → active=false (sin pase libre).
	active := false
	flowStatus := ""
	if h.orchestrator != nil {
		fid, err := uuid.Parse(payload.FlowRunID)
		if err == nil {
			status, err := h.orchestrator.GetFlowStatus(ctx, fid)
			if err == nil {
				flowStatus = status.Status
				if status.Status == "running" || status.Status == "pending" {
					active = true
				}
			}
		}
	}

	if !active {
		body, _ := json.MarshalIndent(map[string]any{
			"valid":        false,
			"reason":       "flow_inactive",
			"flow_run_id":  payload.FlowRunID,
			"flow_status":  flowStatus,
		}, "", "  ")
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(string(body))}}, nil
	}

	body, _ := json.MarshalIndent(map[string]any{
		"valid":         true,
		"flow_run_id":   payload.FlowRunID,
		"session_id":    payload.SessionID,
		"flow_status":   flowStatus,
		"allowed_paths": payload.AllowedPaths,
	}, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(string(body))}}, nil
}

func (h *orchestrateHandlers) handleFlowCancel(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	if h.orchestrator == nil {
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
	reason := req.GetString("reason", "")
	status, err := h.orchestrator.CancelFlow(ctx, flowRunID, reason)
	if err != nil {
		return mcp.NewToolResultError("flow_cancel: " + err.Error()), nil
	}
	body, _ := json.MarshalIndent(status, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(string(body))}}, nil
}

// registerOrchestrateTools devuelve los 3 ServerTool del orquestador.
// El caller (Tools() en server.go) los appendea al slice principal.
func registerOrchestrateTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	h := &orchestrateHandlers{principal: deps.Principal}
	if deps.Orchestrator != nil {
		h.orchestrator = deps.Orchestrator
	}
	if deps.FlowToken != nil {
		h.flowToken = deps.FlowToken
	}
	wrap.SetBudget("domain_orchestrate",
		ToolBudget{CallsPerMinute: 60, MaxRetries: 1, RetryBackoff: defaultBudget.RetryBackoff})
	wrap.SetBudget("domain_orchestrate_phase_result",
		ToolBudget{CallsPerMinute: 60, MaxRetries: 1, RetryBackoff: defaultBudget.RetryBackoff})
	wrap.SetBudget("domain_orchestrate_confirm",
		ToolBudget{CallsPerMinute: 60, MaxRetries: 1, RetryBackoff: defaultBudget.RetryBackoff})
	wrap.SetBudget("domain_flow_grant_token",
		ToolBudget{CallsPerMinute: 120, MaxRetries: 1, RetryBackoff: defaultBudget.RetryBackoff})
	wrap.SetBudget("domain_flow_validate_token",
		ToolBudget{CallsPerMinute: 300, MaxRetries: 1, RetryBackoff: defaultBudget.RetryBackoff})
	return []mcpgo.ServerTool{
		{Tool: toolOrchestrate(), Handler: wrap.Wrap("domain_orchestrate", h.handleOrchestrate)},
		{Tool: toolOrchestratePhaseResult(), Handler: wrap.Wrap("domain_orchestrate_phase_result", h.handleOrchestratePhaseResult)},
		{Tool: toolOrchestrateConfirm(), Handler: wrap.Wrap("domain_orchestrate_confirm", h.handleOrchestrateConfirm)},
		{Tool: toolFlowStatus(), Handler: wrap.Wrap("domain_flow_status", h.handleFlowStatus)},
		{Tool: toolFlowCancel(), Handler: wrap.Wrap("domain_flow_cancel", h.handleFlowCancel)},
		{Tool: toolFlowGrantToken(), Handler: wrap.Wrap("domain_flow_grant_token", h.handleFlowGrantToken)},
		{Tool: toolFlowValidateToken(), Handler: wrap.Wrap("domain_flow_validate_token", h.handleFlowValidateToken)},
	}
}
