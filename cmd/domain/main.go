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
	"strconv"
	"time"

	"github.com/saargo/domain/internal/api/handler"
	"github.com/saargo/domain/internal/audit"
	"github.com/saargo/domain/internal/auth/apikey"
	"github.com/saargo/domain/internal/auth/otp"
	"github.com/saargo/domain/internal/config"
	"github.com/saargo/domain/internal/db"
	"github.com/saargo/domain/internal/httpserver"
	"github.com/saargo/domain/internal/llm"
	"github.com/saargo/domain/internal/logging"
	"github.com/saargo/domain/internal/metrics"
	dmigrate "github.com/saargo/domain/internal/migrate"
	"github.com/saargo/domain/internal/service/invite"
	"github.com/saargo/domain/internal/service/observation"
	orgsvc "github.com/saargo/domain/internal/service/org"
	projsvc "github.com/saargo/domain/internal/service/project"
	promptsvc "github.com/saargo/domain/internal/service/prompt"
	sesssvc "github.com/saargo/domain/internal/service/session"
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

Comandos:
  version             Muestra version + commit + build time
  help                Muestra esta ayuda
  migrate up          Aplica todas las migraciones DB pendientes
  migrate down [N]    Rollback N migraciones (default 1)
  migrate version     Muestra version actual del schema + dirty flag
  server              Inicia servidor HTTP (HU-01.3 /health)
  healthcheck         Probe interno para Dockerfile HEALTHCHECK

Más comandos vienen en Fase 2+ (ver openspec/INDEX.md y docs/roadmap.md).`)
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
	inviteService := &invite.Service{
		Pool: pools.App, Audit: recorder, Mailer: invite.NopMailer{},
		AcceptURL: "https://app.domain.sh/accept",
	}
	sessionService := &sesssvc.Service{Pool: pools.App, Audit: recorder}
	promptService := &promptsvc.Service{Pool: pools.App, Audit: recorder}
	apiKeyStore := &apikey.PGStore{Pool: pools.Auth}
	otpService := &otp.Service{
		Pool: pools.Auth, // Request/Verify cruzan org_id (lookup users por email)
	}

	api := &handler.API{
		OrgService:     orgService,
		ProjectService: projectService,
		ObsService:     obsService,
		InviteService:  inviteService,
		SessionService: sessionService,
		PromptService:  promptService,
		OTPService:     otpService,
		APIKeys:        apiKeyStore,
	}

	addr := fmt.Sprintf("%s:%d", cfg.HTTPBind, cfg.HTTPPort)
	mux := http.NewServeMux()
	info := httpserver.VersionInfo{Version: Version, Commit: Commit, BuildTime: BuildTime}
	mux.Handle("/health", &httpserver.HealthHandler{Info: info, StartedAt: time.Now()})
	mux.Handle("/health/ready", &httpserver.ReadyHandler{Pool: pools.App})

	// API REST montada bajo /api/v1/* con auth middleware aplicada selectivamente.
	authMW := &apikey.Middleware{Resolver: apiKeyStore, Allowlist: handler.AuthAllowlist()}
	mux.Handle("/api/", authMW.Wrap(api.Router()))

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
