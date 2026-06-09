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
	"encoding/json"
	"net/http"

	"nunezlagos/domain/internal/activity"
	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/auth/apikey"
	specsvc "nunezlagos/domain/internal/service/spec"
	tsvc "nunezlagos/domain/internal/service/task"
	tracesvc "nunezlagos/domain/internal/service/traceability"
	attSvc "nunezlagos/domain/internal/service/attachment"
	"nunezlagos/domain/internal/auth/otp"
	agentrunner "nunezlagos/domain/internal/runner/agent"
	flowrunner "nunezlagos/domain/internal/runner/flow"
	agentsvc "nunezlagos/domain/internal/service/agent"
	"nunezlagos/domain/internal/service/billing"
	"nunezlagos/domain/internal/service/cost"
	"nunezlagos/domain/internal/service/flow"
	"nunezlagos/domain/internal/service/invite"
	"nunezlagos/domain/internal/service/knowledge"
	"nunezlagos/domain/internal/service/lifecycle"
	"nunezlagos/domain/internal/api/backpressure"
	"nunezlagos/domain/internal/dbmon"
	"nunezlagos/domain/internal/service/usagealerts"
	"nunezlagos/domain/internal/service/mcpserver"
	"nunezlagos/domain/internal/dbstats"
	"nunezlagos/domain/internal/runtimeconfig"
	"nunezlagos/domain/internal/service/policy"
	"nunezlagos/domain/internal/service/projecttemplate"
	"nunezlagos/domain/internal/service/observation"
	"nunezlagos/domain/internal/service/outboundwebhook"
	orgsvc "nunezlagos/domain/internal/service/org"
	projsvc "nunezlagos/domain/internal/service/project"
	promptsvc "nunezlagos/domain/internal/service/prompt"
	reqsvc "nunezlagos/domain/internal/service/requirement"
	rolesvc "nunezlagos/domain/internal/service/role"
	usvc "nunezlagos/domain/internal/service/userstory"
	searchsvc "nunezlagos/domain/internal/service/search"
	sesssvc "nunezlagos/domain/internal/service/session"
	skillsvc "nunezlagos/domain/internal/service/skill"
	timelinesvc "nunezlagos/domain/internal/service/timeline"
	webhooksvc "nunezlagos/domain/internal/service/webhook"
)

// API agrupa todas las dependencias y monta el router /api/v1/*.
type API struct {
	OrgService     *orgsvc.Service
	ProjectService *projsvc.Service
	ObsService     *observation.Service
	InviteService  *invite.Service
	SessionService  *sesssvc.Service
	PromptService   *promptsvc.Service
	TimelineService  *timelinesvc.Service
	SearchService    *searchsvc.Service
	KnowledgeService *knowledge.Service
	LifecycleService *lifecycle.Service
	SkillService     *skillsvc.Service
	AgentService     *agentsvc.Service
	AgentRunner      *agentrunner.Runner
	FlowService      *flow.Service
	FlowRunner       *flowrunner.Runner
	WebhookService   *webhooksvc.Service
	CostService      *cost.Service
	BillingService   *billing.Service
	OutboundWebhookService     *outboundwebhook.Service
	OutboundWebhookDispatcher  *outboundwebhook.Dispatcher
	OutboundWebhookRequireTLS  bool
	Backpressure               *backpressure.Limiter
	DBMonCollector             *dbmon.Collector
	UsageAlertsService         *usagealerts.Service
	MCPServerService           *mcpserver.Service
	ProjectTemplateService     *projecttemplate.Service
	PolicyService              *policy.Service
	RuntimeConfigRegistry     *runtimeconfig.Registry
	DBStatsService            *dbstats.Service
	Audit          *audit.PGRecorder
	ActivityRecorder activity.Recorder
	ActivityQuerier  activity.Querier
	OTPService     *otp.Service
	APIKeys        *apikey.PGStore
	RoleService    *rolesvc.Service
	ReqService     *reqsvc.Service
	HUService      *usvc.Service
	SpecService    *specsvc.Service
	TaskService    *tsvc.Service
	TraceService        *tracesvc.Service
	AttachmentService   *attSvc.Service
}

// Router devuelve un http.Handler montado en /api/v1/*.
func (a *API) Router() http.Handler {
	mux := http.NewServeMux()

	// Auth (sin Bearer requerido)
	mux.HandleFunc("POST /api/v1/auth/request-otp", a.requestOTP)
	mux.HandleFunc("POST /api/v1/auth/verify-otp", a.verifyOTP)

	// Audit logs (HU-02.4, requiere auth)
	mux.HandleFunc("GET /api/v1/audit-logs", a.listAuditLogs)

	// API keys CRUD (HU-02.1)
	mux.HandleFunc("GET /api/v1/api-keys", a.listAPIKeys)
	mux.HandleFunc("POST /api/v1/api-keys", a.createAPIKey)
	mux.HandleFunc("DELETE /api/v1/api-keys/{id}", a.revokeAPIKey)

	// Activity logs (HU-02.6)
	mux.HandleFunc("GET /api/v1/activity-logs", a.listActivityLogs)

	// Organizaciones (require auth)
	mux.HandleFunc("POST /api/v1/organizations", a.createOrg)
	mux.HandleFunc("GET /api/v1/organizations/{id}", a.getOrg)
	mux.HandleFunc("PATCH /api/v1/organizations/{id}", a.updateOrg)
	mux.HandleFunc("DELETE /api/v1/organizations/{id}", a.deleteOrg)
	mux.HandleFunc("GET /api/v1/organizations/{id}/members", a.listMembers)
	mux.HandleFunc("POST /api/v1/organizations/{id}/transfer-ownership", a.transferOwnership)

	// Requirements (HU-04.1) — SDD dogfood
	mux.HandleFunc("POST /api/v1/requirements", a.createRequirement)
	mux.HandleFunc("GET /api/v1/requirements", a.listRequirements)
	mux.HandleFunc("GET /api/v1/requirements/{slug}", a.getRequirement)
	mux.HandleFunc("PATCH /api/v1/requirements/{slug}", a.updateRequirement)
	mux.HandleFunc("POST /api/v1/requirements/{slug}/archive", a.archiveRequirement)
	mux.HandleFunc("GET /api/v1/requirements/{slug}/tree", a.getRequirementTree)

	// User stories (HU-04.2)
	mux.HandleFunc("POST /api/v1/user-stories", a.createUserStory)
	mux.HandleFunc("GET /api/v1/user-stories", a.listUserStories)
	mux.HandleFunc("GET /api/v1/user-stories/{slug}", a.getUserStory)
	mux.HandleFunc("PATCH /api/v1/user-stories/{slug}", a.updateUserStory)
	mux.HandleFunc("DELETE /api/v1/user-stories/{slug}", a.deleteUserStory)
	mux.HandleFunc("POST /api/v1/user-stories/{slug}/scenarios", a.addScenario)
	mux.HandleFunc("DELETE /api/v1/user-stories/{slug}/scenarios/{id}", a.removeScenario)

	// Proposals & Designs (HU-04.3)
	mux.HandleFunc("POST /api/v1/user-stories/{slug}/proposals", a.createProposal)
	mux.HandleFunc("GET /api/v1/user-stories/{slug}/proposals", a.listProposalVersions)
	mux.HandleFunc("GET /api/v1/user-stories/{slug}/proposals/latest", a.getLatestProposal)
	mux.HandleFunc("PATCH /api/v1/proposals/{id}/status", a.changeProposalStatus)
	mux.HandleFunc("POST /api/v1/user-stories/{slug}/designs", a.createDesign)
	mux.HandleFunc("GET /api/v1/user-stories/{slug}/designs", a.listDesigns)
	mux.HandleFunc("GET /api/v1/user-stories/{slug}/designs/latest", a.getLatestDesign)
	mux.HandleFunc("PATCH /api/v1/designs/{id}/status", a.changeDesignStatus)

	// Tasks (HU-04.4)
	mux.HandleFunc("POST /api/v1/user-stories/{slug}/tasks", a.createTasks)
	mux.HandleFunc("GET /api/v1/user-stories/{slug}/tasks", a.listTasks)
	mux.HandleFunc("GET /api/v1/tasks/{id}", a.getTask)
	mux.HandleFunc("PATCH /api/v1/tasks/{id}/status", a.updateTaskStatus)
	mux.HandleFunc("POST /api/v1/tasks/{id}/verification", a.createVerification)
	mux.HandleFunc("POST /api/v1/tasks/{id}/sabotage", a.createSabotage)
	mux.HandleFunc("GET /api/v1/user-stories/{slug}/progress", a.getProgress)

	// Traceability (HU-04.5)
	mux.HandleFunc("GET /api/v1/traceability/req/{slug}", a.getRequirementTrace)
	mux.HandleFunc("GET /api/v1/traceability/code", a.getCodeTrace)
	mux.HandleFunc("GET /api/v1/traceability/coverage", a.getCoverageDashboard)
	mux.HandleFunc("GET /api/v1/traceability/progress", a.getProgressReport)
	mux.HandleFunc("GET /api/v1/traceability/consolidated", a.getConsolidatedReport)
	mux.HandleFunc("GET /api/v1/traceability/gaps/no-proposal", a.getHUsWithoutProposals)
	mux.HandleFunc("GET /api/v1/traceability/gaps/no-design", a.getHUsWithoutDesigns)
	mux.HandleFunc("GET /api/v1/traceability/gaps/incomplete-tasks", a.getHUsWithIncompleteTasks)
	mux.HandleFunc("POST /api/v1/traceability/code-refs", a.addCodeReference)
	mux.HandleFunc("DELETE /api/v1/traceability/code-refs/{id}", a.removeCodeReference)

	// Attachments / S3 (HU-04.6)
	mux.HandleFunc("POST /api/v1/attachments", a.initUpload)
	mux.HandleFunc("POST /api/v1/attachments/{id}/confirm", a.confirmUpload)
	mux.HandleFunc("GET /api/v1/attachments/{id}/download", a.getAttachmentDownload)
	mux.HandleFunc("GET /api/v1/attachments", a.listAttachments)
	mux.HandleFunc("DELETE /api/v1/attachments/{id}", a.deleteAttachment)

	// Custom roles (HU-02.8)
	mux.HandleFunc("GET /api/v1/organizations/{id}/roles", a.listRoles)
	mux.HandleFunc("POST /api/v1/organizations/{id}/roles", a.createRole)
	mux.HandleFunc("GET /api/v1/organizations/{id}/roles/{slug}", a.getRole)
	mux.HandleFunc("PATCH /api/v1/organizations/{id}/roles/{slug}", a.updateRole)
	mux.HandleFunc("DELETE /api/v1/organizations/{id}/roles/{slug}", a.deleteRole)
	mux.HandleFunc("POST /api/v1/organizations/{id}/members/{user_id}/role", a.assignRole)

	// Invitations
	mux.HandleFunc("POST /api/v1/organizations/{id}/invitations", a.createInvite)
	mux.HandleFunc("GET /api/v1/organizations/{id}/invitations", a.listInvites)
	mux.HandleFunc("POST /api/v1/invitations/{token}/accept", a.acceptInvite)
	mux.HandleFunc("POST /api/v1/invitations/{token}/decline", a.declineInvite)
	mux.HandleFunc("POST /api/v1/invitations/{id}/revoke", a.revokeInvite)

	// Projects (scoped by org via ?organization_id= o derivado del principal)
	mux.HandleFunc("POST /api/v1/projects", a.createProject)
	mux.HandleFunc("GET /api/v1/projects", a.listProjects)
	mux.HandleFunc("GET /api/v1/projects/{slug}", a.getProject)
	mux.HandleFunc("PATCH /api/v1/projects/{slug}", a.updateProject)
	mux.HandleFunc("DELETE /api/v1/projects/{slug}", a.deleteProject)

	// Observations
	mux.HandleFunc("POST /api/v1/observations", a.saveObservation)
	mux.HandleFunc("GET /api/v1/observations/{id}", a.getObservation)
	mux.HandleFunc("DELETE /api/v1/observations/{id}", a.deleteObservation)
	mux.HandleFunc("GET /api/v1/observations", a.listObservations) // ?project_slug=
	mux.HandleFunc("GET /api/v1/search", a.searchObservations)     // ?q=

	// Sessions
	mux.HandleFunc("POST /api/v1/sessions", a.startSession)
	mux.HandleFunc("GET /api/v1/sessions", a.listSessions)
	mux.HandleFunc("GET /api/v1/sessions/active", a.activeSession) // ?project_slug=
	mux.HandleFunc("GET /api/v1/sessions/{id}", a.getSession)
	mux.HandleFunc("POST /api/v1/sessions/{id}/end", a.endSession)

	// Agents
	mux.HandleFunc("POST /api/v1/agents", a.createAgent)
	mux.HandleFunc("GET /api/v1/agents", a.listAgents)
	mux.HandleFunc("GET /api/v1/agents/{id}", a.getAgent)
	mux.HandleFunc("PATCH /api/v1/agents/{id}", a.updateAgent)
	mux.HandleFunc("DELETE /api/v1/agents/{id}", a.deleteAgent)
	mux.HandleFunc("POST /api/v1/agents/{id}/run", a.runAgent)
	mux.HandleFunc("GET /api/v1/agent-runs/{id}/logs", a.getAgentRunLogs)

	// Flows
	mux.HandleFunc("POST /api/v1/flows", a.createFlow)
	mux.HandleFunc("GET /api/v1/flows", a.listFlows)
	mux.HandleFunc("GET /api/v1/flows/{id}", a.getFlow)
	mux.HandleFunc("DELETE /api/v1/flows/{id}", a.deleteFlow)
	mux.HandleFunc("POST /api/v1/flows/{id}/run", a.runFlow)
	mux.HandleFunc("POST /api/v1/flows/{id}/dry-run", a.dryRunFlow)

	// Webhooks inbound (público, HMAC auth — slug + secret en config)
	mux.HandleFunc("POST /api/v1/webhooks/{slug}/receive", a.receiveWebhook)

	// Skills
	mux.HandleFunc("POST /api/v1/skills", a.createSkill)
	mux.HandleFunc("GET /api/v1/skills", a.listSkills)       // ?type= &tag= &limit=
	mux.HandleFunc("GET /api/v1/skills/search", a.searchSkills) // ?q=
	mux.HandleFunc("GET /api/v1/skills/{id}", a.getSkill)
	mux.HandleFunc("PATCH /api/v1/skills/{id}", a.updateSkill)
	mux.HandleFunc("DELETE /api/v1/skills/{id}", a.deleteSkill)

	// Lifecycle (HU-23.2 restore + HU-23.3 GDPR export)
	mux.HandleFunc("POST /api/v1/restore", a.restoreEntity)
	mux.HandleFunc("GET /api/v1/me/export", a.exportMyData)
	mux.HandleFunc("POST /api/v1/me/erase", a.eraseMyData)

	// Knowledge documents
	mux.HandleFunc("POST /api/v1/knowledge", a.saveKnowledge)
	mux.HandleFunc("GET /api/v1/knowledge", a.listKnowledge)        // ?project_slug=
	mux.HandleFunc("GET /api/v1/knowledge/search", a.searchKnowledge) // ?q=
	mux.HandleFunc("GET /api/v1/knowledge/{id}", a.getKnowledge)
	mux.HandleFunc("DELETE /api/v1/knowledge/{id}", a.deleteKnowledge)

	// Context + Timeline (cross-entity feeds)
	mux.HandleFunc("GET /api/v1/context", a.getContext)                          // ?project_slug=
	mux.HandleFunc("GET /api/v1/observations/{id}/timeline", a.getTimeline)      // ?before= &after=

	// Prompts (templates versionados)
	mux.HandleFunc("POST /api/v1/prompts", a.createPrompt)
	mux.HandleFunc("GET /api/v1/prompts/{id}", a.getPrompt)
	mux.HandleFunc("POST /api/v1/prompts/{id}/activate", a.setActivePrompt)
	mux.HandleFunc("DELETE /api/v1/prompts/{id}", a.deletePrompt)
	mux.HandleFunc("GET /api/v1/prompts/by-slug/{slug}/versions", a.listPromptVersions)
	mux.HandleFunc("GET /api/v1/prompts/search", a.searchPrompts)

	// Cost analytics (HU-15.1 + HU-15.2)
	mux.HandleFunc("GET /api/v1/cost/daily", a.getCostDaily) // ?days=N&group_by=org|agent
	mux.HandleFunc("GET /api/v1/usage", a.getCurrentUsage)   // HU-21.3 plan usage actual

	// Admin DB stats (HU-25.12)
	mux.HandleFunc("GET /api/v1/admin/db-stats", a.getDBStats)

	// Runtime configs (HU-27.3)
	mux.HandleFunc("GET /api/v1/admin/runtime-configs/{key}", a.getRuntimeConfig)
	mux.HandleFunc("POST /api/v1/admin/runtime-configs/{key}", a.updateRuntimeConfig)

	// Slow queries (HU-25.2)
	mux.HandleFunc("GET /api/v1/admin/db/slow-queries", a.getSlowQueries)

	// Usage alerts (HU-15.3)
	mux.HandleFunc("POST /api/v1/usage-alerts", a.createUsageAlert)
	mux.HandleFunc("GET /api/v1/usage-alerts", a.listUsageAlerts)
	mux.HandleFunc("PATCH /api/v1/usage-alerts/{id}", a.updateUsageAlert)
	mux.HandleFunc("GET /api/v1/usage-alerts/{id}/fires", a.listUsageAlertFires)
	mux.HandleFunc("DELETE /api/v1/usage-alerts/{id}", a.deleteUsageAlert)

	// Platform policies (HU-01.8)
	mux.HandleFunc("POST /api/v1/platform/policies", a.createPolicy)
	mux.HandleFunc("GET /api/v1/platform/policies", a.listPolicies)
	mux.HandleFunc("GET /api/v1/platform/policies/{slug}", a.getPolicyBySlug)
	mux.HandleFunc("PATCH /api/v1/platform/policies/{id}", a.updatePolicy)
	mux.HandleFunc("DELETE /api/v1/platform/policies/{id}", a.deletePolicy)

	// Project templates (HU-01.4)
	mux.HandleFunc("POST /api/v1/project-templates", a.createProjectTemplate)
	mux.HandleFunc("GET /api/v1/project-templates", a.listProjectTemplates)
	mux.HandleFunc("GET /api/v1/project-templates/{id}", a.getProjectTemplate)
	mux.HandleFunc("DELETE /api/v1/project-templates/{id}", a.deleteProjectTemplate)

	// MCP servers externos (HU-12.4)
	mux.HandleFunc("POST /api/v1/mcp-servers", a.createMCPServer)
	mux.HandleFunc("GET /api/v1/mcp-servers", a.listMCPServers)
	mux.HandleFunc("GET /api/v1/mcp-servers/{id}", a.getMCPServer)
	mux.HandleFunc("DELETE /api/v1/mcp-servers/{id}", a.deleteMCPServer)
	mux.HandleFunc("POST /api/v1/mcp-servers/{id}/sync-tools", a.syncMCPTools)
	mux.HandleFunc("GET /api/v1/mcp-servers/{id}/tools", a.listMCPTools)
	mux.HandleFunc("POST /api/v1/mcp-servers/{id}/invoke", a.invokeMCPTool)

	// Outbound webhooks (HU-10.4)
	mux.HandleFunc("POST /api/v1/outbound-webhooks", a.createOutboundWebhook)
	mux.HandleFunc("GET /api/v1/outbound-webhooks", a.listOutboundWebhooks)
	mux.HandleFunc("GET /api/v1/outbound-webhooks/{id}", a.getOutboundWebhook)
	mux.HandleFunc("DELETE /api/v1/outbound-webhooks/{id}", a.deleteOutboundWebhook)
	mux.HandleFunc("POST /api/v1/outbound-webhooks/{id}/test", a.testOutboundWebhook)

	return mux
}

// Allowlist paths que skipean auth (definida en uno solo lugar para evitar drift).
// Sufijo "/*" hace prefix match (e.g. webhooks autenticados por HMAC).
func AuthAllowlist() []string {
	return []string{
		"/health",
		"/health/ready",
		"/health/startup",
		"/api/v1/auth/request-otp",
		"/api/v1/auth/verify-otp",
		"/api/v1/webhooks/*", // webhooks usan HMAC, no Bearer
		"/metrics",
	}
}

// --- JSON helpers ---

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{"code": code, "message": msg},
	})
}

func writeData(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, map[string]any{"data": data})
}

// writeDataWithMeta envía {data, ...extra} para responses con pagination/warnings/etc.
// Las keys de extra NO deben colisionar con "data".
func writeDataWithMeta(w http.ResponseWriter, status int, data any, extra map[string]any) {
	body := map[string]any{"data": data}
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
