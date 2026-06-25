// Package handler — HTTP handlers REST /api/v1/*.
//
// Router minimalista usando net/http patterns Go 1.22+ (method + path).
// Middleware stack: requestID → recover → metrics → auth (skip allowlist) → handler.
//
// Convenciones (rules/api.md):
//   - JSON requests/responses, error shape {"error":{"code","message",...}}
//   - 201 con Location, 204 en DELETE, 422 validation, 401 unauthorized
//   - X-Request-Id correlacion, X-Rate-Limit-* cuando aplica
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



	WebhookDispatcher *WebhookDispatcher


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


	EventBus *events.Bus


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













	mux.HandleFunc("POST /api/v1/auth/request-otp", a.requestOTP)
	mux.HandleFunc("POST /api/v1/auth/verify-otp", a.verifyOTP)


	mux.HandleFunc("POST /api/v1/auth/login", a.authLogin)


	mux.HandleFunc("GET /api/v1/auth/first-run", a.authFirstRun)
	mux.HandleFunc("POST /api/v1/auth/bootstrap", a.authBootstrap)











	mux.HandleFunc("POST /api/v1/auth/enroll", a.enrollSelf)














































































	mux.HandleFunc("POST /api/v1/webhooks/{slug}/receive", a.receiveWebhook)








































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



// ensureJSONSlice: si data es un slice nil, devuelve un []T{} vacio para que
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

// writeDataWithMeta envia {data, ...extra} para responses con pagination/warnings/etc.
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
















// ErrCrossOrg es el sentinel que devuelve authorizeOrg cuando el recurso
// pertenece a otra org. Mismo trato que ErrNotFound de los services:
// el handler responde 404, no 403 (anti-enumeration: no revela existencia
// de recursos en otras orgs).
var ErrCrossOrg = errors.New("handler: cross-org access denied")

// orgID devuelve el OrganizationID del principal autenticado, parseado
// como uuid.UUID por el middleware PrincipalCtx. Retorna uuid.Nil si
// el request no paso por auth (no deberia ocurrir en paths /api/v1/*).
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
// Uso tipico:
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
