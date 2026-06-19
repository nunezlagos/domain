// Package handler — HTTP handlers REST /api/v1/*.
//
// Router minimalista usando net/http patterns Go 1.22+ (method + path).
// Middleware stack: requestID → recover → metrics → auth (skip allowlist) → handler.
//
// Convenciones (rules/api.md):
//   - JSON requests/responses, error shape {"error":{"code","message",...}}
//   - 201 con Location, 204 en DELETE, 422 validation, 401 unauthorized
//   - X-Request-Id correlación, X-Rate-Limit-* cuando aplica
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"reflect"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v5/pgxpool"
	"nunezlagos/domain/internal/activity"
	"nunezlagos/domain/internal/api/ctxkeys"
	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/auth/bootstrap"
	"nunezlagos/domain/internal/auth/otp"
	"nunezlagos/domain/internal/auth/ratelimit"
	"nunezlagos/domain/internal/auth/session"
	"nunezlagos/domain/internal/dispatch"
	"nunezlagos/domain/internal/events"
	agentrunner "nunezlagos/domain/internal/runner/agent"
	flowrunner "nunezlagos/domain/internal/runner/flow"
	"nunezlagos/domain/internal/secrets"
	agentsvc "nunezlagos/domain/internal/service/agent"
	attSvc "nunezlagos/domain/internal/service/attachment"
	"nunezlagos/domain/internal/service/billing"
	capturedpromptsvc "nunezlagos/domain/internal/service/capturedprompt"
	clientsvc "nunezlagos/domain/internal/service/client"
	"nunezlagos/domain/internal/service/cost"
	cronsvc "nunezlagos/domain/internal/service/cron"
	"nunezlagos/domain/internal/service/flow"
	intakesvc "nunezlagos/domain/internal/service/intake"
	"nunezlagos/domain/internal/service/issuebuilder"
	"nunezlagos/domain/internal/service/knowledge"
	"nunezlagos/domain/internal/service/lifecycle"
	projectpolicysvc "nunezlagos/domain/internal/service/projectpolicy"
	projectreposvc "nunezlagos/domain/internal/service/projectrepo"
	"nunezlagos/domain/internal/service/promptrouter"
	specsvc "nunezlagos/domain/internal/service/spec"
	tsvc "nunezlagos/domain/internal/service/task"
	ticketsvc "nunezlagos/domain/internal/service/ticket"
	tracesvc "nunezlagos/domain/internal/service/traceability"
	"nunezlagos/domain/internal/service/workflowimport"

	"nunezlagos/domain/internal/api/backpressure"
	"nunezlagos/domain/internal/dbmon"
	"nunezlagos/domain/internal/dbstats"
	"nunezlagos/domain/internal/service/enrollment"
	usvc "nunezlagos/domain/internal/service/issue"
	"nunezlagos/domain/internal/service/mcpserver"
	"nunezlagos/domain/internal/service/observation"
	"nunezlagos/domain/internal/service/outboundwebhook"
	"nunezlagos/domain/internal/service/policy"
	projsvc "nunezlagos/domain/internal/service/project"
	"nunezlagos/domain/internal/service/projecttemplate"
	promptsvc "nunezlagos/domain/internal/service/prompt"
	reqsvc "nunezlagos/domain/internal/service/requirement"
	searchsvc "nunezlagos/domain/internal/service/search"
	skillsvc "nunezlagos/domain/internal/service/skill"
	timelinesvc "nunezlagos/domain/internal/service/timeline"
	"nunezlagos/domain/internal/service/usage"
	"nunezlagos/domain/internal/service/usagealerts"
	webhooksvc "nunezlagos/domain/internal/service/webhook"
)

// API agrupa todas las dependencias y monta el router /api/v1/*.
type API struct {
	ProjectService        *projsvc.Service
	ClientService         *clientsvc.Service
	TicketService         *ticketsvc.Service         // REQ-51 sistema de tickets internos
	CapturedPromptService *capturedpromptsvc.Service // REQ-41+47 captura+token tracking
	ProjectRepoService    *projectreposvc.Service    // REQ-42 multi-remotos
	ProjectPolicyService  *projectpolicysvc.Service  // REQ-43 policies por proyecto
	Pool                  *pgxpool.Pool              // REQ-49/50 — queries directas para proposals/verifications
	ObsService            *observation.Service
	PromptService         *promptsvc.Service
	TimelineService       *timelinesvc.Service
	SearchService         *searchsvc.Service
	KnowledgeService      *knowledge.Service
	LifecycleService      *lifecycle.Service
	SkillService          *skillsvc.Service
	SkillExecution        *skillsvc.ExecutionService
	AgentService          *agentsvc.Service
	AgentRunner           *agentrunner.Runner
	FlowService           *flow.Service
	FlowRunner            *flowrunner.Runner
	CronService           *cronsvc.Service
	WebhookService        *webhooksvc.Service
	// ISSUE-28.7: dispatcher para webhooks inbound. Bounded queue +
	// WaitGroup + per-job timeout. Si nil, el handler cae al fallback
	// legacy (go func() directo).
	WebhookDispatcher *WebhookDispatcher
	// Dispatcher (issue-35.1): si no nil, webhook dispatch delega acá.
	// Si nil, usa el switch legacy (compat).
	Dispatcher                *dispatch.Dispatcher
	CostService               *cost.Service
	BillingService            *billing.Service
	OutboundWebhookService    *outboundwebhook.Service
	OutboundWebhookDispatcher *outboundwebhook.Dispatcher
	OutboundWebhookRequireTLS bool
	Backpressure              *backpressure.Limiter
	DBMonCollector            *dbmon.Collector
	UsageAlertsService        *usagealerts.Service
	UsageSnapshot             *usage.Service
	Enrollment                *enrollment.Service
	MCPServerService          *mcpserver.Service
	ProjectTemplateService    *projecttemplate.Service
	PolicyService             *policy.Service
	DBStatsService            *dbstats.Service
	Audit                     *audit.PGRecorder
	ActivityRecorder          activity.Recorder
	ActivityQuerier           activity.Querier
	OTPService                *otp.Service
	OTPRateLimiter            *ratelimit.Limiter
	APIKeys                   *apikey.PGStore
	Bootstrap                 *bootstrap.Service
	SecretsStore              *secrets.PGStore
	ReqService                *reqsvc.Service
	HUService                 *usvc.Service
	SpecService               *specsvc.Service
	TaskService               *tsvc.Service
	TraceService              *tracesvc.Service
	AttachmentService         *attSvc.Service
	Hubuilder                 *issuebuilder.Service
	IssueBuilderAdaptive      *issuebuilder.AdaptiveService
	IntakeService             *intakesvc.Service
	PromptRouter              *promptrouter.Router
	WorkflowImport            *workflowimport.Service
	// REQ-69 SSE event bus para broadcast de cambios al dashboard /
	// otros clientes. Si nil, el endpoint /api/v1/events devuelve 503.
	EventBus *events.Bus
	// REQ-72 auth web (login + roles + sessions). Si nil, los endpoints
	// /api/v1/auth/* devuelven 503.
	AuthSessionService *session.Service
}

// SessionFromContext re-export para que handlers fuera del package
// session lean el Active sin importar session directamente. REQ-72.
func SessionFromContext(ctx context.Context) (*session.Active, bool) {
	return session.FromContext(ctx)
}

// Router devuelve un http.Handler montado en /api/v1/*.
func (a *API) Router() http.Handler {
	mux := http.NewServeMux()

	// REQ-43.10 (Ola 8): endpoints auth web + misc + lifecycle removidos.
// Consumidos solo por el admin Angular archivado. Mantener handlers como dead code.
//
//	Removidos:
//	  - /api/v1/auth/{login,select-role,me,refresh,logout} (vistas admin login/session)
//	  - /api/v1/me/roles (switcher de rol del header)
//	  - /api/v1/captured-prompts (vista admin-captured-prompts)
//	  - /api/v1/usage/turn-summary (vista admin-usage)
//	  - /api/v1/restore (admin restore UI)
//	  - /api/v1/me/export, /api/v1/me/erase (GDPR endpoints del admin)
//
// Mantenidos (consumidos por CLI installer): /auth/request-otp, /auth/verify-otp, /auth/first-run, /auth/bootstrap, /auth/login
	mux.HandleFunc("POST /api/v1/auth/request-otp", a.requestOTP)
	mux.HandleFunc("POST /api/v1/auth/verify-otp", a.verifyOTP)
	// REQ-72 auth web (login user+password con roles). /auth/login se mantiene
	// por compat con CLI installer; el resto del flujo web se removió en Ola 8.
	mux.HandleFunc("POST /api/v1/auth/login", a.authLogin)
	// Bootstrap (issue-01.9): first-run detection + auto-create primer user.
	// Tambien sin Bearer: la primera request al sistema no tiene user todavía.
	mux.HandleFunc("GET /api/v1/auth/first-run", a.authFirstRun)
	mux.HandleFunc("POST /api/v1/auth/bootstrap", a.authBootstrap)

	// REQ-43.3 (Ola 1): endpoints solo-Angular removidos. Consumidos únicamente
	// por el admin Angular archivado en openspec/archive/2026-06-19-domain-admin-angular/.
	// Removidos: /audit-logs, /api-keys, /activity-logs, enrollment-token admin.
	// Mantenidos: /auth/enroll (CLI installer consume).

	// Organización (single-org): la entidad organization se removió. Los
	// endpoints de org settings (GET/PATCH /organizations/{id}) y member
	// management (/organizations/{id}/members) ya no existen. Se conserva
	// solo /auth/enroll (onboarding global, consumido por CLI).
	// issue-37.1: self-enrollment con token compartido (público)
	mux.HandleFunc("POST /api/v1/auth/enroll", a.enrollSelf)

	// REQ-43.4 (Ola 2): endpoints SDD/TDD removidos. Consumidos solo por el admin
	// Angular archivado (vistas admin-requirements, admin-user-stories, admin-proposals,
//	admin-designs, admin-tasks, admin-traceability, admin-hu-builder). Mantener
	// handlers en struct API como dead code hasta HU-43.13 (cleanup).
//
//	Removidos en esta ola:
//	  - /api/v1/requirements/* (POST/GET/{slug}/PATCH/archive/tree)
//	  - /api/v1/user-stories/* (POST/GET/{slug}/PATCH/DELETE/scenarios)
//	  - /api/v1/user-stories/{slug}/{proposals,designs}/*
//	  - /api/v1/proposals/{id}/status
//	  - /api/v1/designs/{id}/status
//	  - /api/v1/user-stories/{slug}/tasks
//	  - /api/v1/tasks/{id} y sub-rutas (status, verification, sabotage, progress)
//	  - /api/v1/traceability/* (req/{slug}, code, coverage, progress, consolidated, gaps/*, code-refs)

	// REQ-43.6 (Ola 3): Projects + Clients + Project-Templates + Attachments removidos.
// Consumidos solo por el admin Angular archivado Y mockeados en SDK tests (breaking
// change en SDKs publicados aceptado por el dueño del proyecto, junio 2026).
// El MCP no usa estos endpoints (consume Services directamente), asi que el sunset
// del HTTP no afecta tools MCP.
//
// Removidos:
//   - /api/v1/attachments/* (POST/POST confirm/GET download/GET list/DELETE)
//   - /api/v1/projects/* (POST/GET/{slug}/PATCH/DELETE)
//   - /api/v1/clients/* (POST/GET/{id_or_slug}/PUT/DELETE/restore/status)

	// REQ-43.3 (Ola 1): endpoints Tickets, Users, Events SSE y Jira webhook removidos.
// Consumidos únicamente por el admin Angular archivado. Mantener handler en código
// (caller methods quedan en struct API) evita romper referencias en tests; el borrado
// físico de handlers + sus tests se difiere a la HU-43.13 (cleanup post-sunset).
//
// Removidos en esta ola:
//   - /api/v1/projects/{slug}/tickets, /api/v1/tickets/*
//   - /api/v1/users (modal reasignar)
//   - /api/v1/events (SSE dashboard)
//   - /api/v1/tickets/link-external-bulk
//   - /api/v1/webhooks/jira/issue-updated

	// REQ-43.10 (Ola 8): captured-prompts + usage/turn-summary removidos
// (eran vistas admin-captured-prompts y admin-usage).

	// REQ-43.7 (Ola 4/5): Project-Repositories + Project-Policies + Proposals + Verifications + Observations + Search removidos.
// Consumidos solo por admin Angular Y mockeados en SDK tests. El MCP consume los Services
// directamente (project_repo_tools.go, verifications_tools.go, etc.).
//
// Removidos:
//   - /api/v1/projects/{slug}/repositories (GET/POST)
//   - /api/v1/project-repositories/{id} (DELETE)
//   - /api/v1/projects/{slug}/policies (GET)
//   - /api/v1/proposals (GET) + /api/v1/proposals/{kind}/{id}/review (POST)
//   - /api/v1/projects/{slug}/verifications (GET)
//   - /api/v1/observations (POST/GET/{id}/DELETE/GET list/search)

// REQ-42.3: rutas /api/v1/sessions removidas (tabla sessions dropeada,
// feature legacy duplicada de auth_sessions).

	// REQ-43.7 (Ola 4): Agents + Agent Runs endpoints removidos.
// Consumidos solo por admin Angular (vistas admin-agents, admin-runs) Y mockeados
// en SDK tests. El MCP consume AgentService/AgentRunner directamente.
//
// Removidos:
//   - /api/v1/agents/* (POST/GET/{id}/PATCH/{id}/versions/DELETE/{id}/run)
//   - /api/v1/agent-runs/{id}/logs (GET)

	// Inbound webhooks management (issue-10.2). El receive público vive en
	// /api/v1/webhooks/{slug}/receive (sin Bearer; HMAC).
	mux.HandleFunc("POST /api/v1/inbound-webhooks", a.createInboundWebhook)
	mux.HandleFunc("GET /api/v1/inbound-webhooks", a.listInboundWebhooks)
	mux.HandleFunc("GET /api/v1/inbound-webhooks/{id}", a.getInboundWebhook)
	mux.HandleFunc("PATCH /api/v1/inbound-webhooks/{id}", a.patchInboundWebhook)
	mux.HandleFunc("DELETE /api/v1/inbound-webhooks/{id}", a.deleteInboundWebhook)
	mux.HandleFunc("GET /api/v1/inbound-webhooks/{id}/deliveries", a.listWebhookDeliveries)
	mux.HandleFunc("POST /api/v1/inbound-webhooks/deliveries/{id}/replay", a.replayWebhookDelivery)

	// Crons (issue-10.1)
	mux.HandleFunc("POST /api/v1/crons", a.createCron)
	mux.HandleFunc("GET /api/v1/crons", a.listCrons)
	mux.HandleFunc("GET /api/v1/crons/{id}", a.getCron)
	mux.HandleFunc("PATCH /api/v1/crons/{id}", a.patchCron)
	mux.HandleFunc("DELETE /api/v1/crons/{id}", a.deleteCron)
	mux.HandleFunc("GET /api/v1/crons/{id}/history", a.cronHistory)

	// Flows
	mux.HandleFunc("POST /api/v1/flows", a.createFlow)
	mux.HandleFunc("GET /api/v1/flows", a.listFlows)
	mux.HandleFunc("GET /api/v1/flows/{id}", a.getFlow)
	mux.HandleFunc("PATCH /api/v1/flows/{id}", a.updateFlow)
	mux.HandleFunc("PUT /api/v1/flows/{id}", a.replaceFlow)
	mux.HandleFunc("GET /api/v1/flows/{id}/export", a.exportFlow)
	mux.HandleFunc("GET /api/v1/flows/{id}/parents", a.listFlowParents)
	mux.HandleFunc("POST /api/v1/flows/import", a.importFlow)
	mux.HandleFunc("DELETE /api/v1/flows/{id}", a.deleteFlow)
	mux.HandleFunc("POST /api/v1/flows/{id}/run", a.runFlow)
	mux.HandleFunc("POST /api/v1/flows/{id}/dry-run", a.dryRunFlow)
	mux.HandleFunc("POST /api/v1/runs/{id}/signals", a.signalFlowRun)
	mux.HandleFunc("GET /api/v1/flow-runs/{id}", a.getFlowRun)
	mux.HandleFunc("POST /api/v1/flow-runs/{id}/pause", a.pauseFlowRun)
	mux.HandleFunc("POST /api/v1/flow-runs/{id}/resume", a.resumeFlowRun)
	mux.HandleFunc("POST /api/v1/flow-runs/{id}/cancel", a.cancelFlowRun)
	mux.HandleFunc("GET /api/v1/flow-runs/{id}/stream", a.streamFlowRun)
	// REQ-42.3: rutas /api/v1/dlq removidas (tabla dead_letter_queue dropeada).

	// Webhooks inbound (público, HMAC auth — slug + secret en config)
	mux.HandleFunc("POST /api/v1/webhooks/{slug}/receive", a.receiveWebhook)

	// REQ-43.7 (Ola 4): Skills + Prompts endpoints removidos.
// Consumidos solo por admin Angular (vistas admin-skills, admin-prompts) Y mockeados
// en SDK tests (breaking change aceptado). El MCP consume SkillService/PromptService
// directamente, asi que tools MCP domain_skill_* y domain_prompt_* siguen funcionando.
//
// Removidos:
//   - /api/v1/skills/* (POST/GET/search/{id}/PATCH/DELETE/execute)
//   - /api/v1/executions/{id} (GET)
//   - /api/v1/prompts/* (POST/GET/{id}/activate/DELETE/by-slug/{slug}/versions/search)

	// REQ-43.10 (Ola 8): lifecycle endpoints (restore, GDPR export/erase) removidos.
// Consumidos solo por admin Angular (vistas admin-lifecycle y admin-account).

	// REQ-43.7 (Ola 4/5): Knowledge + Context + Timeline + Prompts removidos.
// Consumidos solo por admin Angular (vistas admin-knowledge, admin-prompts, admin-context)
// Y mockeados en SDK tests. El MCP consume KnowledgeService/PromptService directamente.
//
// Removidos:
//   - /api/v1/knowledge/* (POST/GET/search/{id}/DELETE)
//   - /api/v1/context (GET)
//   - /api/v1/observations/{id}/timeline (GET)
//   - /api/v1/prompts/* (POST/GET/{id}/activate/DELETE/by-slug/{slug}/versions/search)

	// Cost analytics (issue-15.1). REQ-42.2: spend/breakdown/forecast/budgets/
	// export se eliminaron junto con el dominio billing/costos (cost_logs/budgets).
	mux.HandleFunc("GET /api/v1/cost/daily", a.getCostDaily) // ?days=N&group_by=org|agent
	mux.HandleFunc("GET /api/v1/usage", a.getCurrentUsage)   // issue-21.3 usage actual
	// Quota snapshot read-only (issue-33.4)
	mux.HandleFunc("GET /api/v1/usage/current", a.usageCurrentSnapshot)
	mux.HandleFunc("GET /api/v1/usage/history", a.usageHistory)

	// Admin DB stats (issue-25.12)
	mux.HandleFunc("GET /api/v1/admin/db-stats", a.getDBStats)
	mux.HandleFunc("GET /api/v1/admin/db-schema", a.getDBSchema)
	// HU-41.2: dashboard org overview (stats + top users + recent activity)
	mux.HandleFunc("GET /api/v1/admin/org-overview", a.getOrgOverview)

	// REQ-42.3: rutas /api/v1/admin/runtime-configs removidas (tabla
	// runtime_configs dropeada, feature hot-reload eliminado).

	// Slow queries (issue-25.2)
	mux.HandleFunc("GET /api/v1/admin/db/slow-queries", a.getSlowQueries)

	// Usage alerts (issue-15.3)
	mux.HandleFunc("POST /api/v1/usage-alerts", a.createUsageAlert)
	mux.HandleFunc("GET /api/v1/usage-alerts", a.listUsageAlerts)
	mux.HandleFunc("PATCH /api/v1/usage-alerts/{id}", a.updateUsageAlert)
	mux.HandleFunc("GET /api/v1/usage-alerts/{id}/fires", a.listUsageAlertFires)
	mux.HandleFunc("DELETE /api/v1/usage-alerts/{id}", a.deleteUsageAlert)

	// Platform policies (issue-01.8)
	mux.HandleFunc("POST /api/v1/platform/policies", a.createPolicy)
	mux.HandleFunc("GET /api/v1/platform/policies", a.listPolicies)
	mux.HandleFunc("GET /api/v1/platform/policies/{slug}", a.getPolicyBySlug)
	mux.HandleFunc("PATCH /api/v1/platform/policies/{id}", a.updatePolicy)
	mux.HandleFunc("DELETE /api/v1/platform/policies/{id}", a.deletePolicy)

	// Project templates (issue-01.4)
	mux.HandleFunc("POST /api/v1/project-templates", a.createProjectTemplate)
	mux.HandleFunc("GET /api/v1/project-templates", a.listProjectTemplates)
	mux.HandleFunc("GET /api/v1/project-templates/{id}", a.getProjectTemplate)
	mux.HandleFunc("DELETE /api/v1/project-templates/{id}", a.deleteProjectTemplate)

	// REQ-43.4 (Ola 2): HU builder endpoints removidos.
// Consumidos solo por el admin Angular (vista admin-hu-builder).
// Removidos: /api/v1/hu-drafts/*

	// MCP servers externos (issue-12.4)
	mux.HandleFunc("POST /api/v1/mcp-servers", a.createMCPServer)
	mux.HandleFunc("GET /api/v1/mcp-servers", a.listMCPServers)
	mux.HandleFunc("GET /api/v1/mcp-servers/{id}", a.getMCPServer)
	mux.HandleFunc("DELETE /api/v1/mcp-servers/{id}", a.deleteMCPServer)
	mux.HandleFunc("POST /api/v1/mcp-servers/{id}/sync-tools", a.syncMCPTools)
	mux.HandleFunc("GET /api/v1/mcp-servers/{id}/tools", a.listMCPTools)
	mux.HandleFunc("POST /api/v1/mcp-servers/{id}/invoke", a.invokeMCPTool)

	// Outbound webhooks (issue-10.4)
	mux.HandleFunc("POST /api/v1/outbound-webhooks", a.createOutboundWebhook)
	mux.HandleFunc("GET /api/v1/outbound-webhooks", a.listOutboundWebhooks)
	mux.HandleFunc("GET /api/v1/outbound-webhooks/{id}", a.getOutboundWebhook)
	mux.HandleFunc("DELETE /api/v1/outbound-webhooks/{id}", a.deleteOutboundWebhook)
	mux.HandleFunc("POST /api/v1/outbound-webhooks/{id}/test", a.testOutboundWebhook)
	mux.HandleFunc("POST /api/v1/outbound-webhooks/deliveries/{id}/replay", a.replayOutboundDelivery)

	return mux
}

// Allowlist paths que skipean auth (definida en uno solo lugar para evitar drift).
// Sufijo "/*" hace prefix match (e.g. webhooks autenticados por HMAC).
func AuthAllowlist() []string {
	return []string{
		"/health",
		"/healthz",
		"/health/ready",
		"/health/startup",
		"/api/v1/auth/request-otp",
		"/api/v1/auth/verify-otp",
		"/api/v1/auth/login",
		"/api/v1/auth/select-role",
		"/api/v1/auth/enroll", // issue-37.1: gating por X-Enrollment-Token, no Bearer
		"/api/v1/webhooks/*",  // webhooks usan HMAC, no Bearer
		"/metrics",
	}
}

// --- JSON helpers ---

// ensureJSONSlice: si data es un slice nil, devuelve un []T{} vacío para que
// JSON serialice como [] en vez de null. Si data es otro tipo (struct, map, string),
// lo devuelve tal cual.
func ensureJSONSlice(data any) any {
	if data == nil {
		return []any{}
	}
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Slice && v.IsNil() {
		return reflect.MakeSlice(v.Type(), 0, 0).Interface()
	}
	return data
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	// HU-28.5: status ya escrito → best effort. Si encode rompe (cliente
	// desconectado, stream cerrado), loggeamos pero no podemos cambiar el
	// status code. No enmascaramos el error tragándolo en `_`.
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("failed to encode response", "error", err, "status", status)
	}
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{"code": code, "message": msg},
	})
}

func writeData(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, map[string]any{"data": ensureJSONSlice(data)})
}

// writeDataWithMeta envía {data, ...extra} para responses con pagination/warnings/etc.
// Las keys de extra NO deben colisionar con "data".
func writeDataWithMeta(w http.ResponseWriter, status int, data any, extra map[string]any) {
	body := map[string]any{"data": ensureJSONSlice(data)}
	for k, v := range extra {
		if k == "data" {
			continue
		}
		body[k] = v
	}
	writeJSON(w, status, body)
}

func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// --- Principal / cross-org helpers (HU-28.3) ---
//
// Estos helpers reemplazan el patrón:
//
//   p, _ := principal(r)
//   if p == nil { writeError(...) ; return }
//   orgID, _ := uuid.Parse(p.OrganizationID)
//   if resource.OrganizationID != orgID { writeError(404) ; return }
//
// El middleware middleware.PrincipalCtx (post-auth) ya parseó los UUIDs
// y los inyectó en el ctx vía ctxkeys. Los handlers solo llaman a estos
// accesores. Si el caller NO está autenticado (allowlist path), orgID(ctx)
// devuelve uuid.Nil — los handlers de paths autenticados pueden asumir
// no-nil porque apikey.Middleware bloquea antes.

// ErrCrossOrg es el sentinel que devuelve authorizeOrg cuando el recurso
// pertenece a otra org. Mismo trato que ErrNotFound de los services:
// el handler responde 404, no 403 (anti-enumeration: no revela existencia
// de recursos en otras orgs).
var ErrCrossOrg = errors.New("handler: cross-org access denied")

// orgID devuelve el OrganizationID del principal autenticado, parseado
// como uuid.UUID por el middleware PrincipalCtx. Retorna uuid.Nil si
// el request no pasó por auth (no debería ocurrir en paths /api/v1/*).
func (a *API) orgID(ctx context.Context) uuid.UUID {
	return ctxkeys.OrgID(ctx)
}

// userID devuelve el UserID del principal autenticado.
func (a *API) userID(ctx context.Context) uuid.UUID {
	return ctxkeys.UserID(ctx)
}

// authorizeOrg verifica que el recurso pertenezca a la org del request.
// Retorna ErrCrossOrg si no coincide. El handler debe traducirlo a 404
// (no 403) para no filtrar existencia de recursos cross-org.
//
// Uso típico:
//
//	flow, err := a.FlowService.GetByID(ctx, id)
//	if err != nil { ... }
//	if err := a.authorizeOrg(ctx, flow.OrganizationID); err != nil {
//	    writeError(w, http.StatusNotFound, "not_found", "")
//	    return
//	}
func (a *API) authorizeOrg(ctx context.Context, resourceOrgID uuid.UUID) error {
	if a.orgID(ctx) == resourceOrgID {
		return nil
	}
	return ErrCrossOrg
}
