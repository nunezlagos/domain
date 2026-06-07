// Package main es el entrypoint del binario `domain`: CLI principal + servidor HTTP.
//
// HU-01.1 db-schema-migrations: subcomandos `migrate up|down|version`.
// HU-01.3 health-version: subcomando `version` y `server`.
// HU-14.1 cli-core-commands: estructura base; subcomandos restantes en Fase 2+.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"nunezlagos/domain/internal/api/handler"
	"nunezlagos/domain/internal/api/middleware"
	"nunezlagos/domain/internal/audit"
	clicommands "nunezlagos/domain/internal/cli/commands"
	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/auth/otp"
	"nunezlagos/domain/internal/config"
	"nunezlagos/domain/internal/db"
	"nunezlagos/domain/internal/httpserver"
	"nunezlagos/domain/internal/llm"
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
	"nunezlagos/domain/internal/service/flow"
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

	// Pools (app_user para runtime, app_admin para auth/audit).
	ctx := context.Background()
	pools, err := db.OpenProduction(ctx, cfg.DatabaseURL, cfg.DatabaseAuthURL)
	if err != nil {
		logger.Error("pools open failed", slog.Any("err", err))
		os.Exit(1)
	}
	defer pools.Close()
	if cfg.DatabaseAuthURL == "" && cfg.Env != "dev" {
		logger.Warn("DOMAIN_DATABASE_AUTH_URL not set — auth pool reuses runtime user (NOT recommended outside dev)")
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

	// LLM factory: registra providers basado en env vars DOMAIN_LLM_*.
	// Si no hay ninguna key, el runner devuelve runner_disabled al primer Run.
	llmFactory := llm.NewFactory()
	if k := os.Getenv("DOMAIN_ANTHROPIC_KEY"); k != "" {
		llmFactory.Register("anthropic", anthropic.New(k))
	}
	if k := os.Getenv("DOMAIN_OPENAI_KEY"); k != "" {
		llmFactory.Register("openai", llmopenai.New(k))
	}
	if k := os.Getenv("DOMAIN_OLLAMA_HOST"); k != "" || true {
		p := ollama.New()
		if k != "" {
			p.BaseURL = k
		}
		llmFactory.Register("ollama", p)
	}
	if def := os.Getenv("DOMAIN_LLM_PROVIDER"); def != "" {
		llmFactory.SetDefault(def, def)
	}

	skillRunnerInst := skillrunner.New()
	modelRegistry := &llmregistry.Registry{Pool: pools.App}
	agentRunnerInst := &agentrunner.Runner{
		Pool: pools.App, Audit: recorder, Factory: llmFactory,
		Agents: agentService, Skills: skillService, Billing: billingService,
		SkillRunner: skillRunnerInst, Models: modelRegistry,
	}
	flowRunnerInst := &flowrunner.Runner{
		Pool: pools.App, Audit: recorder, Flows: flowService,
		Agents: agentService, Skills: skillService, Observations: obsService,
		AgentRunner: agentRunnerInst, SkillRunner: skillRunnerInst,
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
		go flowRunnerInst.RunRecovery(leaderCtx, flowrunner.RecoveryConfig{
			StaleAfter: 5 * time.Minute, PollInterval: 60 * time.Second,
		})
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
		OTPService:     otpService,
		APIKeys:        apiKeyStore,
	}

	addr := fmt.Sprintf("%s:%d", cfg.HTTPBind, cfg.HTTPPort)
	mux := http.NewServeMux()
	info := httpserver.VersionInfo{Version: Version, Commit: Commit, BuildTime: BuildTime}
	mux.Handle("/health", &httpserver.HealthHandler{Info: info, StartedAt: time.Now()})
	mux.Handle("/health/ready", &httpserver.ReadyHandler{Pool: pools.App})

	// API REST montada bajo /api/v1/*.
	// Middleware order: auth → idempotency → handler.
	authMW := &apikey.Middleware{Resolver: apiKeyStore, Allowlist: handler.AuthAllowlist()}
	idempMW := &middleware.Idempotency{Pool: pools.App}
	mux.Handle("/api/", authMW.Wrap(idempMW.Wrap(api.Router())))

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

	// Graceful shutdown (HU-26.4): trap SIGINT/SIGTERM, drain in-flight
	// requests con timeout 30s. K8s envía SIGTERM antes del kill -9 (default 30s).
	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-shutdownCh
		logger.Info("shutdown signal received, draining...", slog.String("signal", sig.String()))
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("forced shutdown after timeout", slog.Any("err", err))
		} else {
			logger.Info("graceful shutdown complete")
		}
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
