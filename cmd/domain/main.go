// Package main es el entrypoint del binario `domain`: CLI principal + servidor HTTP.
//
// HU-01.1 db-schema-migrations: subcomandos `migrate up|down|version`.
// HU-01.3 health-version: subcomando `version` y `server`.
// HU-14.1 cli-core-commands: estructura base; subcomandos restantes en Fase 2+.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"nunezlagos/domain/internal/api/backpressure"
	"nunezlagos/domain/internal/api/handler"
	"nunezlagos/domain/internal/dbmon"
	"nunezlagos/domain/internal/api/middleware"
	"nunezlagos/domain/internal/api/versioning"
	"nunezlagos/domain/internal/audit"
	clicommands "nunezlagos/domain/internal/cli/commands"
	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/crypto"
	debugpkg "nunezlagos/domain/internal/debug"
	"nunezlagos/domain/internal/auth/otp"
	"nunezlagos/domain/internal/config"
	"nunezlagos/domain/internal/db"
	"nunezlagos/domain/internal/httpserver"
	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/llm/circuitbreaker"
	"nunezlagos/domain/internal/runtimeconfig"
	"nunezlagos/domain/internal/secrets"
	setuppkg "nunezlagos/domain/internal/cli/setup"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"nunezlagos/domain/internal/logging"
	"nunezlagos/domain/internal/metrics"
	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/llm/anthropic"
	"nunezlagos/domain/internal/llm/ollama"
	llmopenai "nunezlagos/domain/internal/llm/openai"
	llmregistry "nunezlagos/domain/internal/llm/registry"
	smtpmail "nunezlagos/domain/internal/mail/smtp"
	agentrunner "nunezlagos/domain/internal/runner/agent"
	flowrunner "nunezlagos/domain/internal/runner/flow"
	skillrunner "nunezlagos/domain/internal/runner/skill"
	cronsched "nunezlagos/domain/internal/scheduler/cron"
	"nunezlagos/domain/internal/scheduler/leader"
	agentsvc "nunezlagos/domain/internal/service/agent"
	cronsvc "nunezlagos/domain/internal/service/cron"
	"nunezlagos/domain/internal/service/billing"
	"nunezlagos/domain/internal/service/cost"
	"nunezlagos/domain/internal/service/flow"
	"nunezlagos/domain/internal/service/mcpserver"
	"nunezlagos/domain/internal/service/outboundwebhook"
	"nunezlagos/domain/internal/service/usagealerts"
	"nunezlagos/domain/internal/service/invite"
	"nunezlagos/domain/internal/service/knowledge"
	"nunezlagos/domain/internal/service/lifecycle"
	"nunezlagos/domain/internal/service/observation"
	orgsvc "nunezlagos/domain/internal/service/org"
	projsvc "nunezlagos/domain/internal/service/project"
	promptsvc "nunezlagos/domain/internal/service/prompt"
	searchsvc "nunezlagos/domain/internal/service/search"
	sesssvc "nunezlagos/domain/internal/service/session"
	skillsvc "nunezlagos/domain/internal/service/skill"
	timelinesvc "nunezlagos/domain/internal/service/timeline"
)

// Variables sobrescritas por `-ldflags "-X main.Version=..."` (HU-19.2).
var (
	Version   = "0.0.0-dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Printf("domain %s\ncommit: %s\nbuilt: %s\n", Version, Commit, BuildTime)
	case "help", "--help", "-h":
		printUsage()
	case "migrate":
		runMigrate(os.Args[2:])
	case "server":
		runServer()
	case "healthcheck":
		runHealthcheckProbe()
	case "rotate-db-password":
		runRotateDBPassword(os.Args[2:])
	case "setup":
		runSetup(os.Args[2:])
	case "projects", "observations", "obs", "agents", "flows", "skills", "search", "context", "completion":
		// Delegar a CLI commands (REQ-14)
		os.Exit(clicommands.Dispatch(os.Args[1:]))
	default:
		fmt.Fprintf(os.Stderr, "comando no implementado: %s\n", os.Args[1])
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Println(`domain — plataforma de memoria y orquestación para agentes AI

Uso:
  domain <comando> [args]

Server:
  server              Inicia servidor HTTP + scheduler
  healthcheck         Probe interno para Dockerfile HEALTHCHECK

Migrations:
  migrate up          Aplica migraciones pendientes
  migrate down [N]    Rollback N migraciones (default 1)
  migrate version     Schema version + dirty flag

CLI cliente (requiere DOMAIN_API_KEY):
  projects ls|get|create
  observations ls|save  (alias: obs)
  agents ls|get|run
  flows ls|run
  skills ls
  search <query>
  context [--project <slug>]
  completion bash|zsh|fish

Common:
  version             Version + commit + build time
  help                Esta ayuda

Env:
  DOMAIN_API_KEY      requerido para CLI cliente
  DOMAIN_BASE_URL     default http://localhost:8000
  DOMAIN_DATABASE_URL requerido para server/migrate`)
}

func runMigrate(args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "uso: domain migrate <up|down [N]|version>")
		os.Exit(2)
	}
	switch args[0] {
	case "up":
		if err := dmigrate.Up(cfg.DatabaseURL); err != nil {
			fmt.Fprintf(os.Stderr, "migrate up: %v\n", err)
			os.Exit(1)
		}
		v, dirty, _ := dmigrate.Version(cfg.DatabaseURL)
		fmt.Printf("migrations applied. current version: %d (dirty=%v)\n", v, dirty)
	case "down":
		steps := 1
		if len(args) > 1 {
			n, err := strconv.Atoi(args[1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "invalid N: %v\n", err)
				os.Exit(2)
			}
			steps = n
		}
		if err := dmigrate.Down(cfg.DatabaseURL, steps); err != nil {
			fmt.Fprintf(os.Stderr, "migrate down: %v\n", err)
			os.Exit(1)
		}
		v, dirty, _ := dmigrate.Version(cfg.DatabaseURL)
		fmt.Printf("migrations rolled back. current version: %d (dirty=%v)\n", v, dirty)
	case "version":
		v, dirty, err := dmigrate.Version(cfg.DatabaseURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "migrate version: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("schema version: %d (dirty=%v)\n", v, dirty)
	default:
		fmt.Fprintf(os.Stderr, "unknown migrate subcommand: %s\n", args[0])
		os.Exit(2)
	}
}

func runServer() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	logger := logging.Setup(logging.Config{
		Level:     cfg.LogLevel,
		Format:    cfg.LogFormat,
		Output:    cfg.LogOutput,
		AddSource: cfg.LogAddSource,
	})
	// Métricas Prometheus (HU-17.1)
	metricsReg := metrics.New()
	if cfg.MetricsEnabled {
		metricsAddr := fmt.Sprintf("%s:%d", cfg.MetricsBind, cfg.MetricsPort)
		go func() {
			logger.Info("metrics endpoint starting", slog.String("addr", metricsAddr))
			if err := metricsReg.Serve(metricsAddr, "", ""); err != nil && err != http.ErrServerClosed {
				logger.Error("metrics server failed", slog.Any("err", err))
			}
		}()
	}

	// Runtime tuning (HU-27.2)
	debugpkg.TuneRuntime(logger)

	// Debug pprof endpoints (HU-27.1) en puerto separado con basic auth
	if os.Getenv("DOMAIN_DEBUG_ENABLED") == "true" {
		port, _ := strconv.Atoi(os.Getenv("DOMAIN_DEBUG_PORT"))
		go func() {
			err := debugpkg.Serve(debugpkg.Config{
				Enabled:  true,
				Bind:     os.Getenv("DOMAIN_DEBUG_BIND"),
				Port:     port,
				AuthUser: os.Getenv("DOMAIN_DEBUG_AUTH_USER"),
				AuthPass: os.Getenv("DOMAIN_DEBUG_AUTH_PASSWORD"),
			}, logger)
			if err != nil && err != http.ErrServerClosed {
				logger.Error("debug server failed", slog.Any("err", err))
			}
		}()
	}

	// Pools (app_user para runtime, app_admin para auth/audit).
	ctx := context.Background()
	pools, err := db.OpenProductionWithReplica(ctx, cfg.DatabaseURL, cfg.DatabaseAuthURL, cfg.DatabaseReadOnlyURL)
	if err != nil {
		logger.Error("pools open failed", slog.Any("err", err))
		os.Exit(1)
	}
	defer pools.Close()
	if cfg.DatabaseAuthURL == "" && cfg.Env != "dev" {
		logger.Warn("DOMAIN_DATABASE_AUTH_URL not set — auth pool reuses runtime user (NOT recommended outside dev)")
	}
	if pools.ReadOnly != nil {
		pools.LagMonitor = &db.LagMonitor{
			Pool: pools.ReadOnly, PollInterval: 30 * time.Second,
			ThresholdSecs: 10.0, Logger: logger,
		}
		go pools.LagMonitor.Run(ctx)
		logger.Info("read replica configured with lag monitor",
			slog.Float64("threshold_secs", 10.0))
	}

	// Services: dependency wiring explícito.
	recorder := &audit.PGRecorder{Pool: pools.Auth}
	orgService := &orgsvc.Service{Pool: pools.App, Audit: recorder}
	projectService := &projsvc.Service{Pool: pools.App, Audit: recorder}
	obsService := &observation.Service{Pool: pools.App, Audit: recorder, Embedder: llm.NopEmbedder{}}
	// Mailer real si DOMAIN_SMTP_HOST configurado, sino Nop
	var inviteMailer invite.Mailer = invite.NopMailer{}
	var otpMailer otp.Mailer
	if cfg.SMTPHost != "" {
		realMailer := smtpmail.New(smtpmail.Config{
			Host: cfg.SMTPHost, Port: cfg.SMTPPort, Auth: cfg.SMTPAuth,
			User: cfg.SMTPUser, Password: cfg.SMTPPassword,
			UseTLS: cfg.SMTPTLS, From: cfg.SMTPFrom,
		})
		inviteMailer = realMailer
		otpMailer = realMailer
		logger.Info("SMTP mailer configured", slog.String("host", cfg.SMTPHost))
	} else {
		logger.Warn("SMTP not configured — invitations/OTP no enviarán mails reales (DOMAIN_SMTP_HOST missing)")
	}
	_ = otpMailer

	inviteService := &invite.Service{
		Pool: pools.App, Audit: recorder, Mailer: inviteMailer,
		AcceptURL: "https://app.domain.sh/accept",
	}
	sessionService := &sesssvc.Service{Pool: pools.App, Audit: recorder}
	promptService := &promptsvc.Service{Pool: pools.App, Audit: recorder}
	timelineService := &timelinesvc.Service{Pool: pools.App}
	searchService := &searchsvc.Service{Pool: pools.App}
	knowledgeService := &knowledge.Service{Pool: pools.App, Audit: recorder, Embedder: llm.NopEmbedder{}}
	lifecycleService := &lifecycle.Service{Pool: pools.App, Audit: recorder}
	flowService := &flow.Service{Pool: pools.App, Audit: recorder}
	skillService := &skillsvc.Service{Pool: pools.App, Audit: recorder, Embedder: llm.NopEmbedder{}}
	agentService := &agentsvc.Service{Pool: pools.App, Audit: recorder}
	billingService := &billing.Service{Pool: pools.App}
	costService := &cost.Service{Pool: pools.App}

	// Runtime config registry (HU-27.3) — refresca al boot + cada 30s + SIGHUP.
	rtCfgRegistry := &runtimeconfig.Registry{Pool: pools.App, Logger: logger}
	if err := rtCfgRegistry.Refresh(ctx); err != nil {
		logger.Warn("initial runtime config refresh failed (defaults used)",
			slog.String("error", err.Error()))
	}
	go rtCfgRegistry.RunPolling(ctx, 30*time.Second)

	// Cipher opcional para outbound webhook secrets at-rest (HU-02.3 + HU-10.4).
	var masterCipher *crypto.Cipher
	if mk := os.Getenv("DOMAIN_MASTER_KEY"); mk != "" {
		c, err := crypto.LoadFromBase64(mk)
		if err != nil {
			logger.Warn("DOMAIN_MASTER_KEY invalid; outbound webhook secrets will fail",
				slog.String("error", err.Error()))
		} else {
			masterCipher = c
		}
	}
	outboundWebhookService := &outboundwebhook.Service{Pool: pools.App, Cipher: masterCipher}
	outboundDispatcher := &outboundwebhook.Dispatcher{
		Pool: pools.App, Svc: outboundWebhookService,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		Logger:     logger,
	}
	outboundRequireTLS := os.Getenv("DOMAIN_OUTBOUND_REQUIRE_TLS") == "true"

	// LLM factory: registra providers basado en env vars DOMAIN_LLM_*.
	// Cada provider se envuelve con circuit breaker (HU-26.5) para shed-load
	// cuando hay errores sostenidos del provider externo.
	llmFactory := llm.NewFactory()
	cbCfg := circuitbreaker.Config{
		FailureThreshold: 5,
		RecoveryTimeout:  30 * time.Second,
	}
	if k := os.Getenv("DOMAIN_ANTHROPIC_KEY"); k != "" {
		llmFactory.Register("anthropic", circuitbreaker.New(anthropic.New(k), cbCfg))
	}
	if k := os.Getenv("DOMAIN_OPENAI_KEY"); k != "" {
		llmFactory.Register("openai", circuitbreaker.New(llmopenai.New(k), cbCfg))
	}
	if k := os.Getenv("DOMAIN_OLLAMA_HOST"); k != "" || true {
		p := ollama.New()
		if k != "" {
			p.BaseURL = k
		}
		llmFactory.Register("ollama", circuitbreaker.New(p, cbCfg))
	}
	if def := os.Getenv("DOMAIN_LLM_PROVIDER"); def != "" {
		llmFactory.SetDefault(def, def)
	}

	skillRunnerInst := skillrunner.New()
	modelRegistry := &llmregistry.Registry{Pool: pools.App}
	usageAlertsService := &usagealerts.Service{Pool: pools.App}
	mcpServerService := &mcpserver.Service{Pool: pools.App, Cipher: masterCipher, Logger: logger}

	outboundEmitter := &outboundwebhook.RunnerEmitter{
		Dispatcher:  outboundDispatcher,
		Logger:      logger,
		UsageAlerts: usageAlertsService.AsUsageAlerter(),
	}
	agentRunnerInst := &agentrunner.Runner{
		Pool: pools.App, Audit: recorder, Factory: llmFactory,
		Agents: agentService, Skills: skillService, Billing: billingService,
		SkillRunner: skillRunnerInst, Models: modelRegistry,
		Emitter: outboundEmitter,
	}
	flowRunnerInst := &flowrunner.Runner{
		Pool: pools.App, Audit: recorder, Flows: flowService,
		Agents: agentService, Skills: skillService, Observations: obsService,
		AgentRunner: agentRunnerInst, SkillRunner: skillRunnerInst,
		Emitter: outboundEmitter,
	}

	// Cron scheduler (HU-10.1): solo corre en el pod leader (HU-26.2)
	cronService := &cronsvc.Service{Pool: pools.App, Audit: recorder}
	scheduler := &cronsched.Scheduler{
		Crons: cronService, Agents: agentRunnerInst, Flows: flowRunnerInst,
		SkillRunner: skillRunnerInst, Skills: skillService,
		Audit: recorder, Logger: logger,
	}
	leaderElection := &leader.Election{
		Pool: pools.App, LockKey: leader.LockKeyCronScheduler,
		PollPeriod: 10 * time.Second, Logger: logger,
	}
	schedCtx, schedCancel := context.WithCancel(context.Background())
	go leaderElection.RunAsLeader(schedCtx, func(leaderCtx context.Context) {
		// El pod leader corre todos los workers single-instance:
		// - cron scheduler (HU-10.1)
		// - flow recovery (HU-09.6) marca stale flow_runs como failed
		// - outbound webhook dispatcher (HU-10.4) procesa cola de deliveries
		go flowRunnerInst.RunRecovery(leaderCtx, flowrunner.RecoveryConfig{
			StaleAfter: 5 * time.Minute, PollInterval: 60 * time.Second,
		})
		go runOutboundDispatcher(leaderCtx, outboundDispatcher, logger)
		scheduler.Run(leaderCtx)
	})
	defer schedCancel()
	apiKeyStore := &apikey.PGStore{Pool: pools.Auth}
	otpService := &otp.Service{
		Pool: pools.Auth, // Request/Verify cruzan org_id (lookup users por email)
		Mail: otpMailer,
	}

	api := &handler.API{
		OrgService:     orgService,
		ProjectService: projectService,
		ObsService:     obsService,
		InviteService:  inviteService,
		SessionService:  sessionService,
		PromptService:   promptService,
		TimelineService:  timelineService,
		SearchService:    searchService,
		KnowledgeService: knowledgeService,
		LifecycleService: lifecycleService,
		SkillService:     skillService,
		AgentService:     agentService,
		AgentRunner:      agentRunnerInst,
		FlowService:      flowService,
		FlowRunner:       flowRunnerInst,
		CostService:      costService,
		BillingService:   billingService,
		OutboundWebhookService:    outboundWebhookService,
		OutboundWebhookDispatcher: outboundDispatcher,
		OutboundWebhookRequireTLS: outboundRequireTLS,
		Backpressure:              &backpressure.Limiter{Pool: pools.App},
		DBMonCollector:            &dbmon.Collector{Pool: pools.App},
		UsageAlertsService:        usageAlertsService,
		MCPServerService:          mcpServerService,
		OTPService:     otpService,
		APIKeys:        apiKeyStore,
	}

	addr := fmt.Sprintf("%s:%d", cfg.HTTPBind, cfg.HTTPPort)
	mux := http.NewServeMux()
	info := httpserver.VersionInfo{Version: Version, Commit: Commit, BuildTime: BuildTime}
	mux.Handle("/health", &httpserver.HealthHandler{Info: info, StartedAt: time.Now()})
	mux.Handle("/health/ready", &httpserver.ReadyHandler{Pool: pools.App})

	// Versioning catalog (HU-13.8). Por ahora solo v1 active.
	versionCatalog := versioning.NewCatalog("v1",
		versioning.Version{Slug: "v1", State: versioning.StateActive})
	mux.HandleFunc("/api/version", versionCatalog.VersionInfoHandler)

	// API REST montada bajo /api/v1/*.
	// Middleware order: versioning → auth → idempotency → handler.
	authMW := &apikey.Middleware{Resolver: apiKeyStore, Allowlist: handler.AuthAllowlist()}
	idempMW := &middleware.Idempotency{Pool: pools.App}
	mux.Handle("/api/", versionCatalog.Middleware(authMW.Wrap(idempMW.Wrap(api.Router()))))

	// Aplica metrics middleware al mux principal (todos los handlers se cuentan)
	handler := metricsReg.HTTPMiddleware(mux)

	logger.Info("domain server starting",
		slog.String("version", Version),
		slog.String("addr", addr),
		slog.String("env", cfg.Env),
		slog.Bool("metrics_enabled", cfg.MetricsEnabled),
	)
	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  time.Duration(cfg.HTTPReadTimeoutSeconds) * time.Second,
		WriteTimeout: time.Duration(cfg.HTTPWriteTimeoutSeconds) * time.Second,
	}

	// Graceful shutdown (HU-26.4): trap SIGINT/SIGTERM, drain sequenced
	// con budget total ~28s (K8s terminationGracePeriodSeconds=30 default).
	// HU-27.3 hot-reload SIGHUP handler.
	hupCh := make(chan os.Signal, 1)
	signal.Notify(hupCh, syscall.SIGHUP)
	go func() {
		for range hupCh {
			if err := rtCfgRegistry.Refresh(context.Background()); err != nil {
				logger.Warn("SIGHUP refresh failed", slog.Any("err", err))
			} else {
				logger.Info("config reloaded via SIGHUP")
			}
		}
	}()

	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-shutdownCh
		shutdownStart := time.Now()
		logger.Info("shutdown signal received", slog.String("signal", sig.String()))

		// Paso 1: flip readiness → ELB deja de rutear nuevos requests (5s grace)
		httpserver.ShuttingDown.Store(true)
		logger.Info("readiness flipped → unhealthy; waiting ELB drain (5s)")
		time.Sleep(5 * time.Second)

		// Paso 2: HTTP server Shutdown (espera in-flight) — budget 20s
		httpCtx, httpCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer httpCancel()
		forced := false
		if err := srv.Shutdown(httpCtx); err != nil {
			logger.Warn("http shutdown forced after timeout", slog.Any("err", err))
			forced = true
		}

		// Paso 3: cancel leader workers (scheduler + recovery + dispatcher)
		schedCancel()

		// Paso 4: cerrar pools — defer pools.Close() lo hace al return de runServer
		duration := time.Since(shutdownStart).Seconds()
		logger.Info("graceful shutdown complete",
			slog.Float64("duration_s", duration),
			slog.Bool("forced", forced))
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server failed", slog.Any("err", err))
		os.Exit(1)
	}
}

func runHealthcheckProbe() {
	cfg, err := config.Load()
	if err != nil {
		os.Exit(1)
	}
	url := fmt.Sprintf("http://%s:%d/health", cfg.HTTPBind, cfg.HTTPPort)
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url) //nolint:gosec // local probe
	if err != nil {
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		os.Exit(1)
	}
	os.Exit(0)
}

// runOutboundDispatcher procesa la cola de deliveries pendientes cada 5s.
// Single-leader (HU-10.4 + HU-26.2): se invoca solo en el pod leader.
func runOutboundDispatcher(ctx context.Context, d *outboundwebhook.Dispatcher, logger *slog.Logger) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := d.ProcessPending(ctx, 50)
			if err != nil && logger != nil {
				logger.Warn("outbound dispatcher pending failed", slog.String("error", err.Error()))
			}
			if n > 0 && logger != nil {
				logger.Debug("outbound dispatcher processed", slog.Int("count", n))
			}
		}
	}
}

// HU-25.10 rotate-db-password — genera nuevo password + ALTER ROLE.
// Usage: domain rotate-db-password --role app_user
// Imprime el nuevo password en stdout (operator lo copia al Secret Manager).
func runRotateDBPassword(args []string) {
	role := "app_user"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--role", "-r":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "missing value for --role")
				os.Exit(2)
			}
			role = args[i+1]
			i++
		case "--help", "-h":
			fmt.Println("Usage: domain rotate-db-password [--role <name>]")
			fmt.Println("  Default role: app_user")
			fmt.Println("  Requires DOMAIN_DATABASE_AUTH_URL (user with ALTER ROLE perm).")
			os.Exit(0)
		}
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	adminDSN := cfg.DatabaseAuthURL
	if adminDSN == "" {
		adminDSN = cfg.DatabaseURL
	}
	ctx := context.Background()
	pool, err := pgxpoolNew(ctx, adminDSN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pool: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()
	rot := &secrets.Rotator{AdminPool: pool}
	newPass, err := rot.RotateRole(ctx, role)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rotate: %v\n", err)
		os.Exit(1)
	}
	// El password va a stdout para pipe-friendly:
	//   domain rotate-db-password --role app_user > /tmp/new_pwd
	fmt.Println(newPass)
	fmt.Fprintf(os.Stderr, "rotated role=%s — update Secret Manager + rolling deploy app pods.\n", role)
}

// pgxpoolNew wrapper para evitar import alias en main.
func pgxpoolNew(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, dsn)
}

// HU-12.5 setup — wizard CLI para configurar agentes externos.
//
// Usage:
//   domain setup claude-code
//   domain setup --mcp-binary /usr/local/bin/domain-mcp --api-key sk_...
func runSetup(args []string) {
	agent := "claude-code"
	mcpBinary := ""
	apiKey := os.Getenv("DOMAIN_API_KEY")
	baseURL := os.Getenv("DOMAIN_BASE_URL")
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "claude-code", "claude":
			agent = "claude-code"
		case "--mcp-binary":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "missing value for --mcp-binary")
				os.Exit(2)
			}
			mcpBinary = args[i+1]
			i++
		case "--api-key":
			if i+1 < len(args) {
				apiKey = args[i+1]
				i++
			}
		case "--base-url":
			if i+1 < len(args) {
				baseURL = args[i+1]
				i++
			}
		case "--help", "-h":
			fmt.Println("Usage: domain setup [claude-code] [--mcp-binary PATH] [--api-key KEY] [--base-url URL]")
			fmt.Println("Configura un agente externo (Claude Desktop) para usar domain-mcp.")
			os.Exit(0)
		}
	}
	if mcpBinary == "" {
		// Default: asumir que domain-mcp está en el mismo dir que domain.
		ex, err := os.Executable()
		if err != nil {
			fmt.Fprintln(os.Stderr, "no pude detectar el binario actual; pasá --mcp-binary")
			os.Exit(1)
		}
		mcpBinary = strings.Replace(ex, "/domain", "/domain-mcp", 1)
	}

	switch agent {
	case "claude-code":
		path, err := setuppkg.SetupClaudeDesktop(mcpBinary, apiKey, baseURL)
		if errors.Is(err, setuppkg.ErrAlreadyConfigured) {
			fmt.Printf("Domain MCP ya configurado en %s — nada que hacer.\n", path)
			os.Exit(0)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "setup falló: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ Domain MCP agregado a %s\n", path)
		if cwd, err := os.Getwd(); err == nil {
			if dp, err := setuppkg.CreateAIDirectives(cwd); err == nil {
				fmt.Printf("✓ Directivas creadas en %s\n", dp)
			}
		}
		fmt.Println("\nReinicia Claude Desktop para activar Domain.")
	default:
		fmt.Fprintf(os.Stderr, "agente no soportado aún: %s\n", agent)
		os.Exit(2)
	}
}
