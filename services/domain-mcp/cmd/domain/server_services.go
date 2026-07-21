package main

// buildServices es intencionalmente largo: es el grafo de inyección de
// dependencias del servidor. No hay lógica de negocio aquí, solo wiring.

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/activity"
	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/auth/ratelimit"
	"nunezlagos/domain/internal/auth/session"
	"nunezlagos/domain/internal/config"
	"nunezlagos/domain/internal/crypto"
	"nunezlagos/domain/internal/dbstats"
	"nunezlagos/domain/internal/dispatch"
	"nunezlagos/domain/internal/events"
	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/llm/anthropic"
	"nunezlagos/domain/internal/llm/circuitbreaker"
	"nunezlagos/domain/internal/llm/google"
	"nunezlagos/domain/internal/llm/ollama"
	llmopenai "nunezlagos/domain/internal/llm/openai"
	llmratelimit "nunezlagos/domain/internal/llm/ratelimit"
	llmregistry "nunezlagos/domain/internal/llm/registry"
	llmretry "nunezlagos/domain/internal/llm/retry"
	smtpmail "nunezlagos/domain/internal/mail/smtp"
	"nunezlagos/domain/internal/metrics"
	agentrunner "nunezlagos/domain/internal/runner/agent"
	flowrunner "nunezlagos/domain/internal/runner/flow"
	skillrunner "nunezlagos/domain/internal/runner/skill"
	"nunezlagos/domain/internal/secrets"
	agentsvc "nunezlagos/domain/internal/service/agent"
	attSvc "nunezlagos/domain/internal/service/attachment"
	"nunezlagos/domain/internal/service/billing"
	capturedpromptsvc "nunezlagos/domain/internal/service/capturedprompt"
	clientsvc "nunezlagos/domain/internal/service/client"
	codegraphsvc "nunezlagos/domain/internal/service/codegraph"
	"nunezlagos/domain/internal/service/cost"
	cronsvc "nunezlagos/domain/internal/service/cron"
	feedbacksvc "nunezlagos/domain/internal/service/feedback"
	"nunezlagos/domain/internal/service/flow"
	intakesvc "nunezlagos/domain/internal/service/intake"
	usvc "nunezlagos/domain/internal/service/issue"
	"nunezlagos/domain/internal/service/issuebuilder"
	"nunezlagos/domain/internal/service/knowledge"
	"nunezlagos/domain/internal/service/lifecycle"
	"nunezlagos/domain/internal/service/mcpserver"
	"nunezlagos/domain/internal/service/observation"
	"nunezlagos/domain/internal/service/orchestrator"
	analysissvc "nunezlagos/domain/internal/service/orchestrator/analysis"
	"nunezlagos/domain/internal/service/orchestrator/phases"
	"nunezlagos/domain/internal/service/outboundwebhook"
	"nunezlagos/domain/internal/service/policy"
	projsvc "nunezlagos/domain/internal/service/project"
	projectpolicysvc "nunezlagos/domain/internal/service/projectpolicy"
	projectreposvc "nunezlagos/domain/internal/service/projectrepo"
	"nunezlagos/domain/internal/service/projecttemplate"
	promptsvc "nunezlagos/domain/internal/service/prompt"
	"nunezlagos/domain/internal/service/promptrouter"
	reqsvc "nunezlagos/domain/internal/service/requirement"
	searchsvc "nunezlagos/domain/internal/service/search"
	skillsvc "nunezlagos/domain/internal/service/skill"
	"nunezlagos/domain/internal/service/skill/skilldb"
	skillabtestsvc "nunezlagos/domain/internal/service/skill_ab_test"
	skillmetricssvc "nunezlagos/domain/internal/service/skill_metrics"
	skillsuggestionssvc "nunezlagos/domain/internal/service/skill_suggestions"
	specsvc "nunezlagos/domain/internal/service/spec"
	tsvc "nunezlagos/domain/internal/service/task"
	ticketsvc "nunezlagos/domain/internal/service/ticket"
	timelinesvc "nunezlagos/domain/internal/service/timeline"
	tracesvc "nunezlagos/domain/internal/service/traceability"
	"nunezlagos/domain/internal/service/usagealerts"
	webhooksvc "nunezlagos/domain/internal/service/webhook"
	wp "nunezlagos/domain/internal/service/wizardplan"
	wpsources "nunezlagos/domain/internal/service/wizardplan/sources"
	"nunezlagos/domain/internal/service/workflowimport"
	s3client "nunezlagos/domain/internal/storage/s3"
	"nunezlagos/domain/internal/store/txctx"

	"github.com/jackc/pgx/v5/pgxpool"
)

// serverServices agrupa todos los servicios construidos por buildServices.
type serverServices struct {
	Recorder               *audit.PGRecorder
	ClientService          *clientsvc.Service
	CapturedPromptService  *capturedpromptsvc.Service
	ProjectRepoService     *projectreposvc.Service
	ProjectPolicyService   *projectpolicysvc.Service
	TicketService          *ticketsvc.Service
	SessionSvc             *session.Service
	EventBus               *events.Bus
	ProjectService         *projsvc.Service
	ObsService             *observation.Service
	ObsEdgeService         *observation.EdgeService
	CodeGraphService       *codegraphsvc.CodegraphService
	PromptService          *promptsvc.Service
	TimelineService        *timelinesvc.Service
	SearchService          *searchsvc.Service
	KnowledgeService       *knowledge.Service
	LifecycleService       *lifecycle.Service
	FlowService            *flow.Service
	SkillService           *skillsvc.Service
	AgentService           *agentsvc.Service
	BillingService         *billing.Service
	CostService            *cost.Service
	OutboundWebhookService *outboundwebhook.Service
	InboundWebhookService  *webhooksvc.Service
	OutboundDispatcher     *outboundwebhook.Dispatcher
	OutboundRequireTLS     bool
	LLMFactory             *llm.Factory
	SkillRunnerInst        *skillrunner.Runner
	SkillExecService       *skillsvc.ExecutionService
	ModelRegistry          *llmregistry.Registry
	UsageAlertsService     *usagealerts.Service
	MCPServerService       *mcpserver.Service
	ProjectTemplateService *projecttemplate.Service
	PolicyService          *policy.Service
	IssuebuilderSvc        *issuebuilder.Service
	IssuebuilderAdaptive   *issuebuilder.AdaptiveService
	IntakeSvc              *intakesvc.Service
	OrchestratorSvc        *orchestrator.Service
	AnalysisSvc            *analysissvc.Service
	PromptRouterSvc        *promptrouter.Router
	WorkflowImportSvc      *workflowimport.Service
	DBStatsService         *dbstats.Service
	OutboundEmitter        *outboundwebhook.RunnerEmitter
	AgentRunnerInst        *agentrunner.Runner
	FlowRunnerInst         *flowrunner.Runner
	Dispatcher             *dispatch.Dispatcher
	CronService            *cronsvc.Service
	APIKeyStore            *apikey.PGStore
	ActivityStore          *activity.PGStore
	SecretsStore           *secrets.PGStore
	RateLimiter            *ratelimit.Limiter
	RequirementService     *reqsvc.Service
	HUService              *usvc.Service
	SpecService            *specsvc.Service
	TaskService            *tsvc.Service
	TraceService           *tracesvc.Service
	AttachmentService      *attSvc.Service
	MasterCipher           *crypto.Cipher
	Embedder               llm.Embedder
	FeedbackService        *feedbacksvc.Service
	FeedbackLimiter        *ratelimit.Limiter
	SkillMetricsService    *skillmetricssvc.Service
	SkillMetricsAggregator *skillmetricssvc.Aggregator
	SkillSuggestionsSvc    *skillsuggestionssvc.Service
	SkillJudgeAggregator   *skillsuggestionssvc.Aggregator
	SkillABTestService     *skillabtestsvc.Service
	SkillABTestRouter      *skillabtestsvc.Router
}

// buildServices construye los ~40 servicios del servidor a partir de los pools
// y la configuración. Las dependencias cruzadas entre servicios (wiring) se
// resuelven en wireServices().
//
//nolint:cyclop,gocognit // DI graph: alta cardinalidad, sin lógica de negocio
func buildServices(
	ctx context.Context,
	cfg *config.Config,
	pools serverPools,
	logger *slog.Logger,
	metricsReg *metrics.Registry,
) (*serverServices, error) {
	s := &serverServices{}

	s.Recorder = &audit.PGRecorder{Pool: pools.Auth}
	s.Embedder = chooseEmbedder(logger)

	s.ClientService = clientsvc.NewService(pools.App, s.Recorder, nil)
	s.CapturedPromptService = capturedpromptsvc.NewService(capturedpromptsvc.NewPgRepository(pools.App))
	// HU-52.1 — feedback loop. Limiter dedicado 30/min por user_email (anti-spam):
	// capacity=30, refill=30/60s = 0.5 tokens/s.
	s.FeedbackService = feedbacksvc.NewService(feedbacksvc.NewPgRepository(pools.App))
	s.FeedbackLimiter = ratelimit.New(30, 30.0/60.0)
	// HU-52.2 — skill success rate tracking. Service (lectura) + Aggregator
	// (crons) comparten el mismo Repository sobre pools.App.
	{
		smRepo := skillmetricssvc.NewPgRepository(pools.App)
		s.SkillMetricsService = skillmetricssvc.NewService(smRepo)
		s.SkillMetricsAggregator = skillmetricssvc.NewAggregator(smRepo)
	}
	// HU-52.4 — A/B testing de prompts. Service (ciclo de vida + analyzer) y
	// Router (enrutamiento determinista request-time) comparten el Repository.
	// El Router loguea la asignacion de variante en audit (slug, version, user_id),
	// NUNCA el input. Single-tenant: el pin del ganador usa skills.pinned_version,
	// JAMAS organization_id.
	{
		abRepo := skillabtestsvc.NewPgRepository(pools.App)
		s.SkillABTestService = skillabtestsvc.NewService(abRepo)
		s.SkillABTestRouter = skillabtestsvc.NewRouter(abRepo, s.Recorder, logger)
	}
	s.ProjectRepoService = projectreposvc.NewService(projectreposvc.NewPgRepository(pools.App))
	s.ProjectPolicyService = projectpolicysvc.NewService(projectpolicysvc.NewPgRepository(pools.App))
	s.TicketService = ticketsvc.NewService(ticketsvc.NewPgRepository(pools.App))
	s.SessionSvc = session.New(pools.Auth)

	s.EventBus = events.NewBus()
	s.TicketService.SetEventSink(func(topic string, t *ticketsvc.Ticket, actor uuid.UUID, payload map[string]any) {
		if t == nil {
			return
		}
		var actorPtr *uuid.UUID
		if actor != uuid.Nil {
			a := actor
			actorPtr = &a
		}
		tid := t.ID
		s.EventBus.Publish(events.Event{
			OrgID:    uuid.Nil,
			Topic:    topic,
			TicketID: &tid,
			ActorID:  actorPtr,
			Payload:  payload,
		})
	})

	s.ProjectService = projsvc.NewService(pools.App, s.Recorder, nil, nil).
		WithClientService(s.ClientService)
	s.ObsService = observation.NewService(pools.App, s.Recorder, s.Embedder, nil, nil)
	s.ObsEdgeService = observation.NewEdgeService(pools.App, s.Embedder, s.Recorder)
	s.CodeGraphService = codegraphsvc.NewCodegraphService(pools.App)

	s.PromptService = &promptsvc.Service{Pool: pools.App, Audit: s.Recorder}
	s.TimelineService = &timelinesvc.Service{Pool: pools.App}
	s.SearchService = &searchsvc.Service{Pool: pools.App}
	s.KnowledgeService = &knowledge.Service{Pool: pools.App, Audit: s.Recorder, Embedder: s.Embedder}
	s.LifecycleService = &lifecycle.Service{Pool: pools.App, Audit: s.Recorder}
	s.FlowService = flow.NewService(pools.App, s.Recorder, nil)
	s.SkillService = &skillsvc.Service{Pool: pools.App, Audit: s.Recorder, Embedder: s.Embedder}
	s.AgentService = agentsvc.NewService(pools.App, s.Recorder, nil)
	s.BillingService = &billing.Service{Pool: pools.App}
	s.CostService = &cost.Service{Pool: pools.App}

	if mk := os.Getenv("DOMAIN_MASTER_KEY"); mk != "" {
		c, err := crypto.LoadFromBase64(mk)
		if err != nil {
			logger.Warn("DOMAIN_MASTER_KEY invalid; outbound webhook secrets will fail",
				slog.String("error", err.Error()))
		} else {
			s.MasterCipher = c
		}
	}

	s.OutboundWebhookService = &outboundwebhook.Service{Pool: pools.App, Cipher: s.MasterCipher}
	if s.MasterCipher != nil {
		s.InboundWebhookService = &webhooksvc.Service{Pool: pools.App, Audit: s.Recorder, Crypto: s.MasterCipher}
	}
	s.OutboundDispatcher = &outboundwebhook.Dispatcher{
		Pool: pools.App, Svc: s.OutboundWebhookService,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		Logger:     logger,
	}
	s.OutboundRequireTLS = os.Getenv("DOMAIN_OUTBOUND_REQUIRE_TLS") == "true"

	s.LLMFactory = buildLLMFactory()
	// Inyectar el Factory en ObsService para el RERANK opcional de mem_search
	// (degrada solo si MiniMax no está registrado; ver observation/rerank.go).
	s.ObsService.LLM = s.LLMFactory

	// HU-52.3 — LLM-as-judge (skill suggestions). El judge resuelve el provider
	// 'minimax' del Factory; degrada solo si no hay MINIMAX_API_KEY. El Service
	// expone Create (cron) / Approve / Reject / Apply (accion humana). El Refiner
	// (genera content en refine/split al aplicar) lo cubre el propio judge.
	{
		judge := &skillsuggestionssvc.LLMJudge{LLM: s.LLMFactory}
		s.SkillSuggestionsSvc = &skillsuggestionssvc.Service{
			Pool:     pools.App,
			Audit:    s.Recorder,
			Refiner:  judge,
			Versions: &skillVersionRecorder{pool: pools.App},
		}
		s.SkillJudgeAggregator = &skillsuggestionssvc.Aggregator{
			Pool:      pools.App,
			Service:   s.SkillSuggestionsSvc,
			Judge:     judge,
			MaxSkills: cfg.SkillJudgeMaxSkills,
			Logger:    logger,
		}
	}
	s.SkillRunnerInst = skillrunner.New()
	s.ModelRegistry = llmregistry.New()

	if cfg.SMTPHost != "" {
		alertEmailSender := usagealerts.NewSMTPEmailSender(smtpmail.New(smtpmail.Config{
			Host: cfg.SMTPHost, Port: cfg.SMTPPort, Auth: cfg.SMTPAuth,
			User: cfg.SMTPUser, Password: cfg.SMTPPassword,
			UseTLS: cfg.SMTPTLS, From: cfg.SMTPFrom,
		}))
		s.UsageAlertsService = &usagealerts.Service{
			Pool:        pools.App,
			EmailSender: alertEmailSender,
			Logger:      logger,
		}
	} else {
		s.UsageAlertsService = &usagealerts.Service{Pool: pools.App, Logger: logger}
	}

	s.MCPServerService = &mcpserver.Service{Pool: pools.App, Cipher: s.MasterCipher, Logger: logger}
	s.ProjectTemplateService = &projecttemplate.Service{Pool: pools.App}
	s.PolicyService = &policy.Service{Pool: pools.App}
	s.IssuebuilderSvc = &issuebuilder.Service{Pool: pools.App, Audit: s.Recorder, DraftTTLHrs: 24}
	s.IntakeSvc = &intakesvc.Service{Pool: pools.App, Audit: s.Recorder}

	s.OrchestratorSvc = buildOrchestrator(pools, s, logger, cfg)
	s.AnalysisSvc = &analysissvc.Service{
		Pool: pools.App, Audit: s.Recorder, LLM: s.LLMFactory,
		Knowledge: s.KnowledgeService, Observation: s.ObsService,
		PromptLoader: func(ctx context.Context) (string, error) {
			p, err := s.PromptService.GetActive(ctx, uuid.Nil, nil, "analysis")
			if err != nil {
				return "", err
			}
			return p.Body, nil
		},
	}

	promptClassifier := buildPromptClassifier(s, logger)
	wizardAnalyzer, wizardPlanner := buildWizardPlan(pools, s, promptClassifier)
	s.IssuebuilderAdaptive = &issuebuilder.AdaptiveService{
		Service:  s.IssuebuilderSvc,
		Analyzer: wizardAnalyzer,
		Planner:  wizardPlanner,
	}
	s.PromptRouterSvc = &promptrouter.Router{
		IntakeService:       s.IntakeSvc,
		IssueBuilderService: s.IssuebuilderSvc,
		Classifier:          promptClassifier,
		Orchestrator:        s.OrchestratorSvc,
		AnalysisService:     &analysisRunnerAdapter{inner: s.AnalysisSvc},
	}

	s.WorkflowImportSvc = &workflowimport.Service{Pool: pools.App}
	s.DBStatsService = &dbstats.Service{Pool: pools.App}

	s.OutboundEmitter = &outboundwebhook.RunnerEmitter{
		Dispatcher:  s.OutboundDispatcher,
		Logger:      logger,
		UsageAlerts: s.UsageAlertsService.AsUsageAlerter(),
	}

	s.AgentRunnerInst = &agentrunner.Runner{
		Pool: pools.App, Audit: s.Recorder, Factory: s.LLMFactory,
		Agents: s.AgentService, Skills: s.SkillService,
		SkillRunner: s.SkillRunnerInst, Models: s.ModelRegistry,
		Emitter: s.OutboundEmitter, Metrics: metricsReg,
	}
	s.FlowRunnerInst = &flowrunner.Runner{
		Pool: pools.App, Audit: s.Recorder, Flows: s.FlowService,
		Agents: s.AgentService, Skills: s.SkillService, Observations: s.ObsService,
		AgentRunner: s.AgentRunnerInst, SkillRunner: s.SkillRunnerInst,
		Emitter: s.OutboundEmitter, Metrics: metricsReg,
		Signals: &flow.SignalStore{Pool: pools.App},
	}

	// ExecutionService compartido: lo usan el dispatcher (persiste
	// skill_executions con created_by, HU-52.2) y el tool MCP domain_skill_execute.
	s.SkillExecService = &skillsvc.ExecutionService{
		Pool: pools.App, Skills: s.SkillService,
		Versions: &skillsvc.VersionStore{Pool: pools.App},
		Runner:   s.SkillRunnerInst,
	}
	dispatcherAdapters := &dispatch.Adapters{
		FlowRunner:  s.FlowRunnerInst,
		AgentRunner: s.AgentRunnerInst,
		SkillRunner: s.SkillRunnerInst,
		Agents:      s.AgentService,
		Skills:      s.SkillService,
		SkillExec:   s.SkillExecService,
	}
	s.Dispatcher = &dispatch.Dispatcher{
		RunFlow:  dispatcherAdapters.RunFlowForDispatcher(),
		RunAgent: dispatcherAdapters.RunAgentForDispatcher(),
		RunSkill: dispatcherAdapters.RunSkillForDispatcher(),
		Metrics:  &dispatch.PromMetricsRecorder{Reg: metricsReg},
		Logger:   logger,
		SourceValidator: func(src string) bool {
			return src == dispatch.SourceCron || src == dispatch.SourceWebhook ||
				src == dispatch.SourceMCP || src == dispatch.SourceManual
		},
	}

	s.CronService = &cronsvc.Service{Pool: pools.App, Audit: s.Recorder}

	s.APIKeyStore = &apikey.PGStore{Pool: pools.Auth, FieldEncKey: cfg.FieldEncKey}

	s.ActivityStore = &activity.PGStore{Pool: pools.App}
	s.SecretsStore = &secrets.PGStore{Pool: pools.App, Cipher: s.MasterCipher}

	windowDur, _ := time.ParseDuration(cfg.RateLimitWindow)
	refillRate := float64(cfg.RateLimitRequests) / windowDur.Seconds()
	s.RateLimiter = ratelimit.New(cfg.RateLimitRequests, refillRate)
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			s.RateLimiter.Cleanup(10 * time.Minute)
			if s.FeedbackLimiter != nil {
				s.FeedbackLimiter.Cleanup(10 * time.Minute)
			}
		}
	}()

	s.RequirementService = &reqsvc.Service{Pool: pools.App, Audit: s.Recorder}
	s.HUService = &usvc.Service{Pool: pools.App, Audit: s.Recorder}
	s.SpecService = &specsvc.Service{Pool: pools.App, Audit: s.Recorder}
	s.TaskService = &tsvc.Service{Pool: pools.App, Audit: s.Recorder}
	s.TraceService = &tracesvc.Service{Pool: pools.App}

	if cfg.S3Bucket != "" {
		s3c, err := s3client.New(s3client.Config{
			Endpoint: cfg.S3Endpoint,
			Region:   cfg.S3Region,
			Bucket:   cfg.S3Bucket,
			Key:      cfg.S3AccessKey,
			Secret:   cfg.S3SecretKey,
		})
		if err != nil {
			logger.Warn("s3 client init failed; attachments disabled", slog.Any("err", err))
		} else {
			s.AttachmentService = &attSvc.Service{Pool: pools.App, S3: s3c, Audit: s.Recorder}
			logger.Info("s3 storage configured",
				slog.String("bucket", cfg.S3Bucket),
				slog.String("endpoint", cfg.S3Endpoint),
			)
		}
	} else {
		logger.Warn("DOMAIN_S3_BUCKET not set; file attachments disabled")
	}

	return s, nil
}

// buildLLMFactory registra los providers LLM disponibles según env vars.
func buildLLMFactory() *llm.Factory {
	maxConc := 8
	if v := os.Getenv("DOMAIN_LLM_MAX_CONCURRENT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxConc = n
		}
	}
	cbCfg := circuitbreaker.Config{FailureThreshold: 5, RecoveryTimeout: 30 * time.Second}
	wrapLLM := func(p llm.Provider) llm.Provider {
		return circuitbreaker.New(llmratelimit.New(llmretry.New(p, llmretry.Config{}), maxConc), cbCfg)
	}

	factory := llm.NewFactory()
	if k := os.Getenv("DOMAIN_ANTHROPIC_KEY"); k != "" {
		factory.Register("anthropic", wrapLLM(anthropic.New(k)))
	}
	if k := os.Getenv("DOMAIN_OPENAI_KEY"); k != "" {
		factory.Register("openai", wrapLLM(llmopenai.New(k)))
	}
	if k := os.Getenv("DOMAIN_GOOGLE_KEY"); k != "" {
		factory.Register("google", wrapLLM(google.New(k)))
	}
	anthropic.RegisterMiniMax(factory, wrapLLM)
	llmopenai.RegisterOpenAICompat(factory, wrapLLM, slog.Default())
	factory.Register("ollama", wrapLLM(ollama.New()))
	if def := os.Getenv("DOMAIN_LLM_PROVIDER"); def != "" {
		factory.SetDefault(def, def)
	}
	return factory
}

// buildOrchestrator construye el servicio orchestrator con todas sus fases SDD.
func buildOrchestrator(pools serverPools, s *serverServices, logger *slog.Logger, cfg *config.Config) *orchestrator.Service {
	orchPhases := phases.NewRegistry()
	orchPhases.MustRegister(phases.NewSDDExploreHandler())
	orchPhases.MustRegister(phases.NewSDDSpecHandler())
	orchPhases.MustRegister(phases.NewSDDProposeHandler())
	orchPhases.MustRegister(phases.NewSDDDesignHandler())
	orchPhases.MustRegister(phases.NewSDDTasksHandler())
	orchPhases.MustRegister(phases.NewSDDApplyHandler())
	orchPhases.MustRegister(phases.NewSDDVerifyHandler())
	orchPhases.MustRegister(phases.NewSDDJudgeHandler())
	orchPhases.MustRegister(phases.NewSDD4RHandler())
	orchPhases.MustRegister(phases.NewSDDReviewHandler())
	orchPhases.MustRegister(phases.NewSDDArchiveHandler())
	orchPhases.MustRegister(phases.NewSDDOnboardHandler())

	svc := orchestrator.New(pools.App, s.Recorder, orchPhases, cfg.Env)
	svc.LLM = s.LLMFactory
	svc.Skills = s.SkillService
	svc.Spec = s.SpecService
	svc.Tasks = s.TaskService
	svc.IssueSvc = s.HUService
	// REQ-54 issue-54.2: servicios read-only para preparar contexto server-side.
	svc.ProjectPolicies = s.ProjectPolicyService
	svc.Observations = s.ObsService
	return svc
}

// buildPromptClassifier elige clasificador LLM (anthropic) o heurístico según disponibilidad.
func buildPromptClassifier(s *serverServices, logger *slog.Logger) promptrouter.Classifier {
	anthropicProv, _ := s.LLMFactory.Get("anthropic")
	if anthropicProv == nil {
		logger.Info("prompt classifier: heurístico (no anthropic provider)")
		return promptrouter.HeuristicClassifier{}
	}
	logger.Info("prompt classifier: LLM anthropic con fallback heurístico")
	return &promptrouter.LLMClassifier{
		Provider: anthropicProv,
		Model:    "claude-haiku-4-5-20251001",
		Fallback: promptrouter.HeuristicClassifier{},
		PromptLoader: func(ctx context.Context) (string, error) {
			p, err := s.PromptService.GetActive(ctx, uuid.Nil, nil, "triage")
			if err != nil {
				return "", err
			}
			return p.Body, nil
		},
	}
}

// buildWizardPlan construye el analyzer y planner del wizard de issues.
// Recibe el promptClassifier ya construido para compartir la instancia.
func buildWizardPlan(pools serverPools, s *serverServices, promptClassifier promptrouter.Classifier) (*wp.Analyzer, *wp.Planner) {
	classifier := &promptrouter.WizardplanAdapter{Inner: promptClassifier}
	analyzer := &wp.Analyzer{
		Classifier: classifier,
		Sources: []wp.Source{
			&wpsources.IssueDedupSource{Pool: pools.App, Limit: 5},
			&wpsources.CodebaseSource{ProjectRoot: ".", MaxHits: 10},
			&wpsources.MemorySource{Search: s.SearchService, Limit: 5},
		},
		Timeout: 10 * time.Second,
	}

	planner := &wp.Planner{}
	if anthropicProv, _ := s.LLMFactory.Get("anthropic"); anthropicProv != nil {
		planner.QuestionFormulator = &wp.LLMQuestionFormulator{
			Provider: anthropicProv,
			Model:    "claude-haiku-4-5-20251001",
			PromptLoader: func(ctx context.Context) (string, error) {
				p, err := s.PromptService.GetActive(ctx, uuid.Nil, nil, "wizard-formulator")
				if err != nil {
					return "", err
				}
				return p.Body, nil
			},
		}
	}
	return analyzer, planner
}

// skillVersionRecorder adapta skill_versions a skill_suggestions.VersionRecorder.
// HONRA la tx-context: cuando el Apply de un REFINE lo invoca dentro de su
// transaccion (via txctx), el snapshot de la version queda atomico con el UPDATE
// del content (no abre una tx propia, a diferencia de skill.VersionStore.Create).
type skillVersionRecorder struct {
	pool *pgxpool.Pool
}

func (r *skillVersionRecorder) q(ctx context.Context) *skilldb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return skilldb.New(tx)
	}
	return skilldb.New(r.pool)
}

// RecordVersion crea un snapshot inmutable (version = MAX+1) y devuelve el numero.
func (r *skillVersionRecorder) RecordVersion(ctx context.Context, skillID uuid.UUID, content *string, changelog *string, createdBy *uuid.UUID) (int, error) {
	q := r.q(ctx)
	next, err := q.VersionMaxVersion(ctx, skillID)
	if err != nil {
		return 0, err
	}
	v, err := q.VersionCreate(ctx, skilldb.VersionCreateParams{
		SkillID:   skillID,
		Version:   next,
		Content:   content,
		Changelog: changelog,
		CreatedBy: createdBy,
	})
	if err != nil {
		return 0, err
	}
	return int(v.Version), nil
}
