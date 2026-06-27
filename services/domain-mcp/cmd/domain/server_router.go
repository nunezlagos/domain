package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/activity"
	"nunezlagos/domain/internal/api/handler"
	"nunezlagos/domain/internal/api/middleware"
	"nunezlagos/domain/internal/api/versioning"
	"nunezlagos/domain/internal/auth/apikey"
	bootstrapsvc "nunezlagos/domain/internal/auth/bootstrap"
	"nunezlagos/domain/internal/auth/session"
	"nunezlagos/domain/internal/cache"
	"nunezlagos/domain/internal/config"
	"nunezlagos/domain/internal/httpserver"
	mcphttpserver "nunezlagos/domain/internal/mcp/httpserver"
	mcptools "nunezlagos/domain/internal/mcp/server"
	"nunezlagos/domain/internal/metrics"
	enrollsvc "nunezlagos/domain/internal/service/enrollment"
	openspecsvc "nunezlagos/domain/internal/service/openspec"
	"nunezlagos/domain/internal/tracing"
)

// buildRouter construye el http.Handler completo: API REST + MCP HTTP.
func buildRouter(
	cfg *config.Config,
	serverVersion string,
	pools serverPools,
	s *serverServices,
	metricsReg *metrics.Registry,
	logger *slog.Logger,
	queryCacheLRU *cache.LRU,
) http.Handler {
	mux := http.NewServeMux()

	info := httpserver.VersionInfo{Version: serverVersion, Commit: Commit, BuildTime: BuildTime}
	healthH := &httpserver.HealthHandler{Info: info, StartedAt: time.Now()}
	mux.Handle("/health", healthH)
	mux.Handle("/healthz", healthH)
	mux.Handle("/health/ready", &httpserver.ReadyHandler{Pool: pools.App})

	versionCatalog := versioning.NewCatalog("v1",
		versioning.Version{Slug: "v1", State: versioning.StateActive})
	mux.HandleFunc("/api/version", versionCatalog.VersionInfoHandler)

	corsMW := middleware.NewCORS(cfg.CORSOrigins, logger)
	if !corsMW.Enabled() {
		logger.Info("CORS not configured; set DOMAIN_CORS_ORIGINS to enable cross-origin requests")
	} else {
		logger.Info("CORS enabled", slog.Int("origins_count", len(cfg.CORSOrigins)))
	}

	requestLogMW := middleware.RequestLog(logger)
	cachedResolver := apikey.NewCachedResolver(s.APIKeyStore, 5*time.Minute)

	sessionResolver := func(ctx context.Context, plain string) (*apikey.Principal, func(context.Context) context.Context, error) {
		active, err := s.SessionSvc.Resolve(ctx, plain)
		if err != nil {
			return nil, nil, err
		}
		p := &apikey.Principal{
			UserID:         active.UserID.String(),
			OrganizationID: active.OrganizationID.String(),
		}
		attacher := func(c context.Context) context.Context { return session.ToContext(c, active) }
		return p, attacher, nil
	}

	authMW := &apikey.Middleware{
		Resolver:        cachedResolver,
		Allowlist:       handler.AuthAllowlist(),
		Pool:            pools.App,
		SessionResolver: sessionResolver,
	}
	rateLimitMW := &middleware.RateLimitMiddleware{Limiter: s.RateLimiter, KeyFunc: middleware.DefaultKeyFunc}
	auditMW := middleware.AuditMiddleware

	activityMW := &activity.HTTPMiddleware{
		Recorder: s.ActivityStore,
		Logger:   logger,
		Principal: func(r *http.Request) (uuid.UUID, *uuid.UUID, bool) {
			p, ok := apikey.FromContext(r.Context())
			if !ok || p == nil {
				return uuid.Nil, nil, false
			}
			orgID, err := uuid.Parse(p.OrganizationID)
			if err != nil {
				return uuid.Nil, nil, false
			}
			var actor *uuid.UUID
			if uid, err := uuid.Parse(p.UserID); err == nil {
				actor = &uid
			}
			return orgID, actor, true
		},
	}

	api := &handler.API{
		APIKeys:            s.APIKeyStore,
		AuthSessionService: s.SessionSvc,
		Bootstrap:          bootstrapsvc.New(pools.App),
		Enrollment:         &enrollsvc.Service{Pool: pools.App, Audit: s.Recorder},
		WebhookService:     s.InboundWebhookService,
		Dispatcher:         s.Dispatcher,
		Feedback:           s.FeedbackService,
		FeedbackLimiter:    s.FeedbackLimiter,
		SkillMetrics:       s.SkillMetricsService,
		Skills:             s.SkillService,
		SkillSuggestions:   s.SkillSuggestionsSvc,
		SkillJudge:         s.SkillJudgeAggregator,
		SkillABTest:        s.SkillABTestService,
		Projects:           s.ProjectService,
		Openspec: &openspecsvc.Engine{
			IssuesR: s.HUService,
			IssuesW: s.HUService,
			SpecR:   s.SpecService,
			SpecW:   s.SpecService,
			TasksR:  s.TaskService,
			TasksW:  s.TaskService,
			Pool:    pools.App,
		},
	}

	mux.Handle("/api/", corsMW.Wrap(
		versionCatalog.Middleware(
			requestLogMW(
				authMW.Wrap(
					middleware.PrincipalCtx(
						rateLimitMW.Wrap(
							auditMW(activityMW.Wrap(api.Router())))))))))

	mcpBuilder := &mcphttpserver.Builder{
		Base: mcptools.Deps{
			Observations:     s.ObsService,
			ObservationEdges: s.ObsEdgeService,
			CodeGraph:        s.CodeGraphService,
			Projects:         s.ProjectService,
			Prompts:          s.PromptService,
			Timeline:         s.TimelineService,
			Search:           s.SearchService,
			Knowledge:        s.KnowledgeService,
			Skills:           s.SkillService,
			SkillExecution:   s.SkillExecService,
			Agents:          s.AgentService,
			AgentRunner:     s.AgentRunnerInst,
			Crons:           s.CronService,
			Clients:         s.ClientService,
			CapturedPrompts: s.CapturedPromptService,
			ProjectRepos:    s.ProjectRepoService,
			ProjectPolicies: s.ProjectPolicyService,
			Tickets:         s.TicketService,
			Policies:        s.PolicyService,
			Flows:           s.FlowService,
			FlowRunner:      s.FlowRunnerInst,
			Hubuilder:       s.IssuebuilderSvc,
			IssueSvc:        s.HUService,
			Spec:            s.SpecService,
			Tasks:           s.TaskService,
			Intake:          s.IntakeSvc,
			Orchestrator:    s.OrchestratorSvc,
			PromptRouter:    s.PromptRouterSvc,
			WorkflowImport:  s.WorkflowImportSvc,
			Pool:            pools.App,
			Dispatcher:      s.Dispatcher,
			ServerName:      "domain-mcp-http",
			ServerVer:       serverVersion,
			SharedCache:     queryCacheLRU,
			MetricsOnToolCall: func(tool, status string, dur float64) {
				metricsReg.MCPToolCallsTotal.WithLabelValues(tool, status).Inc()
				if status != "cache_hit" {
					metricsReg.MCPToolDuration.WithLabelValues(tool).Observe(dur)
				}
			},
			MetricsOnCacheHit:  func() { metricsReg.MCPCacheHitsTotal.Inc() },
			MetricsOnCacheMiss: func() { metricsReg.MCPCacheMissesTotal.Inc() },
		},
	}
	mcpHTTPHandler := mcphttpserver.NewHandler(mcpBuilder, cachedResolver)
	mux.Handle("/mcp", mcpHTTPHandler)
	mux.Handle("/mcp/", mcpHTTPHandler)
	logger.Info("MCP HTTP transport mounted",
		slog.String("path", "/mcp"),
		slog.String("auth", "Bearer api_key"))

	finalHandler := httpserver.RecoverMiddleware(logger)(
		metricsReg.HTTPMiddleware(
			tracing.HTTPMiddleware("domain")(mux),
		),
	)
	return finalHandler
}
