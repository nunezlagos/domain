// Package mcpserver — issue-12.1 MCP server stdio.
//
// Define los tools nombrados con prefix `domain_*` que cualquier cliente MCP
// (Claude Code, otros agentes IA) puede invocar para persistir y buscar
// observations. Cada tool valida argumentos, llama al service correspondiente
// y formatea respuesta como mcp.CallToolResult.
//
// Principal:
//
//	El proceso domain-mcp resuelve UN principal al boot via API key
//	(env var DOMAIN_API_KEY) y todas las tool calls de la sesion operan en
//	nombre de ese principal. Esto coincide con el modelo MCP stdio: un
//	proceso por sesion de cliente.
package mcpserver

import (
	"context"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/agentprotocol"
	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/dispatch"
	"nunezlagos/domain/internal/observability"
	agentrunner "nunezlagos/domain/internal/runner/agent"
	flowrunner "nunezlagos/domain/internal/runner/flow"
	agentsvc "nunezlagos/domain/internal/service/agent"
	capturedpromptsvc "nunezlagos/domain/internal/service/capturedprompt"
	clientsvc "nunezlagos/domain/internal/service/client"
	codegraphsvc "nunezlagos/domain/internal/service/codegraph"
	cronsvc "nunezlagos/domain/internal/service/cron"
	syncsvc "nunezlagos/domain/internal/service/extsync"
	flowsvc "nunezlagos/domain/internal/service/flow"
	intakesvc "nunezlagos/domain/internal/service/intake"
	issuesvc "nunezlagos/domain/internal/service/issue"
	husvc "nunezlagos/domain/internal/service/issuebuilder"
	knowsvc "nunezlagos/domain/internal/service/knowledge"
	obssvc "nunezlagos/domain/internal/service/observation"
	orchsvc "nunezlagos/domain/internal/service/orchestrator"
	policysvc "nunezlagos/domain/internal/service/policy"
	projsvc "nunezlagos/domain/internal/service/project"
	projectpolicysvc "nunezlagos/domain/internal/service/projectpolicy"
	projectreposvc "nunezlagos/domain/internal/service/projectrepo"
	promptsvc "nunezlagos/domain/internal/service/prompt"
	prouter "nunezlagos/domain/internal/service/promptrouter"
	searchsvc "nunezlagos/domain/internal/service/search"
	skillsvc "nunezlagos/domain/internal/service/skill"
	specsvc "nunezlagos/domain/internal/service/spec"
	tasksvc "nunezlagos/domain/internal/service/task"
	ticketsvc "nunezlagos/domain/internal/service/ticket"
	timelinesvc "nunezlagos/domain/internal/service/timeline"
	"nunezlagos/domain/internal/service/workflowimport"
)

// Deps colecciona las dependencias del servidor MCP.
type Deps struct {
	Observations     *obssvc.Service
	ObservationEdges *obssvc.EdgeService            // fase 1 memory graph — aristas tipadas
	CodeGraph        *codegraphsvc.CodegraphService // fase 2 code graph — nodos/aristas de código
	Projects         *projsvc.Service
	Prompts          *promptsvc.Service
	Timeline         *timelinesvc.Service
	Search           *searchsvc.Service
	Knowledge        *knowsvc.Service
	Skills           *skillsvc.Service
	SkillExecution   *skillsvc.ExecutionService // issue-12.3 domain_skill_execute
	Agents           *agentsvc.Service
	AgentRunner      *agentrunner.Runner
	Crons            *cronsvc.Service           // issue-12.3 domain_cron_list
	Clients          *clientsvc.Service         // clients/mandantes — consultoras gestionan proyectos por cliente
	CapturedPrompts  *capturedpromptsvc.Service // REQ-41 captura raw_text de usuario
	ProjectRepos     *projectreposvc.Service    // REQ-42 multi-remotos por proyecto
	ProjectPolicies  *projectpolicysvc.Service  // REQ-43 policies por proyecto
	Tickets          *ticketsvc.Service         // REQ-51 sistema de tickets internos
	Policies         *policysvc.Service         // issue-01.8 domain_policy_get/list
	Flows            *flowsvc.Service
	FlowToken        *flowsvc.FlowTokenService
	FlowRunner       *flowrunner.Runner
	Orchestrator     *orchsvc.Service            // issue-08.10 sdd-pipeline-orchestrator
	Hubuilder        *husvc.Service              // issue-04.7 interactive HU wizard
	IssueSvc         *issuesvc.Service           // domain_issue_set_status — cierre SDD
	Spec             *specsvc.Service            // domain_openspec_* — round-trip specs DB↔repo
	Tasks            *tasksvc.Service            // domain_openspec_* — sync de tasks por checkbox
	Intake           *intakesvc.Service          // issue-04.8 intake pipeline
	ExtSync          *syncsvc.Service            // issue-04.9 external provider sync
	PromptRouter     *prouter.Router             // issue-12.7 single-shot prompt router
	WorkflowImport   *workflowimport.Service     // issue-12.7 override de .md
	Pool             *pgxpool.Pool               // para queries de agent_run_logs
	Principal        *apikey.Principal           // resuelto al boot
	ErrorTracker     *observability.ErrorTracker // issue-53.9 captura el error SQL real en tx abortadas

	Dispatcher *dispatch.Dispatcher
	ServerName string
	ServerVer  string

	SharedCache CacheStore

	MetricsOnToolCall  func(ctx context.Context, tool, status, errCode, errMsg string, dur float64)
	MetricsOnCacheHit  func()
	MetricsOnCacheMiss func()
}

// toolRegistrar registra un grupo de tools MCP. Agregar un grupo nuevo
// requiere solo agregar una entrada en toolGroups — cero cambios en Tools().
type toolRegistrar func(*ResilientWrapper, Deps) []mcpgo.ServerTool

var toolGroups = []toolRegistrar{
	registerMemoryTools,
	registerMemoryGraphTools,
	registerCodeGraphTools,
	registerCatalogTools,
	registerPolicyTools,
	registerProjectTools,
	registerHUTools,
	registerIntakeTools,
	registerSyncTools,
	registerPromptTools,
	registerOrchestrateTools,
	registerCapturedPromptTools,
	registerProjectRepoTools,
	registerProjectSkillTools,
	registerSessionBootstrapTools,
	registerCronCRUDTools,
	registerProposalsTools,
	registerVerificationsTools,
	registerTicketTools,
	registerHealthTools,
	registerProjectIndexTools,
	registerOpenspecTools,
	registerWorkflowTraceTools,
	registerErrorReportingTools,
}

// defaultBudget rate limit conservador para todas las tools (issue-12.6).
// Sobreescribe per-tool en produccion segun necesidad.
var defaultBudget = ToolBudget{
	CallsPerMinute: 120,
	MaxRetries:     1,
	RetryBackoff:   100 * time.Millisecond,
	CBThreshold:    5,
	CBCooldown:     30 * time.Second,
}

// Tools construye la lista de mcpgo.ServerTool del proyecto (todos prefijo
// domain_*). Util para tests in-process que reciben []ServerTool en
// mcptest.NewServer. Produccion usa New() que internamente reusa Tools().
// Cada handler queda wrapped con ResilientWrapper (rate limit + retry).
func Tools(deps Deps) []mcpgo.ServerTool {
	wrap := NewResilientWrapper(defaultBudget)

	// authz por-tool: el allowlist del principal vigente (nil/vacío = full
	// access). Barrera anti-reentrancia del service token ACP (DOMAINSERV-85).
	wrap.SetAllowedToolsAccessor(func() []string {
		if deps.Principal == nil {
			return nil
		}
		return deps.Principal.AllowedTools
	})

	if deps.MetricsOnToolCall != nil || deps.MetricsOnCacheHit != nil || deps.MetricsOnCacheMiss != nil {
		wrap.SetMetricsHooks(deps.MetricsOnToolCall, deps.MetricsOnCacheHit, deps.MetricsOnCacheMiss)
	}

	if deps.SharedCache != nil {
		wrap.SetCache(deps.SharedCache)

		wrap.SetOrgIDAccessor(func() string {
			if deps.Principal == nil {
				return ""
			}
			return deps.Principal.OrganizationID
		})

		readTTLs := map[string]time.Duration{
			"domain_ticket_list":           5 * time.Second,
			"domain_ticket_get":            5 * time.Second,
			"domain_ticket_comment_list":   5 * time.Second,
			"domain_ticket_status_history": 5 * time.Second,
			"domain_policy_list":           15 * time.Second,
			"domain_policy_get":            15 * time.Second,
			"domain_project_list":          10 * time.Second,
			"domain_project_get":           10 * time.Second,
			"domain_client_list":           15 * time.Second,
			"domain_knowledge_list":        10 * time.Second,
			"domain_prompt_captured_list":  5 * time.Second,
			"domain_health":                10 * time.Second,
		}
		for tool, ttl := range readTTLs {
			wrap.SetCacheable(tool, ttl)
		}

		for _, w := range []string{
			"domain_ticket_create", "domain_ticket_update", "domain_ticket_delete",
			"domain_ticket_change_status", "domain_ticket_claim", "domain_ticket_release",
			"domain_ticket_reassign", "domain_ticket_comment_add",
			"domain_ticket_link_external", "domain_ticket_link_issue",
			"domain_ticket_link_external_bulk",
			"domain_policy_upsert", "domain_policy_delete",
			"domain_project_create", "domain_project_update", "domain_project_delete",
			"domain_client_create", "domain_client_update", "domain_client_delete",
			"domain_knowledge_save",
			"domain_prompt_capture", "domain_turn_complete",
		} {
			wrap.SetInvalidating(w)
		}
	}

	for _, mutTool := range []string{
		"domain_mem_save", "domain_knowledge_save",
		"domain_agent_run",
		"domain_hu_create_start", "domain_hu_create_answer",
		"domain_hu_create_preview", "domain_hu_create_commit", "domain_hu_create_abandon",
		"domain_issue_create_start", "domain_issue_create_answer",
		"domain_issue_create_preview", "domain_issue_create_commit", "domain_issue_create_abandon",
		"domain_intake_submit", "domain_intake_approve", "domain_intake_reject",
		"domain_sync_register_provider", "domain_sync_register_push",
		"domain_sync_mark_drift", "domain_sync_mark_resolved",
	} {
		wrap.SetBudget(mutTool, ToolBudget{
			CallsPerMinute: 60, MaxRetries: 1, RetryBackoff: 100 * time.Millisecond,
			CBThreshold: 5, CBCooldown: 30 * time.Second,
		})
	}

	rls := func(h mcpgo.ToolHandlerFunc) mcpgo.ToolHandlerFunc {
		return withOrgTxHandler(&deps, h)
	}
	tools := []mcpgo.ServerTool{
		{Tool: toolMemSave(), Handler: wrap.Wrap("domain_mem_save", rls(deps.handleMemSave))},
		{Tool: toolMemSearch(), Handler: wrap.Wrap("domain_mem_search", rls(deps.handleMemSearch))},
		{Tool: toolMemContext(), Handler: wrap.Wrap("domain_mem_context", rls(deps.handleMemContext))},
		{Tool: toolMemGetObservation(), Handler: wrap.Wrap("domain_mem_get_observation", rls(deps.handleMemGetObservation))},

		{Tool: toolPromptGet(), Handler: wrap.Wrap("domain_prompt_get", deps.handlePromptGet)},
		{Tool: toolPromptSearch(), Handler: wrap.Wrap("domain_prompt_search", deps.handlePromptSearch)},
		{Tool: toolContext(), Handler: wrap.Wrap("domain_context_snapshot", rls(deps.handleContext))},
		{Tool: toolTimeline(), Handler: wrap.Wrap("domain_timeline", rls(deps.handleTimeline))},
		{Tool: toolGlobalSearch(), Handler: wrap.Wrap("domain_search_global", rls(deps.handleGlobalSearch))},
		{Tool: toolKnowledgeSave(), Handler: wrap.Wrap("domain_knowledge_save", deps.handleKnowledgeSave)},
		{Tool: toolKnowledgeSearch(), Handler: wrap.Wrap("domain_knowledge_search", deps.handleKnowledgeSearch)},
		{Tool: toolKnowledgeGet(), Handler: wrap.Wrap("domain_knowledge_get", deps.handleKnowledgeGet)},
		{Tool: toolSkillList(), Handler: wrap.Wrap("domain_skill_list", deps.handleSkillList)},
		{Tool: toolSkillSearch(), Handler: wrap.Wrap("domain_skill_search", deps.handleSkillSearch)},
		{Tool: toolSkillGet(), Handler: wrap.Wrap("domain_skill_get", deps.handleSkillGet)},
		{Tool: toolAgentList(), Handler: wrap.Wrap("domain_agent_list", deps.handleAgentList)},
		{Tool: toolAgentGet(), Handler: wrap.Wrap("domain_agent_get", deps.handleAgentGet)},
		{Tool: toolAgentRun(), Handler: wrap.Wrap("domain_agent_run", deps.runAgentDispatch)},
		{Tool: toolAgentRunLogs(), Handler: wrap.Wrap("domain_agent_run_logs", deps.handleAgentRunLogs)},
		{Tool: toolFlowList(), Handler: wrap.Wrap("domain_flow_list", deps.handleFlowList)},
		{Tool: toolFlowRun(), Handler: wrap.Wrap("domain_flow_run", deps.runFlowDispatch)},
		{Tool: toolPromptRender(), Handler: wrap.Wrap("domain_prompt_render", deps.handlePromptRender)},
	}
	for _, reg := range toolGroups {
		tools = append(tools, reg(wrap, deps)...)
	}
	return tools
}

// ServerInstructions es el protocolo que el agente recibe en el
// initialize del MCP. Unica fuente: internal/agentprotocol (el mismo
// contenido se seedea en BD como policy 'agent-protocol' — la version
// viva que el agente debe preferir via domain_policy_get).
const ServerInstructions = agentprotocol.Full

// New monta el servidor MCP con los tools del prefijo `domain_*`.
func New(deps Deps) *mcpgo.MCPServer {
	srv := mcpgo.NewMCPServer(
		deps.ServerName,
		deps.ServerVer,
		mcpgo.WithToolCapabilities(true),
		mcpgo.WithInstructions(ServerInstructions),
	)
	srv.AddTools(Tools(deps)...)
	return srv
}

func toolMemSave() mcp.Tool {
	return mcp.NewTool("domain_mem_save",
		mcp.WithDescription("Guarda una observacion de memoria en el project indicado. Genera embedding automaticamente para busqueda hibrida."),
		mcp.WithString("project_slug",
			mcp.Description("Slug del project donde guardar"),
			mcp.Required(),
		),
		mcp.WithString("content",
			mcp.Description("Contenido de la observacion (texto libre)"),
			mcp.Required(),
		),
		mcp.WithString("observation_type",
			mcp.Description("Tipo: note | decision | bug | pattern | discovery (default: note)"),
		),
		mcp.WithArray("tags",
			mcp.Description("Tags opcionales para categorizar"),
			mcp.Items(map[string]any{"type": "string"}),
		),
		mcp.WithObject("metadata",
			mcp.Description("Metadata estructurada arbitraria (JSONB)"),
		),
	)
}

func toolMemSearch() mcp.Tool {
	return mcp.NewTool("domain_mem_search",
		mcp.WithDescription("Busca observations relevantes a una query usando busqueda hibrida BM25 + cosine + RRF fusion. Opcionalmente reordena con un LLM (rerank=true) para mejorar la relevancia del top; si el LLM no esta disponible degrada al orden BM25/RRF sin error."),
		mcp.WithString("query",
			mcp.Description("Texto a buscar"),
			mcp.Required(),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximo resultados (default 20, max 100)"),
		),
		mcp.WithBoolean("rerank",
			mcp.Description("Si true, reordena los candidatos con un LLM (MiniMax-M3) para mejorar relevancia. Default false (no gasta tokens). Best-effort: si el LLM no esta configurado o falla, devuelve el orden BM25/RRF original."),
		),
		mcp.WithNumber("rerank_top_n",
			mcp.Description("Cuantos candidatos del BM25/RRF se mandan al LLM para reordenar cuando rerank=true (default 30, max 50). Solo aplica si rerank=true."),
		),
	)
}

func toolMemContext() mcp.Tool {
	return mcp.NewTool("domain_mem_context",
		mcp.WithDescription("Recupera las ultimas N observations de un project, ordenadas por fecha desc. Util para contexto de sesion."),
		mcp.WithString("project_slug",
			mcp.Description("Slug del project"),
			mcp.Required(),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximo resultados (default 20, max 200)"),
		),
	)
}

func toolPromptGet() mcp.Tool {
	return mcp.NewTool("domain_prompt_get",
		mcp.WithDescription("Obtiene la version ACTIVA de un prompt template por slug. Util para inyectar prompts en runs."),
		mcp.WithString("slug",
			mcp.Description("Slug del prompt template"),
			mcp.Required(),
		),
		mcp.WithString("project_slug",
			mcp.Description("Slug del project (opcional; si vacio usa prompts globales de la org)"),
		),
	)
}

func toolPromptSearch() mcp.Tool {
	return mcp.NewTool("domain_prompt_search",
		mcp.WithDescription("Busca prompts por contenido (full-text en español) con headline destacado."),
		mcp.WithString("query",
			mcp.Description("Texto a buscar en el body del prompt"),
			mcp.Required(),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximo resultados (default 20)"),
		),
	)
}

func toolContext() mcp.Tool {
	return mcp.NewTool("domain_context_snapshot",
		mcp.WithDescription("Devuelve snapshot del contexto: observations + prompts recientes para un project."),
		mcp.WithString("project_slug",
			mcp.Description("Slug del project (opcional; vacio = scope org-wide)"),
		),
	)
}

func toolTimeline() mcp.Tool {
	return mcp.NewTool("domain_timeline",
		mcp.WithDescription("Vecindario cronologico de una observation: N entradas antes y despues incluyendo observations + prompts del mismo project."),
		mcp.WithString("observation_id",
			mcp.Description("UUID de la observation ancla"),
			mcp.Required(),
		),
		mcp.WithNumber("before",
			mcp.Description("Entradas anteriores (default 3, max 50)"),
		),
		mcp.WithNumber("after",
			mcp.Description("Entradas posteriores (default 3, max 50)"),
		),
	)
}

func toolGlobalSearch() mcp.Tool {
	return mcp.NewTool("domain_search_global",
		mcp.WithDescription("Busqueda global cross-entity (observations + prompts) scoped por org del principal. Filtros opcionales."),
		mcp.WithString("query",
			mcp.Description("Texto a buscar"),
			mcp.Required(),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximo resultados (default 20, max 200)"),
		),
		mcp.WithArray("entity_types",
			mcp.Description("Filtrar a tipos especificos: observation, prompt"),
			mcp.Items(map[string]any{"type": "string"}),
		),
		mcp.WithArray("tags",
			mcp.Description("Tags requeridos (AND)"),
			mcp.Items(map[string]any{"type": "string"}),
		),
	)
}

func toolAgentRunLogs() mcp.Tool {
	return mcp.NewTool("domain_agent_run_logs",
		mcp.WithDescription("Recupera los logs detallados (llm_call/tool_call/tool_result/error/final) de un agent_run."),
		mcp.WithString("run_id",
			mcp.Description("UUID del agent_run"),
			mcp.Required(),
		),
	)
}

func toolFlowList() mcp.Tool {
	return mcp.NewTool("domain_flow_list",
		mcp.WithDescription("Lista los flows definidos en la org."),
		mcp.WithNumber("limit", mcp.Description("Maximo 200 (default 50)")),
	)
}

func toolFlowRun() mcp.Tool {
	return mcp.NewTool("domain_flow_run",
		mcp.WithDescription("Ejecuta un flow por id con inputs opcionales. Retorna run_id + status + outputs por step."),
		mcp.WithString("flow_id",
			mcp.Description("UUID del flow"),
			mcp.Required(),
		),
		mcp.WithObject("inputs",
			mcp.Description("Variables que se pasan a los steps (template {{inputs.x}})"),
		),
	)
}

func toolPromptRender() mcp.Tool {
	return mcp.NewTool("domain_prompt_render",
		mcp.WithDescription("Obtiene un prompt activo por slug y renderiza variables {{name}} con args."),
		mcp.WithString("slug",
			mcp.Description("Slug del prompt"),
			mcp.Required(),
		),
		mcp.WithString("project_slug",
			mcp.Description("Project (opcional, default org-level)"),
		),
		mcp.WithObject("variables",
			mcp.Description("Variables {{name}} → value"),
		),
	)
}

func toolAgentRun() mcp.Tool {
	return mcp.NewTool("domain_agent_run",
		mcp.WithDescription("Ejecuta un agent server-side: el agent llama al LLM configurado, usa sus skills como tools, y devuelve la respuesta. Domain orquesta todo el loop."),
		mcp.WithString("agent_slug",
			mcp.Description("Slug del agent a ejecutar"),
			mcp.Required(),
		),
		mcp.WithString("input",
			mcp.Description("Mensaje del usuario al agent"),
			mcp.Required(),
		),
		mcp.WithObject("variables",
			mcp.Description("Variables opcionales contextuales"),
		),
		mcp.WithString("flow_run_id",
			mcp.Description("UUID del flow_run si este agent es parte de un pipeline SDD. Inyecta FlowRunContext al dispatcher para que el agent reciba contexto de fase sin heredar el historial completo del orquestador."),
		),
		mcp.WithString("phase_slug",
			mcp.Description("Slug de la fase SDD actual (sdd-apply, sdd-verify, etc.) cuando flow_run_id está presente."),
		),
	)
}

func toolAgentList() mcp.Tool {
	return mcp.NewTool("domain_agent_list",
		mcp.WithDescription("Lista los agents disponibles en la org."),
		mcp.WithNumber("limit",
			mcp.Description("Maximo resultados (default 50)"),
		),
	)
}

func toolAgentGet() mcp.Tool {
	return mcp.NewTool("domain_agent_get",
		mcp.WithDescription("Recupera detalle de un agent por id o slug."),
		mcp.WithString("id",
			mcp.Description("UUID del agent (opcional si se pasa slug)"),
		),
		mcp.WithString("slug",
			mcp.Description("Slug del agent (opcional si se pasa id)"),
		),
	)
}

func toolSkillList() mcp.Tool {
	return mcp.NewTool("domain_skill_list",
		mcp.WithDescription("Lista los skills disponibles en la org. Filtros opcionales por type/tag."),
		mcp.WithString("type",
			mcp.Description("Filtrar por tipo: prompt | code | api | mcp_tool"),
		),
		mcp.WithString("tag",
			mcp.Description("Filtrar por tag especifico"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximo resultados (default 50)"),
		),
	)
}

func toolSkillSearch() mcp.Tool {
	return mcp.NewTool("domain_skill_search",
		mcp.WithDescription("Busca skills por similitud semantica + BM25 sobre name+description."),
		mcp.WithString("query",
			mcp.Description("Texto descriptivo del capability buscado"),
			mcp.Required(),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximo resultados (default 20)"),
		),
	)
}

func toolSkillGet() mcp.Tool {
	return mcp.NewTool("domain_skill_get",
		mcp.WithDescription("Recupera detalle completo de un skill por id o slug."),
		mcp.WithString("id",
			mcp.Description("UUID del skill (opcional si se pasa slug)"),
		),
		mcp.WithString("slug",
			mcp.Description("Slug del skill (opcional si se pasa id)"),
		),
	)
}

func toolKnowledgeSave() mcp.Tool {
	return mcp.NewTool("domain_knowledge_save",
		mcp.WithDescription("Guarda un documento de conocimiento. Se chunkea automaticamente y genera embeddings por chunk para RAG."),
		mcp.WithString("project_slug",
			mcp.Description("Slug del project donde guardar"),
			mcp.Required(),
		),
		mcp.WithString("title",
			mcp.Description("Titulo del documento"),
			mcp.Required(),
		),
		mcp.WithString("body",
			mcp.Description("Contenido completo (texto largo OK; se chunkea)"),
			mcp.Required(),
		),
		mcp.WithString("source",
			mcp.Description("Fuente: manual | imported | web | etc. (default manual)"),
		),
		mcp.WithString("source_url",
			mcp.Description("URL origen si aplica"),
		),
		mcp.WithArray("tags",
			mcp.Description("Tags opcionales"),
			mcp.Items(map[string]any{"type": "string"}),
		),
	)
}

func toolKnowledgeSearch() mcp.Tool {
	return mcp.NewTool("domain_knowledge_search",
		mcp.WithDescription("Busqueda hibrida (vector + BM25 + RRF) sobre chunks de knowledge documents."),
		mcp.WithString("query",
			mcp.Description("Texto de busqueda"),
			mcp.Required(),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximo resultados (default 20)"),
		),
	)
}

func toolKnowledgeGet() mcp.Tool {
	return mcp.NewTool("domain_knowledge_get",
		mcp.WithDescription("Recupera un knowledge document completo (todos sus chunks reconstruidos)."),
		mcp.WithString("id",
			mcp.Description("UUID del documento"),
			mcp.Required(),
		),
	)
}

func toolMemGetObservation() mcp.Tool {
	return mcp.NewTool("domain_mem_get_observation",
		mcp.WithDescription("Recupera una observation especifica por ID (UUID)."),
		mcp.WithString("id",
			mcp.Description("UUID de la observation"),
			mcp.Required(),
		),
	)
}
