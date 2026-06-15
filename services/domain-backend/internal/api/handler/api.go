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

	"github.com/google/uuid"

	"nunezlagos/domain/internal/activity"
	"nunezlagos/domain/internal/api/ctxkeys"
	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/dispatch"
	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/auth/bootstrap"
	"nunezlagos/domain/internal/auth/ratelimit"
	specsvc "nunezlagos/domain/internal/service/spec"
	tsvc "nunezlagos/domain/internal/service/task"
	tracesvc "nunezlagos/domain/internal/service/traceability"
	attSvc "nunezlagos/domain/internal/service/attachment"
	"nunezlagos/domain/internal/auth/otp"
	"nunezlagos/domain/internal/secrets"
	"nunezlagos/domain/internal/service/issuebuilder"
	intakesvc "nunezlagos/domain/internal/service/intake"
	"nunezlagos/domain/internal/service/promptrouter"
	"nunezlagos/domain/internal/service/workflowimport"
	agentrunner "nunezlagos/domain/internal/runner/agent"
	flowrunner "nunezlagos/domain/internal/runner/flow"
	agentsvc "nunezlagos/domain/internal/service/agent"
	"nunezlagos/domain/internal/service/billing"
	capturedpromptsvc "nunezlagos/domain/internal/service/capturedprompt"
	clientsvc "nunezlagos/domain/internal/service/client"
	projectpolicysvc "nunezlagos/domain/internal/service/projectpolicy"
	projectreposvc "nunezlagos/domain/internal/service/projectrepo"
	ticketsvc "nunezlagos/domain/internal/service/ticket"
	"nunezlagos/domain/internal/service/cost"
	cronsvc "nunezlagos/domain/internal/service/cron"
	"nunezlagos/domain/internal/service/flow"
	"nunezlagos/domain/internal/service/invite"
	"nunezlagos/domain/internal/service/knowledge"
	"nunezlagos/domain/internal/service/lifecycle"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/api/backpressure"
	"nunezlagos/domain/internal/dbmon"
	"nunezlagos/domain/internal/service/enrollment"
	"nunezlagos/domain/internal/service/usage"
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
	usvc "nunezlagos/domain/internal/service/issue"
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
	ClientService  *clientsvc.Service
	TicketService  *ticketsvc.Service // REQ-51 sistema de tickets internos
	CapturedPromptService *capturedpromptsvc.Service // REQ-41+47 captura+token tracking
	ProjectRepoService    *projectreposvc.Service    // REQ-42 multi-remotos
	ProjectPolicyService  *projectpolicysvc.Service  // REQ-43 policies por proyecto
	Pool           *pgxpool.Pool // REQ-49/50 — queries directas para proposals/verifications
	ObsService     *observation.Service
	InviteService  *invite.Service
	SessionService  *sesssvc.Service
	PromptService   *promptsvc.Service
	TimelineService  *timelinesvc.Service
	SearchService    *searchsvc.Service
	KnowledgeService *knowledge.Service
	LifecycleService *lifecycle.Service
	SkillService     *skillsvc.Service
	SkillExecution   *skillsvc.ExecutionService
	AgentService     *agentsvc.Service
	AgentRunner      *agentrunner.Runner
	FlowService      *flow.Service
	FlowRunner       *flowrunner.Runner
	CronService      *cronsvc.Service
	WebhookService   *webhooksvc.Service
	// Dispatcher (issue-35.1): si no nil, webhook dispatch delega acá.
	// Si nil, usa el switch legacy (compat).
	Dispatcher *dispatch.Dispatcher
	CostService      *cost.Service
	BillingService   *billing.Service
	OutboundWebhookService     *outboundwebhook.Service
	OutboundWebhookDispatcher  *outboundwebhook.Dispatcher
	OutboundWebhookRequireTLS  bool
	Backpressure               *backpressure.Limiter
	DBMonCollector             *dbmon.Collector
	UsageAlertsService         *usagealerts.Service
	UsageSnapshot              *usage.Service
	Enrollment                 *enrollment.Service
	MCPServerService           *mcpserver.Service
	ProjectTemplateService     *projecttemplate.Service
	PolicyService              *policy.Service
	RuntimeConfigRegistry     *runtimeconfig.Registry
	DBStatsService            *dbstats.Service
	Audit          *audit.PGRecorder
	ActivityRecorder activity.Recorder
	ActivityQuerier  activity.Querier
	OTPService     *otp.Service
	OTPRateLimiter *ratelimit.Limiter
	APIKeys        *apikey.PGStore
	Bootstrap      *bootstrap.Service
	SecretsStore   *secrets.PGStore
	RoleService    *rolesvc.Service
	ReqService     *reqsvc.Service
	HUService      *usvc.Service
	SpecService    *specsvc.Service
	TaskService    *tsvc.Service
	TraceService        *tracesvc.Service
	AttachmentService   *attSvc.Service
	Hubuilder           *issuebuilder.Service
	IssueBuilderAdaptive   *issuebuilder.AdaptiveService
	IntakeService       *intakesvc.Service
	PromptRouter        *promptrouter.Router
	WorkflowImport      *workflowimport.Service
}

// Router devuelve un http.Handler montado en /api/v1/*.
func (a *API) Router() http.Handler {
	mux := http.NewServeMux()

	// Auth (sin Bearer requerido)
	mux.HandleFunc("POST /api/v1/auth/request-otp", a.requestOTP)
	mux.HandleFunc("POST /api/v1/auth/verify-otp", a.verifyOTP)
	// Bootstrap (issue-01.9): first-run detection + auto-create primer user.
	// Tambien sin Bearer: la primera request al sistema no tiene user todavía.
	mux.HandleFunc("GET /api/v1/auth/first-run", a.authFirstRun)
	mux.HandleFunc("POST /api/v1/auth/bootstrap", a.authBootstrap)

	// Audit logs (issue-02.4, requiere auth)
	mux.HandleFunc("GET /api/v1/audit-logs", a.listAuditLogs)

	// API keys CRUD (issue-02.1)
	mux.HandleFunc("GET /api/v1/api-keys", a.listAPIKeys)
	mux.HandleFunc("POST /api/v1/api-keys", a.createAPIKey)
	mux.HandleFunc("DELETE /api/v1/api-keys/{id}", a.revokeAPIKey)

	// Activity logs (issue-02.6)
	mux.HandleFunc("GET /api/v1/activity-logs", a.listActivityLogs)

	// Organizaciones (require auth)
	mux.HandleFunc("POST /api/v1/organizations", a.createOrg)
	mux.HandleFunc("GET /api/v1/organizations/{id}", a.getOrg)
	mux.HandleFunc("PATCH /api/v1/organizations/{id}", a.updateOrg)
	mux.HandleFunc("DELETE /api/v1/organizations/{id}", a.deleteOrg)
	mux.HandleFunc("GET /api/v1/organizations/{id}/members", a.listMembers)
	// issue-36.1: onboarding sin email — admin/owner crea user + api_key directo
	mux.HandleFunc("POST /api/v1/organizations/{id}/members", a.addMemberWithKey)
	// issue-37.1: self-enrollment con token compartido (público + admin rotate)
	mux.HandleFunc("POST /api/v1/auth/enroll", a.enrollSelf)
	mux.HandleFunc("POST /api/v1/organizations/{id}/enrollment-token/rotate", a.rotateEnrollmentToken)
	mux.HandleFunc("GET /api/v1/organizations/{id}/enrollment-token", a.getEnrollmentTokenMetadata)
	mux.HandleFunc("DELETE /api/v1/organizations/{id}/enrollment-token", a.deleteEnrollmentToken)
	mux.HandleFunc("POST /api/v1/organizations/{id}/transfer-ownership", a.transferOwnership)

	// Requirements (issue-04.1) — SDD dogfood
	mux.HandleFunc("POST /api/v1/requirements", a.createRequirement)
	mux.HandleFunc("GET /api/v1/requirements", a.listRequirements)
	mux.HandleFunc("GET /api/v1/requirements/{slug}", a.getRequirement)
	mux.HandleFunc("PATCH /api/v1/requirements/{slug}", a.updateRequirement)
	mux.HandleFunc("POST /api/v1/requirements/{slug}/archive", a.archiveRequirement)
	mux.HandleFunc("GET /api/v1/requirements/{slug}/tree", a.getRequirementTree)

	// User stories (issue-04.2)
	mux.HandleFunc("POST /api/v1/user-stories", a.createUserStory)
	mux.HandleFunc("GET /api/v1/user-stories", a.listUserStories)
	mux.HandleFunc("GET /api/v1/user-stories/{slug}", a.getUserStory)
	mux.HandleFunc("PATCH /api/v1/user-stories/{slug}", a.updateUserStory)
	mux.HandleFunc("DELETE /api/v1/user-stories/{slug}", a.deleteUserStory)
	mux.HandleFunc("POST /api/v1/user-stories/{slug}/scenarios", a.addScenario)
	mux.HandleFunc("DELETE /api/v1/user-stories/{slug}/scenarios/{id}", a.removeScenario)

	// Proposals & Designs (issue-04.3)
	mux.HandleFunc("POST /api/v1/user-stories/{slug}/proposals", a.createProposal)
	mux.HandleFunc("GET /api/v1/user-stories/{slug}/proposals", a.listProposalVersions)
	mux.HandleFunc("GET /api/v1/user-stories/{slug}/proposals/latest", a.getLatestProposal)
	mux.HandleFunc("PATCH /api/v1/proposals/{id}/status", a.changeProposalStatus)
	mux.HandleFunc("POST /api/v1/user-stories/{slug}/designs", a.createDesign)
	mux.HandleFunc("GET /api/v1/user-stories/{slug}/designs", a.listDesigns)
	mux.HandleFunc("GET /api/v1/user-stories/{slug}/designs/latest", a.getLatestDesign)
	mux.HandleFunc("PATCH /api/v1/designs/{id}/status", a.changeDesignStatus)

	// Tasks (issue-04.4)
	mux.HandleFunc("POST /api/v1/user-stories/{slug}/tasks", a.createTasks)
	mux.HandleFunc("GET /api/v1/user-stories/{slug}/tasks", a.listTasks)
	mux.HandleFunc("GET /api/v1/tasks/{id}", a.getTask)
	mux.HandleFunc("PATCH /api/v1/tasks/{id}/status", a.updateTaskStatus)
	mux.HandleFunc("POST /api/v1/tasks/{id}/verification", a.createVerification)
	mux.HandleFunc("POST /api/v1/tasks/{id}/sabotage", a.createSabotage)
	mux.HandleFunc("GET /api/v1/user-stories/{slug}/progress", a.getProgress)

	// Traceability (issue-04.5)
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

	// Attachments / S3 (issue-04.6)
	mux.HandleFunc("POST /api/v1/attachments", a.initUpload)
	mux.HandleFunc("POST /api/v1/attachments/{id}/confirm", a.confirmUpload)
	mux.HandleFunc("GET /api/v1/attachments/{id}/download", a.getAttachmentDownload)
	mux.HandleFunc("GET /api/v1/attachments", a.listAttachments)
	mux.HandleFunc("DELETE /api/v1/attachments/{id}", a.deleteAttachment)

	// Custom roles (issue-02.8)
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

	// Clients/mandantes — consultoras que gestionan proyectos por cliente.
	// {id_or_slug} en GET admite ambos; mutaciones requieren UUID explícito.
	mux.HandleFunc("POST /api/v1/clients", a.createClient)
	mux.HandleFunc("GET /api/v1/clients", a.listClients)
	mux.HandleFunc("GET /api/v1/clients/{id_or_slug}", a.getClient)
	mux.HandleFunc("PUT /api/v1/clients/{id}", a.updateClient)
	mux.HandleFunc("DELETE /api/v1/clients/{id}", a.deleteClient)
	mux.HandleFunc("POST /api/v1/clients/{id}/restore", a.restoreClient)
	mux.HandleFunc("POST /api/v1/clients/{id}/status", a.setClientStatus)

	// REQ-51 Tickets (sistema de issues internos por proyecto)
	mux.HandleFunc("POST /api/v1/projects/{slug}/tickets", a.createTicket)
	mux.HandleFunc("GET /api/v1/tickets", a.listTickets) // ?project_slug=&status=&assignee_id=...
	mux.HandleFunc("GET /api/v1/tickets/{id_or_key}", a.getTicket)
	mux.HandleFunc("PATCH /api/v1/tickets/{id}", a.updateTicket)
	mux.HandleFunc("DELETE /api/v1/tickets/{id}", a.deleteTicket)
	mux.HandleFunc("POST /api/v1/tickets/{id}/status", a.changeTicketStatus)
	mux.HandleFunc("GET /api/v1/tickets/{id}/comments", a.listTicketComments)
	mux.HandleFunc("POST /api/v1/tickets/{id}/comments", a.addTicketComment)
	mux.HandleFunc("GET /api/v1/tickets/{id}/history", a.listTicketStatusHistory)
	mux.HandleFunc("POST /api/v1/tickets/{id}/link-external", a.linkTicketExternal)
	mux.HandleFunc("POST /api/v1/tickets/{id}/link-issue", a.linkTicketIssue)
	// REQ-58: bulk + webhook Jira (stub). Endpoint listo para cuando se
	// conecte Jira; hoy responde si DOMAIN_JIRA_WEBHOOK_SECRET está set.
	mux.HandleFunc("POST /api/v1/tickets/link-external-bulk", a.bulkLinkTicketsExternal)
	mux.HandleFunc("POST /api/v1/webhooks/jira/issue-updated", a.jiraWebhookIssueUpdated)

	// REQ-52 REST endpoints adicionales para el dashboard
	// Captured prompts + usage summary (REQ-41/47)
	mux.HandleFunc("GET /api/v1/captured-prompts", a.listCapturedPrompts)
	mux.HandleFunc("GET /api/v1/usage/turn-summary", a.usageTurnSummary)
	// Project repositories (REQ-42)
	mux.HandleFunc("GET /api/v1/projects/{slug}/repositories", a.listProjectRepos)
	mux.HandleFunc("POST /api/v1/projects/{slug}/repositories", a.addProjectRepo)
	mux.HandleFunc("DELETE /api/v1/project-repositories/{id}", a.deleteProjectRepo)
	// Project policies (REQ-43) read-only desde REST
	mux.HandleFunc("GET /api/v1/projects/{slug}/policies", a.listProjectPolicies)
	// Proposals (REQ-49)
	mux.HandleFunc("GET /api/v1/proposals", a.listProposals)
	mux.HandleFunc("POST /api/v1/proposals/{kind}/{id}/review", a.reviewProposal)
	// Verifications (REQ-50) read pending
	mux.HandleFunc("GET /api/v1/projects/{slug}/verifications", a.listVerifications)

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
	mux.HandleFunc("GET /api/v1/agents/{id}/versions", a.listAgentVersions)
	mux.HandleFunc("DELETE /api/v1/agents/{id}", a.deleteAgent)
	mux.HandleFunc("POST /api/v1/agents/{id}/run", a.runAgent)
	mux.HandleFunc("GET /api/v1/agent-runs/{id}/logs", a.getAgentRunLogs)

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
	mux.HandleFunc("GET /api/v1/dlq", a.listDLQ)
	mux.HandleFunc("DELETE /api/v1/dlq/{id}", a.resolveDLQ)

	// Webhooks inbound (público, HMAC auth — slug + secret en config)
	mux.HandleFunc("POST /api/v1/webhooks/{slug}/receive", a.receiveWebhook)

	// Skills
	mux.HandleFunc("POST /api/v1/skills", a.createSkill)
	mux.HandleFunc("GET /api/v1/skills", a.listSkills)       // ?type= &tag= &limit=
	mux.HandleFunc("GET /api/v1/skills/search", a.searchSkills) // ?q=
	mux.HandleFunc("GET /api/v1/skills/{id}", a.getSkill)
	mux.HandleFunc("PATCH /api/v1/skills/{id}", a.updateSkill)
	mux.HandleFunc("DELETE /api/v1/skills/{id}", a.deleteSkill)
	mux.HandleFunc("POST /api/v1/skills/{id}/execute", a.executeSkill)
	mux.HandleFunc("GET /api/v1/executions/{id}", a.getExecution)

	// Lifecycle (issue-23.2 restore + issue-23.3 GDPR export)
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

	// Cost analytics (issue-15.1 + issue-15.2)
	mux.HandleFunc("GET /api/v1/cost/daily", a.getCostDaily) // ?days=N&group_by=org|agent
	mux.HandleFunc("GET /api/v1/cost/spend/{granularity}", a.getCostSpend)
	mux.HandleFunc("GET /api/v1/cost/breakdown/{dimension}", a.getCostBreakdown)
	mux.HandleFunc("GET /api/v1/cost/forecast", a.getCostForecast)
	mux.HandleFunc("POST /api/v1/cost/budgets", a.createBudget)
	mux.HandleFunc("GET /api/v1/cost/budgets", a.listBudgets)
	mux.HandleFunc("DELETE /api/v1/cost/budgets/{id}", a.deleteBudget)
	mux.HandleFunc("GET /api/v1/cost/export", a.exportCost)
	mux.HandleFunc("GET /api/v1/usage", a.getCurrentUsage)   // issue-21.3 plan usage actual
	// Quota snapshot read-only (issue-33.4)
	mux.HandleFunc("GET /api/v1/usage/current", a.usageCurrentSnapshot)
	mux.HandleFunc("GET /api/v1/usage/history", a.usageHistory)

	// Admin DB stats (issue-25.12)
	mux.HandleFunc("GET /api/v1/admin/db-stats", a.getDBStats)

	// Runtime configs (issue-27.3)
	mux.HandleFunc("GET /api/v1/admin/runtime-configs/{key}", a.getRuntimeConfig)
	mux.HandleFunc("POST /api/v1/admin/runtime-configs/{key}", a.updateRuntimeConfig)

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

	// HU builder (issue-04.7)
	mux.HandleFunc("POST /api/v1/hu-drafts", a.startHubDraft)
	mux.HandleFunc("POST /api/v1/hu-drafts/{id}/answer", a.answerHubDraft)
	mux.HandleFunc("GET /api/v1/hu-drafts/{id}/preview", a.previewHubDraft)
	mux.HandleFunc("POST /api/v1/hu-drafts/{id}/commit", a.commitHubDraft)
	mux.HandleFunc("POST /api/v1/hu-drafts/{id}/abandon", a.abandonHubDraft)
	mux.HandleFunc("GET /api/v1/hu-drafts", a.listHubDrafts)

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
		"/health/ready",
		"/health/startup",
		"/api/v1/auth/request-otp",
		"/api/v1/auth/verify-otp",
		"/api/v1/auth/enroll", // issue-37.1: gating por X-Enrollment-Token, no Bearer
		"/api/v1/webhooks/*", // webhooks usan HMAC, no Bearer
		"/metrics",
	}
}

// --- JSON helpers ---

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
