package mcpserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/observability"
	flowsvc "nunezlagos/domain/internal/service/flow"
	orchsvc "nunezlagos/domain/internal/service/orchestrator"
	"nunezlagos/domain/internal/service/orchestrator/phases"
	"nunezlagos/domain/internal/store/txctx"
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
			mcp.Description("UUID del proyecto de la corrida (de domain_session_bootstrap). OBLIGATORIO: scopea el flow_run y la cadena SDD/TDD al proyecto. La columna flow_runs.project_id es nullable por historia (migracion 000161), pero un flow_run sin proyecto NO puede obtener token de edicion: el RLS de flow_agent_scopes no tiene eje y el grant se deniega."),
			mcp.Required(),
		),
		// DOMAINSERV-256: declarado acá porque el hook post-orchestrate lo lee del tool_input para
		// pedir el token con scope. Sin declararlo el parámetro se perdía de dos maneras y las dos
		// eran mudas: el cliente lo descartaba, o lo propagaba como string y el isinstance(list)
		// del hook lo tiraba. En ambos casos el token salía sin allowed_paths y el gate degradaba
		// a "sin restricción" — un fail-OPEN que la verificación de 2 agentes de t15 midió.
		mcp.WithArray("allowed_paths",
			mcp.Description("DOMAINSERV-218 batch-mode: globs de paths que ESTE agente autoriza a editar en el flow (ej. [\"services/domain-mcp/**\"]). El hook post-orchestrate lo reenvia a domain_flow_grant_token, que firma el scope dentro del token: a partir de ahi el gate pre-edit deniega toda edicion fuera de esos globs y el deny nombra el scope propio. Es lo que permite que N subagentes paralelos editen sin pisarse. Cada glob necesita prefijo literal ('services/**' si, '**/*.go' no: ese no acota nada). Dos agentes del mismo flow no pueden reclamar territorio solapado — el segundo grant se rechaza al EMITIR. Omitido = sin restriccion de path (comportamiento historico de los flows que no declaran particion)."),
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
	return toolResultJSON(trimOrchestrateForTransport(res))
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
	// DOMAINSERV-142: snapshot_prompt y steps[0].user_prompt son el MISMO string
	// (service.go copia el segundo en el primero), ~5.7k de duplicación pura en un
	// payload que ya excedía el límite del tool result. Se manda una sola vez.
	//
	// El que se suelta es snapshot_prompt, y la elección es por evidencia del contrato
	// (mcp-response-shape-contract): la descripción de la tool le promete al cliente
	// "plan con los steps (system_prompt + user_prompt + suggested_saves) que el
	// cliente IDE debe ejecutar en orden" — steps[].user_prompt ES el contrato
	// documentado; snapshot_prompt no se menciona en ninguna parte. Soltar el step 0
	// habría dejado la primera fase sin prompt por el campo que el cliente sí tiene
	// documentado.
	//
	// El recorte es solo de TRANSPORTE: el OrchestrateResult del service conserva los
	// dos campos, así que el worker async y promptrouter no se enteran. Consumidores
	// verificados, no asumidos: hooks del installer (session-start, user-prompt, stop,
	// post-orchestrate, pre-edit), prompts seeded (first-response, agent-protocol) y
	// dashboard admin — cero usos de snapshot_prompt fuera del propio service.
	if len(steps) > 0 && out.SnapshotPrompt == steps[0].UserPrompt {
		out.SnapshotPrompt = ""
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
	return toolResultJSON(res)
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
	return toolResultJSON(res)
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
	return toolResultJSON(status)
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
		mcp.WithString("agent_id",
			mcp.Description("DOMAINSERV-218: agente al que se ata el token (del hook payload; presente solo dentro de un subagente). Omitido = token de sesión, el comportamiento histórico del hilo principal. Con él, el token solo lo puede usar ESE agente, que es lo que permite darle a cada sub-tarea su propio allowed_paths."),
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
		mcp.WithString("agent_id",
			mcp.Description("DOMAINSERV-218: agente que valida (del hook payload; ausente en el hilo principal). Debe coincidir EXACTAMENTE con el del token: un token de otro agente, o un token sin agente presentado por uno, devuelven agent_mismatch."),
		),
	)
}

// flowActivoParaGrant devuelve el mensaje de error si el flow no puede recibir un token, o ""
// si está activo. Se extrajo de handleFlowGrantToken en DOMAINSERV-234: el guard de allowlists
// la había llevado a 56 líneas y size-lint —que es job OBLIGATORIO del CI y NO lo corre
// `go test`— quedó en rojo sin que nadie lo notara.
//
// Si el orquestador no está configurado devuelve "" a propósito: el fail-closed de este camino
// lo hace la firma HMAC del token, no esta validación, y negar por falta de orquestador dejaría
// el grant insatisfacible en un server sin él.
func (h *orchestrateHandlers) flowActivoParaGrant(ctx context.Context, flowRunID string) string {
	if h.orchestrator == nil {
		return ""
	}
	fid, err := uuid.Parse(flowRunID)
	if err != nil {
		return "invalid flow_run_id"
	}
	status, err := h.orchestrator.GetFlowStatus(ctx, fid)
	if err != nil {
		return "flow_grant_token: " + err.Error()
	}
	if status.Status != "running" && status.Status != "pending" {
		return "flow_grant_token: flow is not active (status=" + status.Status + ")"
	}
	return ""
}

// scopeReservadoParaElGrant resuelve la allowlist del request, la valida y reserva el
// territorio del agente. Devuelve el mensaje de error listo para el caller, o "" si todo
// salió bien.
//
// Está extraída del handler y no inline por size-lint: handleFlowGrantToken ya había cruzado
// las 50 líneas al sumarle el chequeo de tipo de DOMAINSERV-256, y el diseño de DOMAINSERV-218
// ya lo anticipaba — "extraer, no engordar". Los tres pasos van juntos porque son un solo
// invariante: el territorio se reserva ANTES de firmar, así que un scope inválido o solapado
// no puede dejar un token emitido que el gate después rechace.
func scopeReservadoParaElGrant(ctx context.Context, req mcp.CallToolRequest, flowRunID, agentID string) ([]string, string) {
	allowedPaths, err := allowedPathsDelRequest(req)
	if err != nil {
		return nil, "flow_grant_token: " + err.Error()
	}

	// El glob se valida al EMITIR y no al editar (DOMAINSERV-218). Un "**/*.go" tiene scope
	// vacío: como allowlist de batch-mode no acota nada, y hace que cualquier par de
	// sub-tareas se solape. Aceptarlo devolvería un token que parece scopeado y no lo está.
	if err := flowsvc.ValidarAllowlist(allowedPaths); err != nil {
		return nil, "flow_grant_token: " + err.Error()
	}

	if fid, perr := uuid.Parse(flowRunID); perr == nil {
		if err := reservarTerritorio(ctx, fid, agentID, allowedPaths); err != nil {
			return nil, "flow_grant_token: " + err.Error()
		}
	}
	return allowedPaths, ""
}

// allowedPathsDelRequest extrae los globs. La validación de su gramática es aparte, en
// flowsvc.ValidarAllowlist: acá solo se normaliza.
//
// DOMAINSERV-256: no puede colapsar "no declaró scope" con "declaró scope y llegó roto". El
// primero es el flow normal y significa "sin restricción de path"; el segundo, tratado igual,
// firma un token SIN restricción para un agente que creyó estar confinado, y no avisa a nadie.
// El fail-open era invisible justamente porque el server no devuelve la allowlist en el eco:
// se detectó decodificando el claim `p` del marker, no leyendo la respuesta del tool.
func allowedPathsDelRequest(req mcp.CallToolRequest) ([]string, error) {
	v, presente := req.GetArguments()["allowed_paths"]
	if !presente || v == nil {
		return nil, nil
	}
	raw, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("allowed_paths llegó como %T y el contrato es un array de strings: "+
			"tratarlo como 'sin scope' firmaría un token sin restricción de path en silencio", v)
	}
	out := make([]string, 0, len(raw))
	for i, p := range raw {
		s, ok := p.(string)
		if !ok || strings.TrimSpace(s) == "" {
			return nil, fmt.Errorf("allowed_paths[%d] no es un glob no vacío (%T): descartarlo "+
				"achicaría el territorio declarado sin avisar", i, p)
		}
		out = append(out, s)
	}
	return out, nil
}

// scopeDelFlowRun deriva el proyecto del flow_run y deja el GUC seteado. Hace falta un camino
// propio y no alcanza withProjectTxHandler: ese resuelve el scope desde el project_slug DEL
// REQUEST, y grant_token recibe flow_run_id, no slug.
func scopeDelFlowRun(ctx context.Context, tx pgx.Tx, flowRunID uuid.UUID) error {
	var projectID *uuid.UUID
	err := tx.QueryRow(ctx, `SELECT project_id FROM flow_runs WHERE id = $1`, flowRunID).Scan(&projectID)
	if err != nil {
		return fmt.Errorf("flow_run %s: %w", flowRunID, err)
	}
	if projectID == nil {
		return fmt.Errorf("flow_run %s no tiene proyecto: sin eje de scope el RLS devolvería 0 filas sin error", flowRunID)
	}
	return setProjectScope(ctx, *projectID)
}

// reservarTerritorio es el caller que hace real el criterio 3 de DOMAINSERV-218: rechaza el
// solapamiento contra los scopes vigentes ANTES de emitir, y registra el propio.
//
// SIN TX NO PERSISTE Y NO FALLA, a propósito. La firma HMAC y el check de flow activo son el
// fail-closed de este camino; negar el grant porque no hay pool dejaría el gate insatisfacible
// en cualquier server sin base, y un gate que deniega lo legítimo empuja al bypass permanente
// (DOMAINSERV-111/175/195). Lo que se pierde sin tx es el aislamiento entre agentes, no la
// autenticidad del token.
func reservarTerritorio(ctx context.Context, flowRunID uuid.UUID, agentID string, allowedPaths []string) error {
	tx := txctx.TxFromContext(ctx)
	if tx == nil {
		return nil
	}
	if err := scopeDelFlowRun(ctx, tx, flowRunID); err != nil {
		return err
	}
	vigentes, err := flowsvc.ScopesVigentesDelFlow(ctx, tx, flowRunID)
	if err != nil {
		return err
	}
	if err := flowsvc.SolapamientoConOtros(agentID, allowedPaths, vigentes); err != nil {
		return err
	}
	return flowsvc.RegistrarScope(ctx, tx, flowRunID, agentID, allowedPaths, time.Now().UTC().Add(flowsvc.FlowTokenTTL))
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

	if msg := h.flowActivoParaGrant(ctx, flowRunID); msg != "" {
		return mcp.NewToolResultError(msg), nil
	}

	agentID := req.GetString("agent_id", "")
	allowedPaths, errMsg := scopeReservadoParaElGrant(ctx, req, flowRunID, agentID)
	if errMsg != "" {
		return mcp.NewToolResultError(errMsg), nil
	}

	token, err := h.flowToken.GenerateTokenParaAgente(flowRunID, sessionID, h.principal.OrganizationID, agentID, allowedPaths)
	if err != nil {
		return mcp.NewToolResultError("flow_grant_token: " + err.Error()), nil
	}

	return toolResultJSON(map[string]any{
		"token":       token,
		"flow_run_id": flowRunID,
		"session_id":  sessionID,
		"agent_id":    agentID,
		"expires_in":  int(flowsvc.FlowTokenTTL.Seconds()),
	})
}

// devuelve solo el result porque los callers lo usan como valor, no como par (res, err)
func flowInvalidResult(reason string) *mcp.CallToolResult {
	res, _ := toolResultJSON(map[string]any{"valid": false, "reason": reason})
	return res
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

	// DOMAINSERV-218: el check de sesión de arriba NO discrimina entre dos subagentes,
	// porque el session_id se hereda del padre. Sin este, A presenta el token de B y edita
	// su territorio.
	//
	// La comparación es SIMÉTRICA a propósito: si un token sin agente lo pudiera usar
	// cualquiera, un subagente se saldría de su scope pidiendo un grant sin agent_id.
	//
	// Esto NO deja al subagente sin token propio afuera: ese caso no llega acá, cae al
	// fallback del marker de sesión del padre (pre-edit.sh:114-121). El deny aplica cuando
	// HAY un token de otro agente, no cuando no hay ninguno.
	if payload.AgentID != req.GetString("agent_id", "") {
		return flowInvalidResult("agent_mismatch"), nil
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
		return toolResultJSON(map[string]any{
			"valid":       false,
			"reason":      "flow_inactive",
			"flow_run_id": payload.FlowRunID,
			"flow_status": flowStatus,
		})
	}

	res := map[string]any{
		"valid":         true,
		"flow_run_id":   payload.FlowRunID,
		"session_id":    payload.SessionID,
		"flow_status":   flowStatus,
		"allowed_paths": payload.AllowedPaths,
	}
	// DOMAINSERV-218: renovación deslizante. Con esto el TTL mide inactividad y no duración de
	// tarea, así que una fase larga no pierde la autorización a mitad de camino. Solo bajo el
	// umbral: el pre-edit es camino caliente y no puede escribir en cada edición.
	if renovado := h.renovarSiLeQuedaPoco(ctx, payload); renovado != "" {
		res["token"] = renovado
	}
	// DOMAINSERV-218 incremento 5: un token SIN allowed_paths deja pasar cualquier path, y un
	// subagente sin marker propio usa justamente ese —el del padre— por el fallback. Se le avisa
	// al hook que hay una partición declarada por otros para que no herede la vía libre.
	res["scopes_de_otros"] = hayParticionDeclarada(ctx, payload)
	return toolResultJSON(res)
}

// hayParticionDeclarada dice si otro agente del flow tiene territorio reservado. Sin tx devuelve
// false: no se puede afirmar que hay partición si no se pudo mirar, y afirmarlo de más frenaría
// ediciones legítimas en un server sin base.
func hayParticionDeclarada(ctx context.Context, payload *flowsvc.FlowTokenPayload) bool {
	tx := txctx.TxFromContext(ctx)
	if tx == nil {
		return false
	}
	fid, err := uuid.Parse(payload.FlowRunID)
	if err != nil || scopeDelFlowRun(ctx, tx, fid) != nil {
		return false
	}
	vigentes, err := flowsvc.ScopesVigentesDelFlow(ctx, tx, fid)
	if err != nil {
		return false
	}
	return flowsvc.HayScopesDeOtros(payload.AgentID, vigentes)
}

// renovarSiLeQuedaPoco corre la expiración hacia adelante y devuelve un token nuevo, o "" si no
// hacía falta. Un fallo renovando NO invalida la validación que ya pasó: el peor caso es que el
// token venza como antes de este cambio, y eso degrada al comportamiento de hoy, no a algo peor.
func (h *orchestrateHandlers) renovarSiLeQuedaPoco(ctx context.Context, payload *flowsvc.FlowTokenPayload) string {
	if !flowsvc.NecesitaRenovacion(payload.ExpiresAt, time.Now().UTC()) {
		return ""
	}
	nuevo, err := h.flowToken.GenerateTokenParaAgente(
		payload.FlowRunID, payload.SessionID, payload.OrgID, payload.AgentID, payload.AllowedPaths)
	if err != nil {
		return ""
	}
	if tx := txctx.TxFromContext(ctx); tx != nil {
		if fid, perr := uuid.Parse(payload.FlowRunID); perr == nil && scopeDelFlowRun(ctx, tx, fid) == nil {
			_ = flowsvc.RegistrarScope(ctx, tx, fid, payload.AgentID, payload.AllowedPaths,
				time.Now().UTC().Add(flowsvc.FlowTokenTTL))
		}
	}
	return nuevo
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

	// DOMAINSERV-218: el territorio se suelta acá y no se espera al TTL. Sin esto, un re-grant
	// después de cancelar chocaría por solapamiento contra los scopes de un flow que ya no corre.
	// Un fallo liberando NO invalida el cancel, que ya ocurrió: el TTL los vence igual.
	if tx := txctx.TxFromContext(ctx); tx != nil {
		if err := scopeDelFlowRun(ctx, tx, flowRunID); err == nil {
			_ = flowsvc.LiberarScopesDelFlow(ctx, tx, flowRunID)
		}
	}
	return toolResultJSON(status)
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
		{Tool: toolOrchestrateConfirm(), Handler: conWorkflowDeLaCorrida(wrap.Wrap("domain_orchestrate_confirm", h.handleOrchestrateConfirm))},
		{Tool: toolFlowStatus(), Handler: conWorkflowDeLaCorrida(wrap.Wrap("domain_flow_status", h.handleFlowStatus))},
		{Tool: toolFlowCancel(), Handler: conWorkflowDeLaCorrida(withOrgTxHandler(&deps, wrap.Wrap("domain_flow_cancel", h.handleFlowCancel)))},
		// DOMAINSERV-218: el grant necesita tx porque flow_agent_scopes tiene FORCE RLS y el GUC
		// de proyecto solo vive dentro de una. Sin pool sigue emitiendo sin persistir, ver
		// reservarTerritorio.
		{Tool: toolFlowGrantToken(), Handler: conWorkflowDeLaCorrida(withOrgTxHandler(&deps, wrap.Wrap("domain_flow_grant_token", h.handleFlowGrantToken)))},
		// DOMAINSERV-218: validate necesita tx para la renovación deslizante y para saber si hay
		// partición declarada. Es camino caliente, pero la escritura está acotada al umbral de
		// renovación; sin tx el handler sigue funcionando y solo pierde esas dos cosas.
		{Tool: toolFlowValidateToken(), Handler: withOrgTxHandler(&deps, wrap.Wrap("domain_flow_validate_token", h.handleFlowValidateToken))},
	}
}

// conWorkflowDeLaCorrida es el PRODUCTOR de workflow_id: mete en el ctx el
// flow_run_id que el tool declara en sus args, para que todas las tool calls de
// la misma corrida se acumulen en una unica fila de `workflows`.
//
// Envuelve POR FUERA del ResilientWrapper a proposito: el hook de metricas que
// dispara el wrapper (y con el, observability.LogToolInvocation) lee el ctx que
// Wrap recibio, no el que ve el handler adentro.
func conWorkflowDeLaCorrida(h mcpgo.ToolHandlerFunc) mcpgo.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return h(observability.WithWorkflowFromFlowRun(ctx, req.GetString("flow_run_id", "")), req)
	}
}
