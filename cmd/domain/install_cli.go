// Subcomandos `domain install`, `domain update`, `domain restore`,
// `domain seed` (issue-01.10 deploy-modes-update).
//
//   domain install [--mode local|cloud|hybrid] [--base-url URL]
//                  [--non-interactive|-y] [--no-backup] [--no-init]
//                  [--no-opencode] [--dsn URL]
//     Wizard idempotente: detecta estado, hace backups, corre migrate
//     + seed, y opcionalmente configura el agente.
//
//   domain update [--no-backup] [--no-seed] [--no-migrate]
//     Backups + migrate + seed. NO toca configs del agente.
//
//   domain restore <backup-path>
//     One-shot: restaura un archivo desde un backup timestamped.
//
//   domain seed all
//     Corre todos los seeders (skip-by-hash, idempotente).
//
// Cada subcomando es un wrapper delgado sobre las primitivas ya
// implementadas (migrate.Up, seeds.Registry.RunAll, install.BackupFile,
// install.Restore, install.ValidateDSN, install.StartDockerServices).
// Sin TUI, sin shims, sin variables de paquete.

package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/cli/install"
	"nunezlagos/domain/internal/config"
	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/seeds"
)

// runInstall es el entrypoint de `domain install`. Retorna exit code.
func runInstall(args []string) int {
	flags, err := parseInstallFlags(args)
	if err != nil {
		if errors.Is(err, errHelp) {
			return 0
		}
		fmt.Fprintln(os.Stderr, err.Error())
		printInstallHelp()
		return 2
	}

	// 5 steps esperados: detect, backup, migrate, seed, deploy-mode.
	progress := NewInstallProgress(5, os.Stderr)
	printBanner(os.Stderr)

	// 1. Detección de estado
	progress.StartStep("Detecting state")
	state, err := install.DetectState(flags.baseURL)
	if err != nil {
		progress.EndStep(StepFailed, err.Error())
		progress.Summary()
		return 1
	}
	progress.EndStep(StepOK, state.Summary())

	// 2. Backups (idempotentes, skip si el archivo no existe)
	if flags.noBackup {
		progress.StartStep("Backing up configs")
		progress.EndStep(StepSkipped, "--no-backup")
	} else {
		progress.StartStep("Backing up configs")
		backed, skipped := runBackupsCount()
		if backed == 0 && skipped == 0 {
			progress.EndStep(StepWarning, "no files to backup")
		} else {
			progress.EndStep(StepOK, fmt.Sprintf("%d backed up, %d skipped", backed, skipped))
		}
	}

	// 2b. Bootstrap .env (HU-01.13). Si .env falta y .env.example
	// existe, lo copiamos. Asi, el siguiente config.Load() no falla
	// por "DOMAIN_DATABASE_URL is required".
	progress.StartStep("Bootstrap .env")
	if err := ensureLocalEnvFile(); err != nil {
		progress.EndStep(StepFailed, err.Error())
		progress.Summary()
		return 1
	}
	progress.EndStep(StepOK, ".env present")

	// 3. Migrate
	cfg, err := config.Load()
	if err != nil {
		progress.StartStep("Applying migrations")
		progress.EndStep(StepFailed, err.Error())
		progress.Summary()
		return 1
	}
	progress.StartStep("Applying migrations")
	if err := dmigrate.Up(cfg.DatabaseURL); err != nil {
		progress.EndStep(StepFailed, err.Error())
		progress.Summary()
		return 1
	}
	progress.EndStep(StepOK, "schema up to date")

	// 4. Seeders
	progress.StartStep("Running seeders")
	if err := runSeedersViaRegistry(cfg.DatabaseURL, cfg.Env); err != nil {
		progress.EndStep(StepFailed, err.Error())
		progress.Summary()
		return 1
	}
	progress.EndStep(StepOK, "all catalogs at target version")

	// 5. Deployment mode (solo si el server está fresh; sino, ya
	//    está bootstrapped y no necesita re-bootstrap)
	if state.FirstRun {
		mode := flags.mode
		if mode == "" && !flags.nonInter {
			mode = promptDeploymentMode()
		}
		if mode == "" {
			mode = string(install.ModeLocal)
		}
		progress.StartStep(fmt.Sprintf("Deployment mode: %s", mode))
		if err := handleDeploymentMode(mode, flags, state); err != nil {
			progress.EndStep(StepFailed, err.Error())
			progress.Summary()
			return 1
		}
		progress.EndStep(StepOK, fmt.Sprintf("mode=%s configured", mode))
	} else {
		progress.StartStep("Deployment mode")
		progress.EndStep(StepSkipped, "already bootstrapped (use 'domain onboard' to re-auth)")
	}

	progress.Summary()
	return 0
}

// installFlags son los flags parseados de `domain install`.
type installFlags struct {
	mode      string
	baseURL   string
	dsn       string
	nonInter  bool
	noBackup  bool
	noInit    bool
	noOpencode bool
}

func parseInstallFlags(args []string) (installFlags, error) {
	f := installFlags{baseURL: envOr("DOMAIN_BASE_URL", "http://localhost:8000")}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--mode":
			if i+1 >= len(args) {
				return f, errors.New("missing value for --mode")
			}
			f.mode = args[i+1]
			i++
		case "--base-url":
			if i+1 >= len(args) {
				return f, errors.New("missing value for --base-url")
			}
			f.baseURL = args[i+1]
			i++
		case "--dsn":
			if i+1 >= len(args) {
				return f, errors.New("missing value for --dsn")
			}
			f.dsn = args[i+1]
			i++
		case "--non-interactive", "-y":
			f.nonInter = true
		case "--no-backup":
			f.noBackup = true
		case "--no-init":
			f.noInit = true
		case "--no-opencode":
			f.noOpencode = true
		case "--help", "-h":
			printInstallHelp()
			return f, errHelp
		default:
			return f, fmt.Errorf("unknown flag: %s", args[i])
		}
	}
	return f, nil
}

// runBackups corre los 3 backups canonicos. No falla si un archivo
// no existe (install.BackupFile retorna nil en ese caso).
func runBackups() {
	_, _ = runBackupsCount()
}

// runBackupsCount corre los backups y retorna (backed, skipped).
// Sin efectos en output (eso lo hace InstallProgress).
func runBackupsCount() (int, int) {
	backed, skipped := 0, 0
	for _, p := range []string{
		install.CredentialsPath(),
		".env",
		openCodeConfigPath(),
	} {
		res, err := install.BackupFile(p)
		if err != nil {
			skipped++ // contamos como skipped (err no es fatal, continua)
			continue
		}
		if res == nil {
			skipped++
		} else {
			backed++
		}
	}
	return backed, skipped
}

// ensureLocalEnvFile se asegura de que .env exista en el cwd. Si
// falta y .env.example existe, lo copia. Si falta ambos, error
// claro: el user probablemente no esta en el root del proyecto.
//
// Llamar ANTES de config.Load() para evitar "DOMAIN_DATABASE_URL is
// required" en fresh installs (HU-01.13).
func ensureLocalEnvFile() error {
	// .env ya existe: skip
	if _, err := os.Stat(".env"); err == nil {
		return nil
	}
	// .env.example no existe: error claro (no estamos en el root del proyecto)
	if _, err := os.Stat(".env.example"); err != nil {
		return fmt.Errorf(".env.example not found in current directory; " +
			"are you in the domain project root? (try: cd ~/.local/share/domain)")
	}
	// Copiar .env.example → .env
	data, err := os.ReadFile(".env.example")
	if err != nil {
		return fmt.Errorf("read .env.example: %w", err)
	}
	if err := os.WriteFile(".env", data, 0o600); err != nil {
		return fmt.Errorf("write .env: %w", err)
	}
	return nil
}

// handleDeploymentMode ejecuta el bootstrap del mode seleccionado.
// En fresh install (state.FirstRun), corre la logica de mode.
// En install sobre server ya corriendo, solo emite advertencias.
func handleDeploymentMode(mode string, f installFlags, state *install.InstallState) error {
	switch install.Mode(mode) {
	case install.ModeLocal:
		if !state.DockerAvailable {
			return errors.New("docker not found in PATH; install Docker or use --mode cloud")
		}
		services := install.LocalServices()
		fmt.Fprintf(os.Stderr, "  starting docker services: %v\n", services)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := install.StartDockerServices(ctx, services); err != nil {
			return fmt.Errorf("docker: %w", err)
		}
		fmt.Fprintln(os.Stderr, "  ✓ docker services healthy")
	case install.ModeCloud:
		dsn := f.dsn
		if dsn == "" {
			if f.nonInter {
				return errors.New("--dsn required in non-interactive cloud mode")
			}
			fmt.Fprintln(os.Stderr, "  Enter Database URL (postgres://user:pass@host:5432/db?sslmode=require):")
			line, err := readLine()
			if err != nil {
				return fmt.Errorf("read dsn: %w", err)
			}
			dsn = strings.TrimSpace(line)
		}
		if err := install.ValidateDSN(dsn); err != nil {
			return fmt.Errorf("DSN invalid: %w", err)
		}
		envContent := fmt.Sprintf("DOMAIN_DATABASE_URL=%s\nDOMAIN_BASE_URL=%s\n", dsn, f.baseURL)
		if err := os.WriteFile(".env", []byte(envContent), 0o600); err != nil {
			return fmt.Errorf("write .env: %w", err)
		}
		fmt.Fprintln(os.Stderr, "  ✓ DSN valid, .env written")
	case install.ModeHybrid:
		return errors.New("hybrid mode requires per-service prompts; not yet wired in this commit; use --mode local or --mode cloud")
	default:
		return fmt.Errorf("invalid mode: %q (expected local/cloud/hybrid)", mode)
	}

	if !state.ServerReachable {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "⚠ server not reachable. Start it in another terminal:")
		fmt.Fprintln(os.Stderr, "    domain server")
		fmt.Fprintln(os.Stderr, "  Then re-run install (idempotent).")
	}

	if !f.noInit {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Archiving .md files to BD (init)...")
		runInit(nil) // nil = defaults
		fmt.Fprintln(os.Stderr, "  ✓ init done")
	}

	if !f.noOpencode {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Configuring opencode MCP server...")
		// runSetup esta en main.go (issue-14.1). Si la API key ya
		// existe en credentials.json, la reusamos.
		key := readAPIKeyFromCredentials()
		if key == "" {
			fmt.Fprintln(os.Stderr, "  (no credentials yet; run 'domain setup opencode' after onboarding)")
		} else {
			runSetup([]string{
				"opencode",
				"--api-key", key,
				"--base-url", f.baseURL,
				"--skip-init",
			})
			fmt.Fprintln(os.Stderr, "  ✓ opencode MCP server configured")
		}
	}

	return nil
}

// runUpdate: backups + migrate + seed. Idempotente.
func runUpdate(args []string) int {
	noBackup, noSeed, noMigrate := false, false, false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--no-backup":
			noBackup = true
		case "--no-seed":
			noSeed = true
		case "--no-migrate":
			noMigrate = true
		case "--help", "-h":
			printUpdateHelp()
			return 0
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", args[i])
			return 2
		}
	}

	fmt.Fprintln(os.Stderr, "Domain Update (issue-01.10)")
	fmt.Fprintln(os.Stderr, "==========================")

	if !noBackup {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Backing up configs...")
		runBackups()
	} else {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "(skipping backups per --no-backup)")
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		return 1
	}

	if !noMigrate {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Applying migrations...")
		if err := dmigrate.Up(cfg.DatabaseURL); err != nil {
			fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
			return 1
		}
		fmt.Fprintln(os.Stderr, "  ✓ migrations up to date")
	}

	if !noSeed {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Running seeders (skip-by-hash)...")
		if err := runSeedersViaRegistry(cfg.DatabaseURL, cfg.Env); err != nil {
			fmt.Fprintf(os.Stderr, "seed: %v\n", err)
			return 1
		}
	}

	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "✓ Update complete.")
	return 0
}

// runSeed: alias de `update --no-backup --no-migrate`. Acepta
// `seed all` por ahora (puede extenderse a `seed <name>` mas adelante).
func runSeed(args []string) int {
	if len(args) == 0 || args[0] != "all" {
		fmt.Fprintln(os.Stderr, "Usage: domain seed all")
		return 2
	}
	return runUpdate([]string{"--no-backup", "--no-migrate"})
}

// runRestore: one-shot restore de un backup. Mapea el path del
// backup al target original via guessTargetFromBackup.
func runRestore(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: domain restore <backup-path>")
		fmt.Fprintln(os.Stderr, "Example: domain restore ~/.config/domain/credentials.json.bak.20260611T120000Z")
		return 2
	}
	backupPath := args[0]

	cfg, err := config.Load()
	baseURL := "http://localhost:8000"
	if err == nil && cfg != nil {
		baseURL = buildBaseURL(cfg)
	}

	targetPath := guessTargetFromBackup(backupPath)
	if targetPath == "" {
		fmt.Fprintf(os.Stderr, "could not guess target from %s\n", backupPath)
		return 1
	}

	res, err := install.Restore(backupPath, targetPath, baseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "restore: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "✓ Restored %s → %s (%d bytes)\n", res.Backup, res.Target, res.Bytes)
	if res.Notes != "" {
		fmt.Fprintf(os.Stderr, "  %s\n", res.Notes)
	}
	return 0
}

// guessTargetFromBackup mapea path de backup a target original.
// Heuristica: si el nombre base es "credentials.json", va a
// install.CredentialsPath(). Si es "opencode.json", va a
// ~/.config/opencode/opencode.json. Caso contrario, va al path
// sin sufijo .bak.YYYYMMDDTHHMMSSZ.
func guessTargetFromBackup(backupPath string) string {
	base := filepath.Base(backupPath)
	switch {
	case base == "credentials.json" || strings.HasPrefix(base, "credentials.json.bak"):
		return install.CredentialsPath()
	case base == "opencode.json" || strings.HasPrefix(base, "opencode.json.bak"):
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		return filepath.Join(home, ".config", "opencode", "opencode.json")
	case base == ".env" || strings.HasPrefix(base, ".env.bak"):
		return ".env"
	}

	// Generic: strip .bak.YYYY...Z
	name := backupPath
	if idx := strings.LastIndex(name, ".bak."); idx > 0 {
		return name[:idx]
	}
	return ""
}

// runSeedersViaRegistry construye la registry de seeders (igual
// que main.go hace al arrancar server) y la corre via RunAll con
// advisory lock. Esto reusa el codigo existente sin duplicar.
func runSeedersViaRegistry(databaseURL string, envStr string) error {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()

	registry := seeds.NewRegistry()
	registry.Register(&seeds.PlansSeeder{})
	registry.Register(&seeds.ModelRegistrySeeder{})
	registry.Register(&seeds.PlatformPoliciesSeeder{})
	registry.Register(&seeds.ProjectTemplatesSeeder{})

	reports, err := registry.RunAll(ctx, pool, seeds.Env(envStr))
	if err != nil {
		return fmt.Errorf("run all: %w", err)
	}

	// Print summary
	fmt.Fprintln(os.Stderr, "  seeders:")
	for name, rep := range reports {
		if rep.Skipped > 0 && rep.Created == 0 && rep.Updated == 0 && rep.Preserved == 0 {
			fmt.Fprintf(os.Stderr, "    · %s (skipped, up to date)\n", name)
		} else {
			fmt.Fprintf(os.Stderr, "    ✓ %s (created=%d updated=%d preserved=%d skipped=%d)\n",
				name, rep.Created, rep.Updated, rep.Preserved, rep.Skipped)
		}
	}
	return nil
}

// --- helpers ---

// readLine lee una linea de stdin. Usado para el prompt de DSN en
// cloud mode no-interactive.
func readLine() (string, error) {
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", errors.New("no input")
	}
	return scanner.Text(), nil
}

// readAPIKeyFromCredentials lee la API key actual de credentials.json.
// Retorna "" si no existe o es invalido (caso: pre-bootstrap).
func readAPIKeyFromCredentials() string {
	data, err := os.ReadFile(install.CredentialsPath())
	if err != nil {
		return ""
	}
	creds, err := install.ParseCredentials(data)
	if err != nil {
		return ""
	}
	return creds.APIKey
}

// buildBaseURL construye la URL base del server desde cfg.
// "127.0.0.1" -> "http://127.0.0.1:8000", "0.0.0.0" -> "http://localhost:8000".
// (Solo http; TLS handling vive en otra capa.)
func buildBaseURL(cfg *config.Config) string {
	bind := cfg.HTTPBind
	if bind == "" || bind == "0.0.0.0" {
		bind = "localhost"
	}
	return fmt.Sprintf("http://%s:%d", bind, cfg.HTTPPort)
}

// openCodeConfigPath retorna la ruta canonica de opencode.json.
func openCodeConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "opencode", "opencode.json")
}

// promptDeploymentMode pregunta al user el mode en stdin.
// Retorna "local" / "cloud" / "hybrid" o "" si cancelo.
func promptDeploymentMode() string {
	fmt.Fprintln(os.Stderr, "  Select deployment mode:")
	fmt.Fprintln(os.Stderr, "    1) local   — Postgres + S3 + SMTP via Docker (dev-friendly)")
	fmt.Fprintln(os.Stderr, "    2) cloud   — Bring your own services (DSN, S3, SMTP)")
	fmt.Fprintln(os.Stderr, "    3) hybrid  — Mix per-service (Postgres local, S3 cloud, etc.)")
	fmt.Fprintln(os.Stderr, "  Choice [1]:")
	line, err := readLine()
	if err != nil {
		return ""
	}
	switch strings.TrimSpace(line) {
	case "", "1", "local":
		return string(install.ModeLocal)
	case "2", "cloud":
		return string(install.ModeCloud)
	case "3", "hybrid":
		return string(install.ModeHybrid)
	}
	return ""
}

// errHelp es un sentinel para salir sin error despues de --help.
var errHelp = errors.New("help printed")

func printInstallHelp() {
	fmt.Println("Usage: domain install [flags]")
	fmt.Println()
	fmt.Println("  --mode {local|cloud|hybrid}    Deployment mode (default: interactive prompt)")
	fmt.Println("  --base-url URL                  Domain server URL (default: $DOMAIN_BASE_URL or http://localhost:8000)")
	fmt.Println("  --non-interactive, -y           Skip prompts (use defaults or flags)")
	fmt.Println("  --no-backup                     Skip automatic backups before mutations")
	fmt.Println("  --no-init                       Skip init (archiving .md to BD)")
	fmt.Println("  --no-opencode                   Skip opencode MCP config")
	fmt.Println("  --dsn URL                       Database URL (cloud mode, non-interactive)")
	fmt.Println("  --help, -h                      Show this help")
}

func printUpdateHelp() {
	fmt.Println("Usage: domain update [flags]")
	fmt.Println("  --no-backup    Skip automatic backups")
	fmt.Println("  --no-seed      Skip seeders")
	fmt.Println("  --no-migrate   Skip migrations")
	fmt.Println("  --help, -h     Show this help")
}
