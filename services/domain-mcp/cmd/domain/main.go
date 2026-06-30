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
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
	clicommands "nunezlagos/domain/internal/cli/commands"
	"nunezlagos/domain/internal/cli/onboard"
	setuppkg "nunezlagos/domain/internal/cli/setup"
	autodetect "nunezlagos/domain/internal/cli/setup/autodetect"
	claudehook "nunezlagos/domain/internal/cli/setup/claudehook"
	propagatepkg "nunezlagos/domain/internal/cli/setup/propagate"
	"nunezlagos/domain/internal/config"
	"nunezlagos/domain/internal/crypto"
	"nunezlagos/domain/internal/dbstats"
	debugpkg "nunezlagos/domain/internal/debug"
	"nunezlagos/domain/internal/logging"
	"nunezlagos/domain/internal/metrics"
	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/secrets"

	"nunezlagos/domain/internal/db"
	"nunezlagos/domain/internal/seeds"
	"nunezlagos/domain/internal/service/flow"
	"nunezlagos/domain/internal/service/lifecycle"
	analysissvc "nunezlagos/domain/internal/service/orchestrator/analysis"
	"nunezlagos/domain/internal/service/outboundwebhook"
	"nunezlagos/domain/internal/service/promptrouter"
	"nunezlagos/domain/internal/service/usagealerts"
	"nunezlagos/domain/internal/service/workflowimport"
	"nunezlagos/domain/internal/tracing"
)

// Variables sobrescritas por `-ldflags "-X main.Version=..."` (issue-19.2).
var (
	Version   = "0.0.0-dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func main() {

	loadEnvCascade()

	if len(os.Args) < 2 {

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
	case "seed-demo":
		runSeedDemo(os.Args[2:])
	case "embed-backfill":
		runEmbedBackfill(os.Args[2:])
	case "admin-passwd":
		runAdminPasswd(os.Args[2:])
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

		os.Exit(clicommands.Dispatch(os.Args[1:]))
	case "tui":

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
	debugpkg.TuneRuntime(logger)

	ctx := context.Background()

	slowQueryTracer, slowQueryStore := setupSlowQueryTracer(logger)
	db.SetObservabilityTracer(slowQueryTracer)
	defer slowQueryTracer.Close()

	pools, poolsClose, err := buildPools(ctx, cfg, logger, metricsReg)
	if err != nil {
		logger.Error("pools open failed", slog.Any("err", err))
		os.Exit(1)
	}
	defer poolsClose()

	slowQueryStore.SetPool(pools.App)

	otelSampleRatio := 0.1
	if v := os.Getenv("DOMAIN_OTEL_SAMPLE_RATIO"); v != "" {
		if f, perr := strconv.ParseFloat(v, 64); perr == nil {
			otelSampleRatio = f
		}
	}
	otelShutdown, oTelErr := tracing.Setup(ctx, tracing.Config{
		Enabled:      os.Getenv("DOMAIN_OTEL_ENABLED") == "true",
		OTLPEndpoint: envOr("DOMAIN_OTEL_ENDPOINT", "localhost:4317"),
		ServiceName:  "domain",
		Version:      Version,
		Environment:  cfg.Env,
		SampleRatio:  otelSampleRatio,
		Insecure:     envOr("DOMAIN_OTEL_INSECURE", "true") == "true",
	})
	if oTelErr != nil {
		logger.Error("tracing setup failed", slog.Any("err", oTelErr))
		os.Exit(1)
	}
	defer otelShutdown(ctx)

	seedRegistry := seeds.NewRegistry()
	seedRegistry.Register(&seeds.PlatformPoliciesSeeder{})
	seedRegistry.Register(&seeds.ProjectTemplatesSeeder{})
	seedRegistry.Register(&seeds.MCPProvidersSeeder{})
	seedRegistry.Register(&seeds.SkillsCatalogSeeder{})
	seedRegistry.Register(&seeds.AgentTemplatesCatalogSeeder{})
	seedRegistry.Register(&seeds.FlowsCatalogSeeder{})
	seedRegistry.Register(&seeds.TriagePromptSeeder{})
	seedRegistry.Register(&seeds.AnalysisPromptSeeder{})
	seedRegistry.Register(&seeds.WizardFormulatorPromptSeeder{})
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

	svc, err := buildServices(ctx, cfg, pools, logger, metricsReg)
	if err != nil {
		logger.Error("buildServices failed", slog.Any("err", err))
		os.Exit(1)
	}

	if os.Getenv("DOMAIN_DEBUG_ENABLED") == "true" {
		port, _ := strconv.Atoi(os.Getenv("DOMAIN_DEBUG_PORT"))
		go func() {
			err := debugpkg.Serve(debugpkg.Config{
				Enabled:       true,
				Bind:          os.Getenv("DOMAIN_DEBUG_BIND"),
				Port:          port,
				AuthUser:      os.Getenv("DOMAIN_DEBUG_AUTH_USER"),
				AuthPass:      os.Getenv("DOMAIN_DEBUG_AUTH_PASSWORD"),
				AuditRecorder: svc.Recorder,
				Metrics:       metricsReg,
			}, logger)
			if err != nil && err != http.ErrServerClosed {
				logger.Error("debug server failed", slog.Any("err", err))
			}
		}()
	}

	wireServices(svc)
	runners := buildRunners(ctx, cfg, pools, svc, metricsReg, logger)
	defer runners.SchedCancel()

	queryCacheLRU := mcpQueryCache()
	httpHandler, invocationLogger, httpLogger, resourceCollector, fnLogger, workflowTracker := buildRouter(cfg, Version, pools, svc, metricsReg, logger, queryCacheLRU)
	defer invocationLogger.Close()
	defer httpLogger.Close()
	defer resourceCollector.Stop()
	defer fnLogger.Close()
	workflowTracker.Start(context.Background())
	defer workflowTracker.Stop()

	logger.Info("domain server starting",
		slog.String("version", Version),
		slog.String("addr", fmt.Sprintf("%s:%d", cfg.HTTPBind, cfg.HTTPPort)),
		slog.String("env", cfg.Env),
		slog.Bool("metrics_enabled", cfg.MetricsEnabled),
	)
	startBackground(ctx, cfg, pools, svc, runners, metricsReg, httpHandler, queryCacheLRU, logger)
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

	if ex, err := os.Executable(); err == nil {
		w.DomainBinPath = ex
	}
	if mcp, err := findDomainMCPSibling(w.DomainBinPath); err == nil {
		w.DomainMCPPath = mcp
	}

	if nonInteractive && email != "" {

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
//
//	domain setup claude-code
//	domain setup --mcp-binary /usr/local/bin/domain-mcp --api-key sk_...
func runAutoDetect(args []string) {
	projectDir := "."
	quiet := false
	dryRun := false
	sessionContext := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--quiet", "-q":
			quiet = true
		case "--dry-run":
			dryRun = true
		case "--session-context":
			sessionContext = true
		case "--help", "-h":
			fmt.Println("Usage: domain setup auto-detect [path] [--quiet] [--dry-run] [--session-context]")
			fmt.Println("  path               Project directory (default: .)")
			fmt.Println("  --quiet, -q        Suppress non-error output")
			fmt.Println("  --dry-run          Show what would be done without modifying files")
			fmt.Println("  --session-context  Emit Claude Code SessionStart additionalContext JSON")
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

	// Modo hook de Claude Code: aplica las actions en silencio y emite el
	// additionalContext (precedencia domain + reglas locales detectadas). Se
	// emite SIEMPRE — el hook quiere el contexto en cada sesión, haya o no
	// cambios de config. No comparte salida con el wrapper de OpenCode (que
	// llama sin este flag).
	if sessionContext {
		_, _ = autodetect.Apply(absDir) // best-effort: no abortar el contexto si falla
		emitSessionContext(absDir)
		os.Exit(0)
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

// emitSessionContext imprime el JSON que Claude Code consume como
// additionalContext en el hook SessionStart. Recuerda el handshake domain y
// declara la precedencia sobre las reglas locales detectadas en el repo.
func emitSessionContext(absDir string) {
	var b strings.Builder
	b.WriteString("[domain] Protocolo domain ACTIVO en este repo. ")
	b.WriteString("Primer llamado SIEMPRE: domain_session_bootstrap(cwd, git_remote, git_branch, git_head); ")
	b.WriteString("luego domain_policy_get(slug=\"agent-protocol\") para el protocolo completo. ")
	b.WriteString("domain tiene PRIORIDAD sobre las reglas locales del repo en: memoria persistente, skills, policies SDD y tools domain_*.")

	if rules := autodetect.DetectRuleFiles(absDir); len(rules) > 0 {
		b.WriteString(" Reglas de IA locales detectadas (SUBORDINADAS a domain en esos temas; sus reglas técnicas —estilo/stack— siguen valiendo y domain las importa a BD): ")
		b.WriteString(strings.Join(rules, ", "))
		b.WriteString(".")
	}

	out := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "SessionStart",
			"additionalContext": b.String(),
		},
	}
	// Si el encode falla (no debería), no imprimas nada roto: mejor vacío.
	if data, err := json.Marshal(out); err == nil {
		fmt.Println(string(data))
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

		ex, err := os.Executable()
		if err != nil {
			fmt.Fprintln(os.Stderr, "no pude detectar el binario actual; pasá --mcp-binary")
			os.Exit(1)
		}

		if found, err := findDomainMCPSibling(ex); err == nil {
			mcpBinary = found
		} else if found, lpErr := exec.LookPath("domain-mcp"); lpErr == nil {

			mcpBinary = found
		} else {

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

			path, err = setuppkg.SetupClaudeCodeGlobal(mcpBinary, apiKey, baseURL)
			restartHint = "Claude Code carga ~/.claude.json al iniciar (todos los proyectos)."
		} else {

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

// analysisRunnerAdapter adapta analysissvc.Service al
// promptrouter.AnalysisRunner. Mismo patrón que cmd/domain-mcp/main.go.
type analysisRunnerAdapter struct{ inner *analysissvc.Service }

func (a *analysisRunnerAdapter) RunAnalysis(ctx context.Context, in promptrouter.AnalysisInput) (*promptrouter.AnalysisResult, error) {
	result, err := a.inner.RunAnalysis(ctx, analysissvc.Input{
		OrganizationID: in.OrganizationID,
		UserID:         in.UserID,
		RawText:        in.RawText,
	})
	if err != nil {
		return nil, err
	}
	return &promptrouter.AnalysisResult{
		KnowledgeDocID: result.KnowledgeDocID,
		Title:          result.Title,
		Body:           result.Body,
	}, nil
}
