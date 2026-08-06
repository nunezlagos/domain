package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
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
	"nunezlagos/domain/internal/observability"
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
) (http.Handler, *observability.InvocationLogger, *observability.HTTPLogger, *observability.ResourceCollector, *observability.FnLogger, *observability.Tracker) {
	invocationLogger := observability.NewInvocationLogger(
		&observability.PGInvocationStore{Pool: pools.App},
		logger,
		0, 0,
	)
	httpLogger := observability.NewHTTPLogger(
		&observability.PGHTTPLogStore{Pool: pools.App},
		logger,
		0,
	)
	resourceCollector := observability.NewResourceCollector(
		&observability.PGResourceStore{Pool: pools.App},
		logger,
		0,
	)
	resourceCollector.Start()
	fnLogger := observability.NewFnLogger(
		&observability.PGFnLogStore{Pool: pools.App},
		logger,
		0,
	)
	workflowTracker := observability.NewTracker(
		&observability.PGWorkflowStore{Pool: pools.App},
		logger,
		0, 0,
	)
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

	// El camino firmado va contra el store directo, no contra cachedResolver: el
	// lookup es por prefix + firma, no por la key en claro, así que no hay clave
	// de caché estable (DOMAINSERV-129).
	authMW := &apikey.Middleware{
		Resolver:         cachedResolver,
		Allowlist:        handler.AuthAllowlist(),
		Pool:             pools.App,
		SessionResolver:  sessionResolver,
		FailureLogger:    s.SessionSvc,
		SignedResolver:   s.APIKeyStore,
		NonceBurner:      s.APIKeyStore,
		RequireSignature: cfg.HMACRequireSignature,
	}
	logger.Info("API auth schemes mounted",
		slog.String("path", "/api/"),
		slog.String("schemes", "Bearer + "+apikey.HMACScheme),
		slog.Bool("signature_required", cfg.HMACRequireSignature))
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

	// DOMAINSERV-240: sin esto API.WebhookDispatcher quedaba en nil y cada entrega
	// arrancaba una goroutine suelta por request, sin cota ni backpressure.
	s.WebhookShutdown = api.StartWebhookDispatcher(handler.WebhookDispatcherConfig{
		QueueSize:  256,
		JobTimeout: 30 * time.Second,
		Logger:     logger,
	})

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
			Agents:           s.AgentService,
			AgentRunner:      s.AgentRunnerInst,
			Crons:            s.CronService,
			InboundWebhooks:  s.InboundWebhookService,
			Clients:          s.ClientService,
			CapturedPrompts:  s.CapturedPromptService,
			ProjectRepos:     s.ProjectRepoService,
			ProjectPolicies:  s.ProjectPolicyService,
			Tickets:          s.TicketService,
			Attachments:      s.AttachmentService,
			Policies:         s.PolicyService,
			Flows:            s.FlowService,
			FlowToken:        s.FlowTokenSvc,
			FlowRunner:       s.FlowRunnerInst,
			Hubuilder:        s.IssuebuilderSvc,
			IssueSvc:         s.HUService,
			Spec:             s.SpecService,
			Tasks:            s.TaskService,
			Intake:           s.IntakeSvc,
			Orchestrator:     s.OrchestratorSvc,
			PromptRouter:     s.PromptRouterSvc,
			WorkflowImport:   s.WorkflowImportSvc,
			Pool:             pools.App,
			Dispatcher:       s.Dispatcher,
			ServerName:       "domain-mcp-http",
			ServerVer:        serverVersion,
			SharedCache:      queryCacheLRU,
			MetricsOnToolCall: func(ctx context.Context, tool, status, errCode, errMsg string, dur float64) {
				metricsReg.MCPToolCallsTotal.WithLabelValues(tool, status).Inc()
				if status != "cache_hit" {
					metricsReg.MCPToolDuration.WithLabelValues(tool).Observe(dur)
				}
				observability.LogToolInvocation(ctx, logger, invocationLogger, workflowTracker,
					observability.ToolCall{
						Tool:         tool,
						Status:       status,
						ErrorCode:    errCode,
						ErrorMessage: errMsg,
						DurationMS:   int64(dur * 1000),
					})
			},
			MetricsOnCacheHit:  func() { metricsReg.MCPCacheHitsTotal.Inc() },
			MetricsOnCacheMiss: func() { metricsReg.MCPCacheMissesTotal.Inc() },
		},
	}
	// Token del mcpServer ACP nativo (mismo que usa buildACPNative): el handler
	// lo reconoce para acotar fail-closed ese bearer sin depender del header.
	//
	// FUERA DE ALCANCE de DOMAINSERV-129: /mcp NO pasa por authMW, tiene su
	// propio Bearer en internal/mcp/httpserver/handler.go. La firma HMAC hoy
	// cubre solo /api/; portarla al transporte MCP es otro ticket.
	mcpHTTPHandler := mcphttpserver.NewHandler(mcpBuilder, cachedResolver, os.Getenv("DOMAIN_ACP_MCP_TOKEN"))
	mux.Handle("/mcp", mcpHTTPHandler)
	mux.Handle("/mcp/", mcpHTTPHandler)
	logger.Info("MCP HTTP transport mounted",
		slog.String("path", "/mcp"),
		slog.String("auth", "Bearer api_key"))

	finalHandler := httpserver.RecoverMiddleware(logger)(
		metricsReg.HTTPMiddleware(
			tracing.HTTPMiddleware("domain")(
				httpLogger.Middleware(mux),
			),
		),
	)
	return finalHandler, invocationLogger, httpLogger, resourceCollector, fnLogger, workflowTracker
}
