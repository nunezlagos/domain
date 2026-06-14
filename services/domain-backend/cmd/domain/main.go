// Package main es el entrypoint del binario `domain`: CLI principal + servidor HTTP.
//
// issue-01.1 db-schema-migrations: subcomandos `migrate up|down|version`.
// issue-01.3 health-version: subcomando `version` y `server`.
// issue-14.1 cli-core-commands: estructura base; subcomandos restantes en Fase 2+.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/api/backpressure"
	"nunezlagos/domain/internal/api/handler"
	"nunezlagos/domain/internal/dbmon"
	"nunezlagos/domain/internal/api/middleware"
	"nunezlagos/domain/internal/api/versioning"
	"nunezlagos/domain/internal/activity"
	"nunezlagos/domain/internal/audit"
	clicommands "nunezlagos/domain/internal/cli/commands"
	"nunezlagos/domain/internal/auth/apikey"
	bootstrapsvc "nunezlagos/domain/internal/auth/bootstrap"
	autodetect "nunezlagos/domain/internal/cli/setup/autodetect"
	claudehook "nunezlagos/domain/internal/cli/setup/claudehook"
	propagatepkg "nunezlagos/domain/internal/cli/setup/propagate"
	"nunezlagos/domain/internal/cli/onboard"
	"nunezlagos/domain/internal/auth/rbac"
	"nunezlagos/domain/internal/crypto"
	"nunezlagos/domain/internal/dbstats"
	debugpkg "nunezlagos/domain/internal/debug"
	"nunezlagos/domain/internal/auth/otp"
	"nunezlagos/domain/internal/auth/ratelimit"
	"nunezlagos/domain/internal/config"
	"nunezlagos/domain/internal/db"
	"nunezlagos/domain/internal/dispatch"
	"nunezlagos/domain/internal/httpserver"
	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/llm/circuitbreaker"
	"nunezlagos/domain/internal/runtimeconfig"
	"nunezlagos/domain/internal/secrets"
	"nunezlagos/domain/internal/seeds"
	s3client "nunezlagos/domain/internal/storage/s3"
	"nunezlagos/domain/internal/tracing"
	setuppkg "nunezlagos/domain/internal/cli/setup"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"nunezlagos/domain/internal/logging"
	"nunezlagos/domain/internal/metrics"
	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/llm/anthropic"
	"nunezlagos/domain/internal/llm/google"
	"nunezlagos/domain/internal/llm/ollama"
	llmopenai "nunezlagos/domain/internal/llm/openai"
	llmratelimit "nunezlagos/domain/internal/llm/ratelimit"
	llmregistry "nunezlagos/domain/internal/llm/registry"
	llmretry "nunezlagos/domain/internal/llm/retry"
	smtpmail "nunezlagos/domain/internal/mail/smtp"
	agentrunner "nunezlagos/domain/internal/runner/agent"
	flowrunner "nunezlagos/domain/internal/runner/flow"
	skillrunner "nunezlagos/domain/internal/runner/skill"
	cronsched "nunezlagos/domain/internal/scheduler/cron"
	systemcron "nunezlagos/domain/internal/scheduler/cron/system"
	"nunezlagos/domain/internal/scheduler/leader"
	agentsvc "nunezlagos/domain/internal/service/agent"
	cronsvc "nunezlagos/domain/internal/service/cron"
	"nunezlagos/domain/internal/service/billing"
	"nunezlagos/domain/internal/service/cost"
	"nunezlagos/domain/internal/service/issuebuilder"
	"nunezlagos/domain/internal/service/flow"
	mcphttpserver "nunezlagos/domain/internal/mcp/httpserver"
	mcptools "nunezlagos/domain/internal/mcp/server"
	"nunezlagos/domain/internal/service/mcpserver"
	"nunezlagos/domain/internal/service/outboundwebhook"
	webhooksvc "nunezlagos/domain/internal/service/webhook"
	"nunezlagos/domain/internal/service/policy"
	"nunezlagos/domain/internal/service/projecttemplate"
	enrollsvc "nunezlagos/domain/internal/service/enrollment"
	usagesvc "nunezlagos/domain/internal/service/usage"
	"nunezlagos/domain/internal/service/usagealerts"
	capturedpromptsvc "nunezlagos/domain/internal/service/capturedprompt"
	clientsvc "nunezlagos/domain/internal/service/client"
	projectreposvc "nunezlagos/domain/internal/service/projectrepo"
	"nunezlagos/domain/internal/service/invite"
	"nunezlagos/domain/internal/service/knowledge"
	"nunezlagos/domain/internal/service/lifecycle"
	"nunezlagos/domain/internal/service/observation"
	orgsvc "nunezlagos/domain/internal/service/org"
	projsvc "nunezlagos/domain/internal/service/project"
	promptsvc "nunezlagos/domain/internal/service/prompt"
	reqsvc "nunezlagos/domain/internal/service/requirement"
	rolesvc "nunezlagos/domain/internal/service/role"
	usvc "nunezlagos/domain/internal/service/issue"
	searchsvc "nunezlagos/domain/internal/service/search"
	sesssvc "nunezlagos/domain/internal/service/session"
	specsvc "nunezlagos/domain/internal/service/spec"
	tsvc "nunezlagos/domain/internal/service/task"
	tracesvc "nunezlagos/domain/internal/service/traceability"
	attSvc "nunezlagos/domain/internal/service/attachment"
	intakesvc "nunezlagos/domain/internal/service/intake"
	"nunezlagos/domain/internal/service/promptrouter"
	skillsvc "nunezlagos/domain/internal/service/skill"
	timelinesvc "nunezlagos/domain/internal/service/timeline"
	"nunezlagos/domain/internal/service/workflowimport"
	wp "nunezlagos/domain/internal/service/wizardplan"
	wpsources "nunezlagos/domain/internal/service/wizardplan/sources"
)

// Variables sobrescritas por `-ldflags "-X main.Version=..."` (issue-19.2).
var (
	Version   = "0.0.0-dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func main() {
	// Plug-and-play: cargar config en cascada ANTES de cualquier
	// subcomando, sin pisar env vars ya exportadas. Así `domain server`
	// o `domain projects ls` funcionan desde cualquier directorio sin
	// `source .env` manual — igual que domain-mcp.
	loadEnvCascade()

	if len(os.Args) < 2 {
		// Sin args: detectar proyecto + mostrar capabilities (issue F2).
		// Override: si --tui → bubbletea.
		if isTerminal(os.Stdin) {
			os.Exit(runTUI(nil))
		}
		os.Exit(runDomainDetect(context.Background()))
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
	case "secrets":
		runSecretsCmd(os.Args[2:])
	case "audit":
		runAuditPrune(os.Args[2:])
	case "setup":
		runSetup(os.Args[2:])
	case "init":
		runInit(os.Args[2:])
	case "workflow":
		runWorkflow(os.Args[2:])
	case "install":
		os.Exit(runInstall(os.Args[2:]))
	case "service":
		os.Exit(runService(os.Args[2:]))
	case "update":
		os.Exit(runUpdate(os.Args[2:]))
	case "seed":
		os.Exit(runSeed(os.Args[2:]))
	case "restore":
		os.Exit(runRestore(os.Args[2:]))
	case "onboard", "bootstrap":
		os.Exit(runOnboard(os.Args[2:]))
	case "mcp":
		os.Exit(runMCP(context.Background(), os.Args[2:]))
	case "detect":
		os.Exit(runDomainDetect(context.Background()))
	case "projects", "observations", "obs", "agents", "flows", "skills", "search", "context", "completion", "policies":
		// Delegar a CLI commands (REQ-14)
		os.Exit(clicommands.Dispatch(os.Args[1:]))
	case "tui":
		// Lanza TUI bubbletea con menu (install/update/backups/exit).
		// Tambien accesible como 'domain' (sin args) → printUsage + tui.
		os.Exit(runTUI(os.Args[2:]))
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
  policies import-md|export-md [options]
  completion bash|zsh|fish

Plug-and-play:
  init [--root .] [--dry-run] [--no-stub]
                      Detecta .md de IA (CLAUDE.md, .claude/**, .opencode/**,
                      .cursor/**, .windsurfrules, AGENTS.md...) y los archiva
                      en BD reemplazándolos por stubs que apuntan al MCP.
  workflow list       Lista archivos importados con su status
  workflow restore <rel-path>
                      Restaura el .md original desde el backup en BD
  install [flags]     Wizard idempotente de instalación (deploy mode +
                      migrate + seed + opcional init + setup opencode).
                      Flags: --mode {local|cloud|hybrid}, --base-url,
                      --non-interactive, --no-backup, --no-init, --no-opencode,
                      --dsn
  update [flags]      Backups + migrate + seed (sin tocar configs del agente).
                      Flags: --no-backup, --no-seed, --no-migrate
  seed all            Corre todos los seeders (skip-by-hash, idempotente)
  restore <bak-path>  Restaura un archivo desde un backup timestamped
  mcp list|install|uninstall
                      Catálogo de MCPs instalables (filesystem, fetch,
                      github, git, memory, time) en opencode/claude-code.
  detect              Auto-detect del proyecto en CWD + inventario + sesión.

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
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Hint: corré 'domain install' (genera .env y ~/.config/domain/env),")
		fmt.Fprintln(os.Stderr, "o exportá DOMAIN_DATABASE_URL manualmente.")
		os.Exit(1)
	}
	logger := logging.Setup(logging.Config{
		Level:     cfg.LogLevel,
		Format:    cfg.LogFormat,
		Output:    cfg.LogOutput,
		AddSource: cfg.LogAddSource,
	})
	// Métricas Prometheus (issue-17.1)
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

	// Runtime tuning (issue-27.2)
	debugpkg.TuneRuntime(logger)

	// Pools (app_user para runtime, app_admin para auth/audit).
	ctx := context.Background()
	pools, err := db.OpenProductionWithReplica(ctx, cfg.DatabaseURL, cfg.DatabaseAuthURL, cfg.DatabaseReadOnlyURL)
	if err != nil {
		logger.Error("pools open failed", slog.Any("err", err))
		os.Exit(1)
	}
	defer pools.Close()
	metrics.RunPoolStatsReporter(ctx, metricsReg, pools.App, pools.Auth, pools.ReadOnly, logger)
	if cfg.DatabaseAuthURL == "" && cfg.Env != "dev" {
		logger.Warn("DOMAIN_DATABASE_AUTH_URL not set — auth pool reuses runtime user (NOT recommended outside dev)")
	}
	if pools.ReadOnly != nil {
		pools.LagMonitor = &db.LagMonitor{
			Pool: pools.ReadOnly, PollInterval: 30 * time.Second,
			ThresholdSecs: 10.0, Logger: logger,
			MetricsCB: func(lag float64) {
				metricsReg.ReplicationLagSeconds.Set(lag)
				if lag > 10.0 {
					metricsReg.ReplicaFallbackTotal.Inc()
				}
			},
		}
		go pools.LagMonitor.Run(ctx)
		logger.Info("read replica configured with lag monitor",
			slog.Float64("threshold_secs", 10.0))
	} else {
		logger.Info("no read replica configured — all reads go to primary")
	}

	// Services: dependency wiring explícito.
	recorder := &audit.PGRecorder{Pool: pools.Auth}
	orgService := &orgsvc.Service{Pool: pools.App, Audit: recorder}

	// Debug pprof endpoints (issue-27.1) en puerto separado con basic auth
	if os.Getenv("DOMAIN_DEBUG_ENABLED") == "true" {
		port, _ := strconv.Atoi(os.Getenv("DOMAIN_DEBUG_PORT"))
		go func() {
			err := debugpkg.Serve(debugpkg.Config{
				Enabled:        true,
				Bind:           os.Getenv("DOMAIN_DEBUG_BIND"),
				Port:           port,
				AuthUser:       os.Getenv("DOMAIN_DEBUG_AUTH_USER"),
				AuthPass:       os.Getenv("DOMAIN_DEBUG_AUTH_PASSWORD"),
				AuditRecorder:  recorder,
				Metrics:        metricsReg,
			}, logger)
			if err != nil && err != http.ErrServerClosed {
				logger.Error("debug server failed", slog.Any("err", err))
			}
		}()
	}
	// HU-28.1: Service depende de Repository — usamos los constructores nuevos
	// que internamente arman el pgRepository wrappeando pools.App.
	clientService := clientsvc.NewService(pools.App, recorder, nil)
	capturedPromptService := capturedpromptsvc.NewService(capturedpromptsvc.NewPgRepository(pools.App))
	projectRepoService := projectreposvc.NewService(projectreposvc.NewPgRepository(pools.App))
	// REQ-28.2: projectService recibe referencia a ClientService para
	// resolver client_slug → client_id en Create/Update/List.
	projectService := projsvc.NewService(pools.App, recorder, nil, nil).
		WithClientService(clientService)
	obsService := observation.NewService(pools.App, recorder, llm.NopEmbedder{}, nil, nil)
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

	inviteService := &invite.Service{
		Pool: pools.App, Audit: recorder, Mailer: inviteMailer,
		AcceptURL: "https://app.domain.sh/accept",
	}
	sessionService := sesssvc.NewService(pools.App, recorder, nil)
	promptService := &promptsvc.Service{Pool: pools.App, Audit: recorder}
	timelineService := &timelinesvc.Service{Pool: pools.App}
	searchService := &searchsvc.Service{Pool: pools.App}
	knowledgeService := &knowledge.Service{Pool: pools.App, Audit: recorder, Embedder: llm.NopEmbedder{}}
	lifecycleService := &lifecycle.Service{Pool: pools.App, Audit: recorder}
	flowService := flow.NewService(pools.App, recorder, nil)
	skillService := &skillsvc.Service{Pool: pools.App, Audit: recorder, Embedder: llm.NopEmbedder{}}
	agentService := agentsvc.NewService(pools.App, recorder, nil)
	billingService := &billing.Service{Pool: pools.App}
	costService := &cost.Service{Pool: pools.App}

	// Runtime config registry (issue-27.3) — refresca al boot + cada 30s + SIGHUP.
	rtCfgRegistry := &runtimeconfig.Registry{Pool: pools.App, Logger: logger}
	if err := rtCfgRegistry.Refresh(ctx); err != nil {
		logger.Warn("initial runtime config refresh failed (defaults used)",
			slog.String("error", err.Error()))
	}
	go rtCfgRegistry.RunPolling(ctx, 30*time.Second)

	// OpenTelemetry tracing (issue-17.2) — usa sample ratio del runtime config.
	otelShutdown, oTelErr := tracing.Setup(context.Background(), tracing.Config{
		Enabled:      os.Getenv("DOMAIN_OTEL_ENABLED") == "true",
		OTLPEndpoint: envOr("DOMAIN_OTEL_ENDPOINT", "localhost:4317"),
		ServiceName:  "domain",
		Version:      Version,
		Environment:  cfg.Env,
		SampleRatio:  rtCfgRegistry.Current().OTELSampleRatio,
		Insecure:     envOr("DOMAIN_OTEL_INSECURE", "true") == "true",
	})
	if oTelErr != nil {
		logger.Error("tracing setup failed", slog.Any("err", oTelErr))
		os.Exit(1)
	}
	defer otelShutdown(context.Background())

	// Seeders (issue-01.7) — catálogos del sistema: idempotente, solo líder ejecuta.
	seedRegistry := seeds.NewRegistry()
	seedRegistry.Register(&seeds.PlansSeeder{})
	seedRegistry.Register(&seeds.ModelRegistrySeeder{})
	seedRegistry.Register(&seeds.PlatformPoliciesSeeder{})
	seedRegistry.Register(&seeds.ProjectTemplatesSeeder{})
	seedRegistry.Register(&seeds.MCPProvidersSeeder{})
	// Nota: seeds.SkillCatalog y AgentTemplateCatalog son per-org —
	// materializados desde org.Create() via seeds.SeedSkillsForOrg /
	// seeds.SeedAgentTemplatesForOrg (issue-21.1 org-management hook).
	results, seedErr := seedRegistry.RunAll(ctx, pools.App, seeds.Env(cfg.Env))
	if seedErr != nil {
		logger.Error("seed run failed (partial results may apply)", slog.Any("err", seedErr))
	}
	for name, rep := range results {
		logger.Info("seed completed",
			slog.String("seeder", name),
			slog.Int("created", rep.Created),
			slog.Int("updated", rep.Updated),
			slog.Int("skipped", rep.Skipped),
		)
	}

	// Cipher opcional para outbound webhook secrets at-rest (issue-02.3 + issue-10.4).
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
	// Inbound webhooks requieren cipher para el secret at-rest; sin master key
	// el service queda nil y los endpoints responden webhooks_disabled.
	var inboundWebhookService *webhooksvc.Service
	if masterCipher != nil {
		inboundWebhookService = &webhooksvc.Service{Pool: pools.App, Audit: recorder, Crypto: masterCipher}
	}
	outboundDispatcher := &outboundwebhook.Dispatcher{
		Pool: pools.App, Svc: outboundWebhookService,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		Logger:     logger,
	}
	outboundRequireTLS := os.Getenv("DOMAIN_OUTBOUND_REQUIRE_TLS") == "true"

	// LLM factory: registra providers basado en env vars DOMAIN_LLM_*.
	// Stack de resiliencia por provider (issue-06.2 + issue-26.5):
	// circuit breaker( ratelimit( retry( provider ) ) )
	llmFactory := llm.NewFactory()
	cbCfg := circuitbreaker.Config{
		FailureThreshold: 5,
		RecoveryTimeout:  30 * time.Second,
	}
	maxConc := 8
	if v := os.Getenv("DOMAIN_LLM_MAX_CONCURRENT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxConc = n
		}
	}
	wrapLLM := func(p llm.Provider) llm.Provider {
		return circuitbreaker.New(
			llmratelimit.New(llmretry.New(p, llmretry.Config{}), maxConc), cbCfg)
	}
	if k := os.Getenv("DOMAIN_ANTHROPIC_KEY"); k != "" {
		llmFactory.Register("anthropic", wrapLLM(anthropic.New(k)))
	}
	if k := os.Getenv("DOMAIN_OPENAI_KEY"); k != "" {
		llmFactory.Register("openai", wrapLLM(llmopenai.New(k)))
	}
	if k := os.Getenv("DOMAIN_GOOGLE_KEY"); k != "" {
		llmFactory.Register("google", wrapLLM(google.New(k)))
	}
	llmFactory.Register("ollama", wrapLLM(ollama.New()))
	if def := os.Getenv("DOMAIN_LLM_PROVIDER"); def != "" {
		llmFactory.SetDefault(def, def)
	}

	skillRunnerInst := skillrunner.New()
	modelRegistry := &llmregistry.Registry{Pool: pools.App}
	var alertEmailSender usagealerts.EmailSender
	if cfg.SMTPHost != "" {
		alertEmailSender = usagealerts.NewSMTPEmailSender(smtpmail.New(smtpmail.Config{
			Host: cfg.SMTPHost, Port: cfg.SMTPPort, Auth: cfg.SMTPAuth,
			User: cfg.SMTPUser, Password: cfg.SMTPPassword,
			UseTLS: cfg.SMTPTLS, From: cfg.SMTPFrom,
		}))
	}
	usageAlertsService := &usagealerts.Service{
		Pool:        pools.App,
		EmailSender: alertEmailSender,
		Logger:      logger,
	}
	mcpServerService := &mcpserver.Service{Pool: pools.App, Cipher: masterCipher, Logger: logger}
	projectTemplateService := &projecttemplate.Service{Pool: pools.App}
	policyService := &policy.Service{Pool: pools.App}
	// issue-04.7 wizard interactivo. Attachments se inyectan más abajo cuando
	// se construye el S3 client (puede ser nil si DOMAIN_S3_BUCKET no está).
	issuebuilderSvc := &issuebuilder.Service{
		Pool: pools.App, Audit: recorder, DraftTTLHrs: 24,
	}

	// issue-04.8 intake pipeline service.
	intakeSvc := &intakesvc.Service{Pool: pools.App, Audit: recorder}

	// issue-12.7 prompt router + analyzer + classifier.
	// LLM classifier si hay provider configurado; fallback heurístico siempre.
	var promptClassifier promptrouter.Classifier = promptrouter.HeuristicClassifier{}
	if anthropicProv, _ := llmFactory.Get("anthropic"); anthropicProv != nil {
		promptClassifier = &promptrouter.LLMClassifier{
			Provider: anthropicProv,
			Model:    "claude-haiku-4-5-20251001",
			Fallback: promptrouter.HeuristicClassifier{},
		}
		logger.Info("prompt classifier: LLM anthropic con fallback heurístico")
	} else {
		logger.Info("prompt classifier: heurístico (no anthropic provider)")
	}

	wizardAnalyzer := &wp.Analyzer{
		Classifier: &promptrouter.WizardplanAdapter{Inner: promptClassifier},
		Sources: []wp.Source{
			&wpsources.IssueDedupSource{Pool: pools.App, Limit: 5},
			&wpsources.CodebaseSource{ProjectRoot: ".", MaxHits: 10},
			&wpsources.MemorySource{Search: searchService, Limit: 5},
		},
		Timeout: 10 * time.Second,
	}
	wizardPlanner := &wp.Planner{}
	// LLM formulator si hay provider.
	if anthropicProv, _ := llmFactory.Get("anthropic"); anthropicProv != nil {
		wizardPlanner.QuestionFormulator = &wp.LLMQuestionFormulator{
			Provider: anthropicProv,
			Model:    "claude-haiku-4-5-20251001",
		}
	}

	issuebuilderAdaptive := &issuebuilder.AdaptiveService{
		Service:  issuebuilderSvc,
		Analyzer: wizardAnalyzer,
		Planner:  wizardPlanner,
	}

	promptRouterSvc := &promptrouter.Router{
		IntakeService:    intakeSvc,
		IssueBuilderService: issuebuilderSvc,
		Classifier:       promptClassifier,
	}

	// issue-12.7 workflow import (override de .md de IA en repo cliente).
	workflowImportSvc := &workflowimport.Service{Pool: pools.App}

	dbStatsService := &dbstats.Service{Pool: pools.App}

	outboundEmitter := &outboundwebhook.RunnerEmitter{
		Dispatcher:  outboundDispatcher,
		Logger:      logger,
		UsageAlerts: usageAlertsService.AsUsageAlerter(),
	}
	// issue-10.4 ow-002: hooks de entidad (observation.created, invite.created)
	obsService.Events = outboundEmitter
	inviteService.Events = outboundEmitter
	agentRunnerInst := &agentrunner.Runner{
		Pool: pools.App, Audit: recorder, Factory: llmFactory,
		Agents: agentService, Skills: skillService, Billing: billingService,
		SkillRunner: skillRunnerInst, Models: modelRegistry,
		Emitter: outboundEmitter, Metrics: metricsReg,
	}
	flowRunnerInst := &flowrunner.Runner{
		Pool: pools.App, Audit: recorder, Flows: flowService,
		Agents: agentService, Skills: skillService, Observations: obsService,
		AgentRunner: agentRunnerInst, SkillRunner: skillRunnerInst,
		Emitter: outboundEmitter, Metrics: metricsReg,
		Signals: &flow.SignalStore{Pool: pools.App},
		DLQ:     &flow.DLQStore{Pool: pools.App},
	}

	// issue-35.1: unified dispatcher. Se inyecta en cron, webhook
	// handler y (vía cmd/domain-mcp) los tools MCP. Centraliza
	// métricas y audit del dispatch.
	dispatcherAdapters := &dispatch.Adapters{
		FlowRunner:  flowRunnerInst,
		AgentRunner: agentRunnerInst,
		SkillRunner: skillRunnerInst,
		Agents:      agentService,
		Skills:      skillService,
	}
	dispatcher := &dispatch.Dispatcher{
		RunFlow:  dispatcherAdapters.RunFlowForDispatcher(),
		RunAgent: dispatcherAdapters.RunAgentForDispatcher(),
		RunSkill: dispatcherAdapters.RunSkillForDispatcher(),
		Metrics:  &dispatch.PromMetricsRecorder{Reg: metricsReg},
		Logger:   logger,
		SourceValidator: func(s string) bool {
			return s == dispatch.SourceCron || s == dispatch.SourceWebhook ||
				s == dispatch.SourceMCP || s == dispatch.SourceManual
		},
	}

	// Cron scheduler (issue-10.1): solo corre en el pod leader (issue-26.2)
	cronService := &cronsvc.Service{Pool: pools.App, Audit: recorder}
	scheduler := &cronsched.Scheduler{
		Crons: cronService,
		Audit: recorder, Logger: logger, Dispatcher: dispatcher,
	}
	leaderElection := &leader.Election{
		Pool: pools.App, LockKey: leader.LockKeyCronScheduler,
		PollPeriod: 10 * time.Second, Logger: logger,
	}
	schedCtx, schedCancel := context.WithCancel(context.Background())
	go leaderElection.RunAsLeader(schedCtx, func(leaderCtx context.Context) {
		// El pod leader corre todos los workers single-instance:
		// - cron scheduler (issue-10.1)
		// - flow recovery (issue-09.6) marca stale flow_runs como failed
		// - outbound webhook dispatcher (issue-10.4) procesa cola de deliveries
		go flowRunnerInst.RunRecovery(leaderCtx, flowrunner.RecoveryConfig{
			StaleAfter: 5 * time.Minute, PollInterval: 60 * time.Second,
		})
		go runOutboundDispatcher(leaderCtx, outboundDispatcher, logger)
		go runDBStatsAnalyzer(leaderCtx, dbStatsService, metricsReg, logger)
		go runDBMonitor(leaderCtx, pools.App, metricsReg, logger)
		go runSessionAutoClose(leaderCtx, sessionService, logger)
		go runSoftDeletePurge(leaderCtx, lifecycleService, logger)
		go runAuditPruneScheduler(leaderCtx, recorder, logger)
		go runUsageAlertEvaluator(leaderCtx, usageAlertsService, logger)
		// issue-08.11 heartbeat-watcher (detecta flow_run_steps stuck)
		if cfg.HeartbeatWatcherEnabled {
			watcher := &systemcron.HeartbeatWatcher{
				Pool:    pools.App,
				Metrics: metricsReg,
				Timeout: time.Duration(cfg.HeartbeatWatcherTimeoutMinutes) * time.Minute,
				Tick:    time.Duration(cfg.HeartbeatWatcherTickSeconds) * time.Second,
				Logger:  logger,
			}
			go watcher.Start(leaderCtx)
		}
		// issue-09.7 fv-009: archiva flow_versions deprecated >90d sin runs
		go runFlowVersionArchiver(leaderCtx, pools.App, logger)
		// issue-08.12 orphan-runs-audit (cuenta agent_runs bypass del enforcement)
		if cfg.OrphanAuditEnabled {
			auditor := &systemcron.OrphanAuditor{
				Pool:    pools.App,
				Metrics: metricsReg,
				Tick:    24 * time.Hour,
				Batch:   1000,
				Logger:  logger,
			}
			go auditor.Start(leaderCtx)
		}
		scheduler.Run(leaderCtx)
	})
	defer schedCancel()
	apiKeyStore := &apikey.PGStore{Pool: pools.Auth}
	otpUserLookup := &otpUserLookupAdapter{pool: pools.Auth}
	otpService := &otp.Service{
		Pool:  pools.Auth,
		Users: otpUserLookup,
		Mail:  otpMailer,
	}

	activityStore := &activity.PGStore{Pool: pools.App}
	secretsStore := &secrets.PGStore{Pool: pools.App, Cipher: masterCipher}

	// Rate limiter (issue-02.5)
	windowDur, _ := time.ParseDuration(cfg.RateLimitWindow)
	refillRate := float64(cfg.RateLimitRequests) / windowDur.Seconds()
	rateLimiter := ratelimit.New(cfg.RateLimitRequests, refillRate)
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			rateLimiter.Cleanup(10 * time.Minute)
		}
	}()
	// Per-route OTP rate limiter (issue-02.5): 5 reqs/min por (identifier, IP)
	otpRateLimiter := ratelimit.New(5, 5.0/60.0)
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			otpRateLimiter.Cleanup(10 * time.Minute)
		}
	}()

	customResolver := rbac.NewPGResolver(pools.App)
	rbacChecker := &rbac.Checker{CustomResolver: customResolver}
	roleService := &rolesvc.Service{Pool: pools.App, Audit: recorder}
	requirementService := &reqsvc.Service{Pool: pools.App, Audit: recorder}
	huService := &usvc.Service{Pool: pools.App, Audit: recorder}

	specService := &specsvc.Service{Pool: pools.App, Audit: recorder}
	taskService := &tsvc.Service{Pool: pools.App, Audit: recorder}
	traceService := &tracesvc.Service{Pool: pools.App}

	// S3 client (issue-04.6) — opcional; nil si no configurado
	var attachmentService *attSvc.Service
	if cfg.S3Bucket != "" {
		s3Client, err := s3client.New(s3client.Config{
			Endpoint: cfg.S3Endpoint,
			Region:   cfg.S3Region,
			Bucket:   cfg.S3Bucket,
			Key:      cfg.S3AccessKey,
			Secret:   cfg.S3SecretKey,
		})
		if err != nil {
			logger.Warn("s3 client init failed; attachments disabled", slog.Any("err", err))
		} else {
			attachmentService = &attSvc.Service{Pool: pools.App, S3: s3Client, Audit: recorder}
			logger.Info("s3 storage configured",
				slog.String("bucket", cfg.S3Bucket),
				slog.String("endpoint", cfg.S3Endpoint),
			)
		}
	} else {
		logger.Warn("DOMAIN_S3_BUCKET not set; file attachments disabled")
	}

	// issue-04.7 + 04.6: inyectar attachment service al issuebuilder Service ya
	// creado para que AttachToDraft / PromoteAttachmentsToHU funcionen.
	if attachmentService != nil {
		issuebuilderSvc.Attachments = &issuebuilder.AttachmentServiceAdapter{Inner: attachmentService}
	}

	_ = rbacChecker // TODO: wire RequirePermission middleware on per-route basis
	if err := customResolver.StartCacheListener(ctx); err != nil {
		logger.Warn("custom roles cache listener", slog.String("error", err.Error()))
	}

	api := &handler.API{
		OrgService:     orgService,
		ProjectService: projectService,
		ClientService:  clientService,
		ObsService:     obsService,
		InviteService:  inviteService,
		SessionService:  sessionService,
		PromptService:   promptService,
		TimelineService:  timelineService,
		SearchService:    searchService,
		KnowledgeService: knowledgeService,
		LifecycleService: lifecycleService,
		SkillService:     skillService,
		SkillExecution: &skillsvc.ExecutionService{
			Pool: pools.App, Skills: skillService,
			Versions: &skillsvc.VersionStore{Pool: pools.App},
			Runner:   skillRunnerInst,
		},
		AgentService:     agentService,
		AgentRunner:      agentRunnerInst,
		FlowService:      flowService,
		FlowRunner:       flowRunnerInst,
		CronService:      cronService,
		WebhookService:   inboundWebhookService,
		Dispatcher:       dispatcher, // issue-35.1
		CostService:      costService,
		BillingService:   billingService,
		OutboundWebhookService:    outboundWebhookService,
		OutboundWebhookDispatcher: outboundDispatcher,
		OutboundWebhookRequireTLS: outboundRequireTLS,
		Backpressure:              &backpressure.Limiter{Pool: pools.App},
		DBMonCollector:            &dbmon.Collector{Pool: pools.App},
		UsageAlertsService:        usageAlertsService,
		UsageSnapshot:             &usagesvc.Service{Pool: pools.App},
		Enrollment:                &enrollsvc.Service{Pool: pools.App, Audit: recorder},
		MCPServerService:          mcpServerService,
		ProjectTemplateService:    projectTemplateService,
		PolicyService:             policyService,
		RuntimeConfigRegistry:    rtCfgRegistry,
		DBStatsService:           dbStatsService,
		Hubuilder:                issuebuilderSvc,
		Audit:          recorder,
		ActivityRecorder: activityStore,
		ActivityQuerier:  activityStore,
		OTPService:     otpService,
		OTPRateLimiter: otpRateLimiter,
		APIKeys:        apiKeyStore,
		Bootstrap:      bootstrapsvc.New(pools.App),
		SecretsStore:   secretsStore,
		RoleService:    roleService,
		ReqService:     requirementService,
		HUService:      huService,
		SpecService:    specService,
		TaskService:         taskService,
		TraceService:        traceService,
		AttachmentService:   attachmentService,
		// issue-04.7 v2 (wizard adaptive) + issue-04.8 intake + issue-12.7 plug-and-play.
		IssueBuilderAdaptive: issuebuilderAdaptive,
		IntakeService:    intakeSvc,
		PromptRouter:     promptRouterSvc,
		WorkflowImport:   workflowImportSvc,
	}

	addr := fmt.Sprintf("%s:%d", cfg.HTTPBind, cfg.HTTPPort)
	mux := http.NewServeMux()
	info := httpserver.VersionInfo{Version: Version, Commit: Commit, BuildTime: BuildTime}
	mux.Handle("/health", &httpserver.HealthHandler{Info: info, StartedAt: time.Now()})
	mux.Handle("/health/ready", &httpserver.ReadyHandler{Pool: pools.App})

	// Versioning catalog (issue-13.8). Por ahora solo v1 active.
	versionCatalog := versioning.NewCatalog("v1",
		versioning.Version{Slug: "v1", State: versioning.StateActive})
	mux.HandleFunc("/api/version", versionCatalog.VersionInfoHandler)

	// API REST montada bajo /api/v1/*.
	// Middleware order: CORS → versioning → request-log → auth → principal-ctx → rate-limit → audit → activity → idempotency → handler.
	// principal-ctx (HU-28.3) extrae Principal del ctx y reinyecta OrgID/UserID
	// como uuid.UUID via ctxkeys, eliminando el `p, _ := principal(r); uuid.Parse(...)`
	// repetido en cada handler.
	// CORS (issue-32.2): allowlist desde DOMAIN_CORS_ORIGINS.
	corsMW := middleware.NewCORS(cfg.CORSOrigins, logger)
	if !corsMW.Enabled() {
		logger.Info("CORS not configured; set DOMAIN_CORS_ORIGINS to enable cross-origin requests")
	} else {
		logger.Info("CORS enabled", slog.Int("origins_count", len(cfg.CORSOrigins)))
	}
	requestLogMW := middleware.RequestLog(logger)
	cachedResolver := apikey.NewCachedResolver(apiKeyStore, 5*time.Minute)
	authMW := &apikey.Middleware{Resolver: cachedResolver, Allowlist: handler.AuthAllowlist(), Pool: pools.App}
	rateLimitMW := &middleware.RateLimitMiddleware{Limiter: rateLimiter, KeyFunc: middleware.DefaultKeyFunc}
	auditMW := middleware.AuditMiddleware
	idempMW := &middleware.Idempotency{Pool: pools.App}
	// issue-02.6: activity feed automático en mutaciones (post-auth)
	activityMW := &activity.HTTPMiddleware{
		Recorder: activityStore,
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
	mux.Handle("/api/", corsMW.Wrap(
		versionCatalog.Middleware(
			requestLogMW(
				authMW.Wrap(
					middleware.PrincipalCtx(
						rateLimitMW.Wrap(
							auditMW(activityMW.Wrap(idempMW.Wrap(api.Router()))))))))))

	// REQ-31 (issue-31.1 mcp-http-vps-mode): expone las mismas tools MCP
	// que `cmd/domain-mcp` (stdio) sobre HTTP Streamable transport. Clientes
	// MCP remotos (claude-code, Cursor, Cline...) se conectan via
	//   https://<vps>/mcp  con header  Authorization: Bearer <api_key>.
	// El handler valida el token contra cachedResolver (mismo store que
	// /api/) y construye un MCPServer por request con Principal resuelto.
	mcpBuilder := &mcphttpserver.Builder{
		Base: mcptools.Deps{
			Observations:   obsService,
			Projects:       projectService,
			Sessions:       sessionService,
			Prompts:        promptService,
			Timeline:       timelineService,
			Search:         searchService,
			Knowledge:      knowledgeService,
			Skills:         skillService,
			SkillExecution: &skillsvc.ExecutionService{
				Pool: pools.App, Skills: skillService,
				Versions: &skillsvc.VersionStore{Pool: pools.App},
				Runner:   skillRunnerInst,
			},
			Agents:         agentService,
			AgentRunner:    agentRunnerInst,
			Crons:          cronService,
			Clients:        clientService,
			CapturedPrompts: capturedPromptService,
			ProjectRepos:   projectRepoService,
			Policies:       policyService,
			Flows:          flowService,
			FlowRunner:     flowRunnerInst,
			Hubuilder:      issuebuilderSvc,
			Intake:         intakeSvc,
			PromptRouter:   promptRouterSvc,
			WorkflowImport: workflowImportSvc,
			Pool:           pools.App,
			Dispatcher:     dispatcher,
			ServerName:     "domain-mcp-http",
			ServerVer:      Version,
		},
	}
	mcpHTTPHandler := mcphttpserver.NewHandler(mcpBuilder, cachedResolver)
	mux.Handle("/mcp", mcpHTTPHandler)
	mux.Handle("/mcp/", mcpHTTPHandler)
	logger.Info("MCP HTTP transport mounted",
		slog.String("path", "/mcp"),
		slog.String("auth", "Bearer api_key"))

	// Aplica tracing + metrics middleware al mux principal
	handler := metricsReg.HTTPMiddleware(tracing.HTTPMiddleware("domain")(mux))

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

	// Graceful shutdown (issue-26.4): trap SIGINT/SIGTERM, drain sequenced
	// con budget total ~28s (K8s terminationGracePeriodSeconds=30 default).
	// issue-27.3 hot-reload SIGHUP handler.
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

		// Paso 1: flip readiness → ELB deja de rutear nuevos requests.
		// Grace configurable: DOMAIN_SHUTDOWN_GRACE_SECONDS (default 5).
		grace := 5 * time.Second
		if v := os.Getenv("DOMAIN_SHUTDOWN_GRACE_SECONDS"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 25 {
				grace = time.Duration(n) * time.Second
			}
		}
		httpserver.ShuttingDown.Store(true)
		logger.Info("readiness flipped → unhealthy; waiting ELB drain",
			slog.Duration("grace", grace))
		time.Sleep(grace)

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

	// ListenAndServe en goroutine + canal. Si retorna error != nil
	// (y != http.ErrServerClosed), log FATAL explícito + os.Exit(1).
	// Esto evita el caso "proceso vivo pero listener no responde"
	// (issue-29.3, bug detectado 2026-06-12).
	//
	// Helpers testeables: httpserver.ListenAndServeWithFatalLog y
	// httpserver.RunPostBindWatchdog (en internal/httpserver/listen_wrap.go)
	// encapsulan esta lógica. Aquí el patrón inline es equivalente.
	listenErrCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			listenErrCh <- err
		} else {
			listenErrCh <- nil
		}
	}()

	// Watchdog post-bind: después de 2s (deja al server armar
	// el mux + DB pool), 3 intentos de GET /health. Si los 3
	// fallan, el listener está zombie (proceso vivo pero
	// respondiendo 000) — log FATAL + exit 1.
	go func() {
		time.Sleep(2 * time.Second)
		for i := 0; i < 3; i++ {
			if err := httpserver.ProbeHealth(cfg.HTTPPort); err == nil {
				return
			}
			time.Sleep(1 * time.Second)
		}
		logger.Error("FATAL: health-check post-bind failed 3x — listener not responding",
			slog.Int("port", cfg.HTTPPort))
		os.Exit(1)
	}()

	if err := <-listenErrCh; err != nil {
		logger.Error("FATAL: HTTP listener failed", slog.Any("err", err))
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
// Single-leader (issue-10.4 + issue-26.2): se invoca solo en el pod leader.
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

// runDBMonitor corre tickers para métricas de issue-25.12 (locks-vacuum).
// Ticker 30s: connection states + lock waits + dead tuples.
// Solo en el pod leader.
func runDBMonitor(ctx context.Context, app *pgxpool.Pool, reg *metrics.Registry, logger *slog.Logger) {
	stateTicker := time.NewTicker(30 * time.Second)
	defer stateTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-stateTicker.C:
			var active, idle, idleInTx int
			var longestSeconds float64
			err := app.QueryRow(ctx, `
				SELECT
					COUNT(*) FILTER (WHERE state = 'active') AS active,
					COUNT(*) FILTER (WHERE state = 'idle') AS idle,
					COUNT(*) FILTER (WHERE state = 'idle in transaction') AS idle_in_tx,
				COALESCE(MAX(EXTRACT(EPOCH FROM (now() - query_start)))
					FILTER (WHERE state = 'active'), 0)
				FROM pg_stat_activity
				WHERE backend_type = 'client backend'
			`).Scan(&active, &idle, &idleInTx, &longestSeconds)
			if err != nil {
				logger.Warn("dbmon pg_stat_activity query failed", slog.String("error", err.Error()))
			} else {
				reg.DBConnectionsActive.Set(float64(active))
				reg.DBConnectionsIdle.Set(float64(idle))
				reg.DBConnectionsIdleInTransaction.Set(float64(idleInTx))
				reg.DBLongestQuerySeconds.Set(longestSeconds)
			}

			// Lock waits
			lockRows, err := app.Query(ctx, `
				SELECT pg_locks.mode, COALESCE(pg_class.relname, 'unknown')
				FROM pg_locks
				JOIN pg_stat_activity AS sa ON pg_locks.pid = sa.pid
				LEFT JOIN pg_class ON pg_locks.relation = pg_class.oid
				WHERE sa.state = 'active'
				  AND pg_locks.granted = false
			`)
			if err != nil {
				logger.Warn("dbmon pg_locks query failed", slog.String("error", err.Error()))
			} else {
				for lockRows.Next() {
					var mode, tbl string
					if err := lockRows.Scan(&mode, &tbl); err != nil {
						continue
					}
					reg.DBLockWaitsTotal.WithLabelValues(mode, tbl).Inc()
				}
				lockRows.Close()
			}

			// Dead tuples top 20
			tupRows, err := app.Query(ctx, `
				SELECT relname, n_dead_tup
				FROM pg_stat_user_tables
				WHERE n_dead_tup > 0
				ORDER BY n_dead_tup DESC
				LIMIT 20
			`)
			if err != nil {
				logger.Warn("dbmon dead tuples query failed", slog.String("error", err.Error()))
			} else {
				for tupRows.Next() {
					var tbl string
					var dead int64
					if err := tupRows.Scan(&tbl, &dead); err != nil {
						continue
					}
					reg.DBTableDeadTuples.WithLabelValues(tbl).Set(float64(dead))
				}
				tupRows.Close()
			}
			logger.Debug("dbmon tick complete")
		}
	}
}

// runUsageAlertEvaluator evalua métricas agregadas cada 5min (issue-15.3).
func runUsageAlertEvaluator(ctx context.Context, svc *usagealerts.Service, logger *slog.Logger) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fired, err := svc.EvaluateAggregates(ctx)
			if err != nil {
				logger.Warn("usage alert evaluator failed", slog.String("error", err.Error()))
				continue
			}
			if fired > 0 {
				logger.Info("usage alerts fired", slog.Int("count", fired))
			}
		}
	}
}

// runSoftDeletePurge purga rows soft-deleted fuera de retention (issue-23.2).
// otpUserLookupAdapter implementa otp.UserLookup contra la DB.
type otpUserLookupAdapter struct {
	pool *pgxpool.Pool
}

func (a *otpUserLookupAdapter) ByEmail(ctx context.Context, email string) (*otp.User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	var u otp.User
	err := a.pool.QueryRow(ctx,
		`SELECT id, email, COALESCE(rut, '') FROM users WHERE LOWER(email) = $1 AND deleted_at IS NULL LIMIT 1`,
		email,
	).Scan(&u.ID, &u.Email, &u.RUT)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apikey.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (a *otpUserLookupAdapter) ByRUT(ctx context.Context, rut string) (*otp.User, error) {
	rut = strings.TrimSpace(rut)
	var u otp.User
	err := a.pool.QueryRow(ctx,
		`SELECT id, email, rut FROM users WHERE rut = $1 AND deleted_at IS NULL LIMIT 1`,
		rut,
	).Scan(&u.ID, &u.Email, &u.RUT)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apikey.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func runAuditPruneScheduler(ctx context.Context, recorder *audit.PGRecorder, logger *slog.Logger) {
	retention := 90 * 24 * time.Hour
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-retention)
			n, err := recorder.Prune(ctx, cutoff)
			if err != nil {
				logger.Warn("audit prune failed", slog.String("error", err.Error()))
				continue
			}
			if n > 0 {
				logger.Info("pruned old audit logs", slog.Int64("count", n))
			}
		}
	}
}

// runFlowVersionArchiver elimina flow_versions deprecated >90d que ningún
// run referencia (issue-09.7 fv-009). Corre diario en el pod leader.
func runFlowVersionArchiver(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) {
	vs := &flow.VersioningStore{Pool: pool}
	retention := 90 * 24 * time.Hour
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := vs.ArchiveDeprecated(ctx, retention)
			if err != nil {
				logger.Warn("flow version archive failed", slog.String("error", err.Error()))
				continue
			}
			if n > 0 {
				logger.Info("archived deprecated flow versions", slog.Int64("count", n))
			}
		}
	}
}

func runSoftDeletePurge(ctx context.Context, svc *lifecycle.Service, logger *slog.Logger) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := svc.PurgeExpiredSoftDeleted(ctx)
			if err != nil {
				logger.Warn("soft delete purge failed", slog.String("error", err.Error()))
				continue
			}
			if n > 0 {
				logger.Info("purged soft-deleted rows", slog.Int64("count", n))
			}
		}
	}
}

// runSessionAutoClose cierra sesiones inactivas >24h (issue-03.2).
func runSessionAutoClose(ctx context.Context, svc *sesssvc.Service, logger *slog.Logger) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ids, err := svc.CloseInactive(ctx, 24*time.Hour)
			if err != nil {
				logger.Warn("session auto-close failed", slog.String("error", err.Error()))
				continue
			}
			if len(ids) > 0 {
				logger.Info("auto-closed inactive sessions", slog.Int("count", len(ids)))
			}
		}
	}
}

// runDBStatsAnalyzer corre cada 5min: slow queries → métricas + snapshot weekly.
// Solo en el pod leader (issue-26.2 + issue-25.2).
func runDBStatsAnalyzer(ctx context.Context, svc *dbstats.Service, reg *metrics.Registry, logger *slog.Logger) {
	slowTicker := time.NewTicker(5 * time.Minute)
	snapTicker := time.NewTicker(7 * 24 * time.Hour)
	defer slowTicker.Stop()
	defer snapTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-slowTicker.C:
			queries, err := svc.SlowQueries(ctx, 100, 50)
			if err != nil {
				logger.Warn("dbstats slow queries failed", slog.String("error", err.Error()))
				continue
			}
			if len(queries) > 0 && reg != nil {
				reg.SlowQueriesTotal.WithLabelValues("100").Add(float64(len(queries)))
			}
			logger.Debug("dbstats slow queries analyzed", slog.Int("count", len(queries)))
		case <-snapTicker.C:
			snap, err := svc.Snapshot(ctx)
			if err != nil {
				logger.Warn("dbstats snapshot failed", slog.String("error", err.Error()))
				continue
			}
			logger.Info("dbstats weekly snapshot", slog.Int("rows", snap.Inserted))
			if err := svc.Reset(ctx); err != nil {
				logger.Warn("dbstats reset failed", slog.String("error", err.Error()))
			}
		}
	}
}

// runAuditPrune CLI: domain audit prune [--retention N] [--dry-run]
// Borra entradas de audit_log anteriores a N días (default 90).
func runAuditPrune(args []string) {
	dryRun := false
	retentionDays := 90
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "prune":
			continue
		case "--retention":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "missing value for --retention")
				os.Exit(2)
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 1 {
				fmt.Fprintln(os.Stderr, "invalid retention days")
				os.Exit(2)
			}
			retentionDays = n
			i++
		case "--dry-run":
			dryRun = true
		case "--help", "-h":
			fmt.Println("Usage: domain audit prune [--retention N] [--dry-run]")
			fmt.Println("  Default retention: 90 days")
			fmt.Println("  Requires DOMAIN_DATABASE_AUTH_URL")
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", args[i])
			os.Exit(2)
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

	recorder := &audit.PGRecorder{Pool: pool}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	if dryRun {
		// Count only
		var count int64
		err = pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM audit_log WHERE occurred_at < $1`, cutoff).Scan(&count)
		if err != nil {
			fmt.Fprintf(os.Stderr, "count: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("dry-run: %d entries to delete (before %s)\n", count, cutoff.Format("2006-01-02"))
		return
	}

	deleted, err := recorder.Prune(ctx, cutoff)
	if err != nil {
		fmt.Fprintf(os.Stderr, "prune: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("pruned %d audit log entries (before %s)\n", deleted, cutoff.Format("2006-01-02"))
}

// issue-02.3 secrets re-encrypt — re-cifra secrets tras rotar la master key.
// Usage: domain secrets re-encrypt
// Requiere DOMAIN_MASTER_KEYS="1:<b64>,2:<b64>" (keyring multi-versión: la
// más alta cifra, las anteriores solo descifran) o DOMAIN_MASTER_KEY simple.
func runSecretsCmd(args []string) {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Println("Usage: domain secrets re-encrypt")
		fmt.Println("  Re-cifra todos los secrets con la versión current del keyring.")
		fmt.Println("  Env: DOMAIN_MASTER_KEYS=\"1:<b64>,2:<b64>\" (o DOMAIN_MASTER_KEY)")
		fmt.Println("       DOMAIN_DATABASE_AUTH_URL (batch cross-org, BYPASSRLS)")
		os.Exit(0)
	}
	if args[0] != "re-encrypt" {
		fmt.Fprintf(os.Stderr, "subcomando no implementado: secrets %s\n", args[0])
		os.Exit(2)
	}

	var cipherInst *crypto.Cipher
	var err error
	if spec := os.Getenv("DOMAIN_MASTER_KEYS"); spec != "" {
		cipherInst, err = crypto.LoadKeyring(spec)
	} else if mk := os.Getenv("DOMAIN_MASTER_KEY"); mk != "" {
		cipherInst, err = crypto.LoadFromBase64(mk)
	} else {
		fmt.Fprintln(os.Stderr, "DOMAIN_MASTER_KEYS o DOMAIN_MASTER_KEY requerido")
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "keyring: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	dsn := cfg.DatabaseAuthURL
	if dsn == "" {
		dsn = cfg.DatabaseURL
	}
	ctx := context.Background()
	pool, err := pgxpoolNew(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pool: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	store := &secrets.PGStore{Pool: pool, Cipher: cipherInst}
	n, err := store.ReEncryptAll(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "re-encrypt: %v (re-encrypted %d before failure)\n", err, n)
		os.Exit(1)
	}
	fmt.Printf("re-encrypted %d secret(s) to key version %d\n", n, cipherInst.CurrentVersion())
}

// issue-25.10 rotate-db-password — genera nuevo password + ALTER ROLE.
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

// runOnboard ejecuta el wizard first-run (issue-01.9).
// Uso:
//
//	domain onboard [--base-url URL] [--non-interactive] [--no-opencode]
//
// Sin flags corre en modo interactivo: detecta first-run, pide email,
// dispara bootstrap u OTP, guarda credenciales, opcionalmente
// configura opencode.
func runOnboard(args []string) int {
	baseURL := envOr("DOMAIN_BASE_URL", "http://localhost:8000")
	nonInteractive := false
	noOpencode := false
	email := ""
	// keyName y orgName son parametros del wizard, no del CLI por ahora.
	// El wizard usa defaults sensatos: key_name="default", org_name derivado
	// del email domain. Si el user quiere custom, edita el codigo o
	// usamos env vars DOMAIN_KEY_NAME / DOMAIN_ORG_NAME en el futuro.
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--base-url":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "missing value for --base-url")
				return 2
			}
			baseURL = args[i+1]
			i++
		case "--non-interactive", "-y":
			nonInteractive = true
		case "--no-opencode":
			noOpencode = true
		case "--email":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "missing value for --email")
				return 2
			}
			email = args[i+1]
			i++
		case "--help", "-h":
			fmt.Println("Usage: domain onboard [flags]")
			fmt.Println("  --base-url URL      Domain server URL (default http://localhost:8000)")
			fmt.Println("  --non-interactive  Skip prompts (use defaults)")
			fmt.Println("  --no-opencode      Skip opencode config step")
			fmt.Println("  --email EMAIL      Pre-fill email (non-interactive mode)")
			fmt.Println("  --key-name NAME    API key name (default 'default')")
			fmt.Println("  --org-name NAME    Organization name (auto-derived from email domain)")
			return 0
		}
	}

	w := onboard.New(baseURL)
	w.NonInteractive = nonInteractive
	w.NoOpencode = noOpencode
	// Auto-detect domain bin path: assume domain junto a domain-mcp en el mismo dir.
	if ex, err := os.Executable(); err == nil {
		w.DomainBinPath = ex
	}
	if mcp, err := findDomainMCPSibling(w.DomainBinPath); err == nil {
		w.DomainMCPPath = mcp
	}

	// In non-interactive mode, pre-fill email via ask callback.
	if nonInteractive && email != "" {
		// We set the email on the wizard by wrapping SaveCredentials. The
		// ask("Your email", email, true) will be called by Run; the
		// default returns email. We need to pass email through somehow.
		// Simpler: set a custom SaveCredentials that injects email.
		original := w.SaveCredentials
		w.SaveCredentials = func(c *onboard.Credentials) error {
			if c.Email == "" {
				c.Email = email
			}
			return original(c)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := w.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "onboard failed: %v\n", err)
		return 1
	}
	return 0
}

// findDomainMCPSibling busca domain-mcp junto al binario domain. Si
// domain esta en /usr/bin/domain, busca /usr/bin/domain-mcp. Si no
// existe, retorna error (el caller puede pasar --mcp-binary manualmente).
func findDomainMCPSibling(domainPath string) (string, error) {
	dir := filepath.Dir(domainPath)
	base := strings.TrimSuffix(filepath.Base(domainPath), "domain")
	if base != "" && base != filepath.Base(domainPath) {
		// Binary name had a prefix (e.g., "domain-cli"); not our case.
		_ = base
	}
	candidate := filepath.Join(dir, "domain-mcp")
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	return "", fmt.Errorf("domain-mcp not found next to %s; pass --mcp-binary", domainPath)
}

// pgxpoolNew wrapper para evitar import alias en main.
func pgxpoolNew(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, dsn)
}

// envOr retorna env var o default si está vacía.
func envOr(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// issue-12.5 setup — wizard CLI para configurar agentes externos.
//
// Usage:
//   domain setup claude-code
//   domain setup --mcp-binary /usr/local/bin/domain-mcp --api-key sk_...
func runAutoDetect(args []string) {
	projectDir := "."
	quiet := false
	dryRun := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--quiet", "-q":
			quiet = true
		case "--dry-run":
			dryRun = true
		case "--help", "-h":
			fmt.Println("Usage: domain setup auto-detect [path] [--quiet] [--dry-run]")
			fmt.Println("  path          Project directory (default: .)")
			fmt.Println("  --quiet, -q   Suppress non-error output")
			fmt.Println("  --dry-run     Show what would be done without modifying files")
			os.Exit(0)
		default:
			if !strings.HasPrefix(args[i], "-") {
				projectDir = args[i]
			} else {
				fmt.Fprintf(os.Stderr, "unknown flag: %s\n", args[i])
				os.Exit(2)
			}
		}
	}

	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if dryRun {
		actions, err := autodetect.ApplyDryRun(absDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if len(actions) == 0 {
			if !quiet {
				fmt.Println("no changes needed")
			}
			os.Exit(0)
		}
		if !quiet {
			fmt.Printf("would apply %d action(s) to %s:\n", len(actions), absDir)
			for _, a := range actions {
				switch a.Type {
				case autodetect.ActionSymlink:
					fmt.Printf("  symlink %s → %s\n", a.Path, a.Target)
				case autodetect.ActionJSONUpsert:
					fmt.Printf("  upsert %s key=%s\n", a.Path, a.Key)
				case autodetect.ActionOpenCodeGen:
					fmt.Printf("  generate %s (minimal opencode.json)\n", a.Path)
				}
			}
		}
		os.Exit(0)
	}

	actions, err := autodetect.Apply(absDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if len(actions) == 0 {
		if !quiet {
			fmt.Println("no changes needed")
		}
		os.Exit(0)
	}

	if !quiet {
		fmt.Printf("applied %d action(s) to %s:\n", len(actions), absDir)
		for _, a := range actions {
			switch a.Type {
			case autodetect.ActionSymlink:
				fmt.Printf("  linked %s → %s\n", a.Path, a.Target)
			case autodetect.ActionJSONUpsert:
				fmt.Printf("  added domain to %s\n", a.Path)
			case autodetect.ActionOpenCodeGen:
				fmt.Printf("  generated %s (minimal opencode.json)\n", a.Path)
			}
		}
	}
}

func runWrapperSnippet(args []string) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--shell":
			if i+1 < len(args) {
				i++
			}
		case "--help", "-h":
			fmt.Println("Usage: domain setup wrapper-snippet [--shell zsh|bash]")
			fmt.Println("  Prints the shell wrapper snippet for opencode + domain auto-detect.")
			os.Exit(0)
		}
	}
	fmt.Println("# Pegá esto en tu ~/.zshrc (o ~/.bashrc) y reiniciá la shell")
	fmt.Println("# o corré: source ~/.zshrc")
	fmt.Println()
	fmt.Println(setuppkg.GenerateWrapperSnippet())
}

func runClaudeHook(args []string) {
	apply := false
	show := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--apply":
			apply = true
		case "--show":
			show = true
		case "--help", "-h":
			fmt.Println("Usage: domain setup claude-hook [--apply | --show]")
			fmt.Println("  --apply    Install the hook without prompt")
			fmt.Println("  --show     Show diff without installing")
			os.Exit(0)
		}
	}
	if show {
		doc, raw, err := claudehook.ReadSettings()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if claudehook.HasDomainHook(doc) {
			fmt.Println("Claude Code hook already configured.")
			return
		}
		newDoc := claudehook.AddDomainHook(doc)
		newRaw, _ := json.MarshalIndent(newDoc, "", "  ")
		if raw != nil {
			fmt.Printf("before: %s\n", string(raw))
		} else {
			fmt.Println("before: (file does not exist)")
		}
		fmt.Printf("after:  %s\n", string(newRaw))
		return
	}
	action, err := claudehook.InstallClaudeHook(!apply, apply)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	switch action {
	case "already_installed":
		fmt.Println("Claude Code hook ya configurado (skip)")
	case "installed":
		fmt.Println("✓ Claude Code SessionStart hook installed")
	case "skipped":
		fmt.Println("Claude Code hook skipped (non-interactive)")
	case "declined":
		fmt.Println("Claude Code hook declined")
	}
}

func runPropagate(args []string) {
	scanPath := ""
	all := false
	yes := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--scan":
			if i+1 < len(args) {
				i++
				scanPath = args[i]
			}
		case "--all":
			all = true
		case "--yes":
			yes = true
		case "--help", "-h":
			fmt.Println("Usage: domain setup propagate [--scan <path>] [--all] [--yes]")
			fmt.Println("  --scan <path>  Scan a directory (default: ~/Proyectos)")
			fmt.Println("  --all          Propagate to all unconfigured projects")
			fmt.Println("  --yes          Skip confirmation prompt")
			os.Exit(0)
		}
	}

	if scanPath == "" {
		paths, err := propagatepkg.LoadPropagatePaths()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error loading paths: %v\n", err)
			os.Exit(1)
		}
		if len(paths) == 0 {
			fmt.Fprintln(os.Stderr, "no paths configured")
			os.Exit(1)
		}
		scanPath = paths[0]
	}

	expandedPath := os.ExpandEnv(scanPath)
	infos, err := propagatepkg.Scan(expandedPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error scanning %s: %v\n", scanPath, err)
		os.Exit(1)
	}

	fmt.Print(propagatepkg.FormatTable(infos))

	unconfigured := make([]propagatepkg.ProjectInfo, 0)
	for _, info := range infos {
		if !info.HasDomain {
			unconfigured = append(unconfigured, info)
		}
	}

	if len(unconfigured) == 0 {
		fmt.Println("All projects already configured.")
		return
	}

	if all || yes {
		success, failed, errs := propagatepkg.Propagate(unconfigured, false)
		fmt.Printf("propagated to %d projects", success)
		if failed > 0 {
			fmt.Printf(", %d failed:\n", failed)
			for _, e := range errs {
				fmt.Printf("  - %v\n", e)
			}
		} else {
			fmt.Println()
		}
		return
	}

	fmt.Printf("\nFound %d unconfigured projects.\n", len(unconfigured))
	fmt.Println("Run with --all --yes to propagate all, or --scan <path> for a different directory.")
}

func runSetup(args []string) {
	if len(args) > 0 && args[0] == "auto-detect" {
		runAutoDetect(args[1:])
		return
	}
	if len(args) > 0 && args[0] == "wrapper-snippet" {
		runWrapperSnippet(args[1:])
		return
	}
	if len(args) > 0 && args[0] == "claude-hook" {
		runClaudeHook(args[1:])
		return
	}
	if len(args) > 0 && args[0] == "propagate" {
		runPropagate(args[1:])
		return
	}

	agent := "claude-code"
	mcpBinary := ""
	apiKey := os.Getenv("DOMAIN_API_KEY")
	baseURL := os.Getenv("DOMAIN_BASE_URL")
	autoInit := false
	skipInit := false
	global := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "claude-code", "claude":
			agent = "claude-code"
		case "opencode":
			agent = "opencode"
		case "claude-desktop", "desktop":
			agent = "claude-desktop"
		case "status":
			agent = "status"
		case "uninstall":
			agent = "uninstall"
		case "--global":
			global = true
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
		case "--auto-init":
			autoInit = true
		case "--skip-init":
			skipInit = true
		case "--help", "-h":
			fmt.Println("Usage: domain setup [claude-code|opencode|claude-desktop|status|uninstall] [flags]")
			fmt.Println("Registra domain-mcp como server MCP del agente elegido.")
			fmt.Println()
			fmt.Println("  claude-code     .mcp.json del proyecto actual (default, commiteable)")
			fmt.Println("  opencode        opencode.json del proyecto actual")
			fmt.Println("  claude-desktop  config global de Claude Desktop")
			fmt.Println("  status          muestra qué agentes tienen domain configurado")
			fmt.Println("  uninstall       quita domain de los configs del proyecto")
			fmt.Println()
			fmt.Println("Flags: --mcp-binary PATH | --api-key KEY | --base-url URL")
			fmt.Println("--auto-init  Después del setup, corre `domain init` que reemplaza")
			fmt.Println("             los .md de IA del repo actual por stubs apuntando al MCP.")
			fmt.Println("--skip-init  Saltea el prompt interactivo de init (no ofrece reemplazar).")
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
		// issue-01.9: use os.Stat en vez de strings.Replace (que falla si
		// el path tiene "/domain" adentro, e.g., /projects/domain/bin/domain).
		if found, err := findDomainMCPSibling(ex); err == nil {
			mcpBinary = found
		} else if found, lpErr := exec.LookPath("domain-mcp"); lpErr == nil {
			// Fallback: domain-mcp en PATH (install.sh lo deja en ~/go/bin).
			mcpBinary = found
		} else {
			// NUNCA escribir un entry con command vacío (HU-01.14: eso
			// deja al agente con "Connection closed" y requiere repair).
			fmt.Fprintf(os.Stderr, "%v\n", err)
			fmt.Fprintln(os.Stderr, "domain-mcp tampoco está en PATH. Compilalo primero:")
			fmt.Fprintln(os.Stderr, "  go build -o ~/go/bin/domain-mcp ./cmd/domain-mcp")
			fmt.Fprintln(os.Stderr, "o pasá la ruta con --mcp-binary /ruta/a/domain-mcp")
			os.Exit(1)
		}
	}

	cwd, _ := os.Getwd()

	var path string
	var err error
	var restartHint string
	switch agent {
	case "claude-code":
		if global {
			// User-scope: ~/.claude.json (disponible en todos los proyectos).
			path, err = setuppkg.SetupClaudeCodeGlobal(mcpBinary, apiKey, baseURL)
			restartHint = "Claude Code carga ~/.claude.json al iniciar (todos los proyectos)."
		} else {
			// Project-scope: .mcp.json en el repo actual (commiteable).
			path, err = setuppkg.SetupClaudeCode(cwd, mcpBinary, apiKey, baseURL)
			restartHint = "Claude Code detecta .mcp.json al abrir el proyecto (aprobá el server al primer uso)."
		}
	case "opencode":
		dir := cwd
		if global {
			if gd, gerr := setuppkg.OpenCodeGlobalDir(); gerr == nil {
				if mkErr := os.MkdirAll(gd, 0o755); mkErr == nil {
					dir = gd
				}
			}
		}
		path, err = setuppkg.SetupOpenCode(dir, mcpBinary, apiKey, baseURL)
		restartHint = "OpenCode carga opencode.json al iniciar en este proyecto."
		if global {
			restartHint = "OpenCode carga ~/.config/opencode/opencode.json al iniciar (todos los proyectos)."
		}
	case "claude-desktop":
		path, err = setuppkg.SetupClaudeDesktop(mcpBinary, apiKey, baseURL)
		restartHint = "Reinicia Claude Desktop para activar Domain."
	case "status":
		for _, st := range setuppkg.Status(cwd) {
			mark := "✗"
			if st.Configured {
				mark = "✓"
			} else if st.Exists {
				mark = "—"
			}
			fmt.Printf("%s %-15s %s\n", mark, st.Agent, st.ConfigPath)
		}
		return
	case "uninstall":
		removedAny := false
		for _, ag := range setuppkg.SupportedAgents {
			p, removed, uerr := setuppkg.Uninstall(ag, cwd)
			if uerr == nil && removed {
				fmt.Printf("✓ domain removido de %s (%s)\n", ag, p)
				removedAny = true
			}
		}
		if !removedAny {
			fmt.Println("domain no estaba configurado en ningún agente del proyecto.")
		}
		return
	default:
		fmt.Fprintf(os.Stderr, "agente no soportado: %s (soportados: claude-code, opencode, claude-desktop, status, uninstall)\n", agent)
		os.Exit(2)
	}

	if errors.Is(err, setuppkg.ErrAlreadyConfigured) {
		// return (no os.Exit): runSetup también se invoca in-process desde
		// `domain install` (configureAgents) — un Exit acá mataba el
		// install entero a mitad del paso de agentes en re-runs.
		fmt.Printf("Domain MCP ya configurado en %s — nada que hacer.\n", path)
		return
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "setup falló: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Domain MCP agregado a %s\n", path)
	if cwd != "" {
		if dp, derr := setuppkg.CreateAIDirectives(cwd); derr == nil {
			fmt.Printf("✓ Directivas creadas en %s\n", dp)
		}
	}
	fmt.Println("\n" + restartHint)

	// Hook auto-init: detectar archivos .md de IA en el cwd y ofrecer
	// reemplazarlos. Esto cierra el flow plug-and-play: post-setup el
	// agente IA tendrá su contexto desde el MCP en lugar de los .md.
	if skipInit || cwd == "" {
		return
	}
	offerOrRunAutoInit(cwd, autoInit)
}

// offerOrRunAutoInit detecta archivos .md de IA en cwd. Si autoInit=true,
// los reemplaza directo. Si false, los lista y sugiere correr `domain init`
// manualmente. Cierra el flow setup → init plug-and-play.
func offerOrRunAutoInit(cwd string, autoInit bool) {
	scanner := &workflowimport.Scanner{ProjectRoot: cwd}
	files, err := scanner.Detect(false)
	if err != nil || len(files) == 0 {
		return
	}
	fmt.Println()
	fmt.Printf("✓ Detectados %d archivos .md de IA en el proyecto actual:\n", len(files))
	for _, f := range files {
		fmt.Printf("    %s  (%s)\n", f.RelPath, f.SourceTool)
	}
	fmt.Println()

	if !autoInit {
		fmt.Println("Para reemplazar estos archivos por stubs que apunten al MCP de Domain:")
		fmt.Println("    domain init")
		fmt.Println("Para hacerlo automático en el próximo setup: --auto-init")
		return
	}

	// Auto-init: invoca workflowimport.Service.Import.
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "auto-init: no pude cargar config: %v\n", err)
		return
	}
	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "auto-init: db open falló: %v\n", err)
		return
	}
	defer pool.Close()

	svc := &workflowimport.Service{Pool: pool}
	rep, err := svc.Import(context.Background(), workflowimport.ImportInput{
		ProjectRoot:  cwd,
		StubTemplate: workflowimport.DefaultStub,
		WriteStub:    true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "auto-init: import falló: %v\n", err)
		return
	}
	fmt.Printf("✓ Auto-init OK — %d backed up, %d reemplazados con stub.\n",
		rep.BackedUp, rep.Replaced)
	fmt.Println("  Originales en BD (tabla imported_workflow_files).")
	fmt.Println("  Rollback: domain workflow restore <rel-path>")
}
