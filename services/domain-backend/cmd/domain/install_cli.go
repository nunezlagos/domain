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
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/cli/install"
	"nunezlagos/domain/internal/cli/onboard"
	setuppkg "nunezlagos/domain/internal/cli/setup"
	claudehook "nunezlagos/domain/internal/cli/setup/claudehook"
	primarymem "nunezlagos/domain/internal/cli/setup/primary_memory"
	"nunezlagos/domain/internal/config"
	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/seeds"
)

// runInstall es el entrypoint de `domain install`. Retorna exit code.
//
// Orden (rediseño 2026-06-11): la infraestructura (docker/DSN) arranca
// ANTES de migrate — en una máquina fresh la BD no existe hasta que
// docker compose la levanta. Después de seed, si no hay credenciales,
// se bootstrapea la API key local automáticamente para que domain-mcp
// pueda arrancar desde los agentes sin pasos manuales.
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

	// Step 0: cwd guard. ANTES de cualquier side effect (loadEnv,
	// backups, docker, etc). Si el cwd no es un root de repo válido
	// y no se pasó --src, aborta con exit 1 y mensaje accionable.
	if _, ok := checkProjectRootGuard(flags.src); !ok {
		return 1
	}

	mode := flags.mode
	if mode == "" && !flags.nonInter {
		mode = promptDeploymentMode()
	}
	if mode == "" {
		mode = string(install.ModeLocal)
	}

	progress := NewInstallProgress(12, os.Stderr)
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
	progress.StartStep("Backing up configs")
	if flags.noBackup {
		progress.EndStep(StepSkipped, "--no-backup")
	} else {
		backed, skipped := runBackupsCount()
		if backed == 0 && skipped == 0 {
			progress.EndStep(StepWarning, "no files to backup")
		} else {
			progress.EndStep(StepOK, fmt.Sprintf("%d backed up, %d skipped", backed, skipped))
		}
	}

	// 3. Bootstrap .env: copia .env.example si falta, carga al env,
	// y persiste el puerto/base-url elegidos (DOMAIN_HTTP_PORT).
	progress.StartStep("Bootstrap .env")
	if err := ensureLocalEnvFile(); err != nil {
		progress.EndStep(StepFailed, err.Error())
		progress.Summary()
		return 1
	}
	persistBaseURLEnv(flags.baseURL)
	if err := loadEnvFile(".env"); err != nil {
		progress.EndStep(StepWarning, "could not load .env: "+err.Error())
	} else {
		progress.EndStep(StepOK, ".env present and loaded")
	}

	// 4. Infraestructura: docker (local) o DSN (cloud). ANTES de
	// migrate — sin esto la BD no existe en fresh installs.
	progress.StartStep(fmt.Sprintf("Starting services (%s)", mode))
	if err := startInfra(mode, flags, state); err != nil {
		progress.EndStep(StepFailed, err.Error())
		progress.Summary()
		return 1
	}
	progress.EndStep(StepOK, fmt.Sprintf("mode=%s ready", mode))

	// 5. Migrate
	cfg, err := config.Load()
	if err != nil {
		progress.StartStep("Applying migrations")
		progress.EndStep(StepFailed, err.Error())
		progress.Summary()
		return 1
	}
	progress.StartStep("Applying migrations")
	if err := dmigrate.Up(cfg.DatabaseURL); err != nil {
		msg := err.Error()
		// Caso clásico: la BD tiene migraciones MÁS NUEVAS que este código
		// (e.g. install.sh corrido desde un clon desactualizado mientras el
		// repo de desarrollo ya aplicó versiones nuevas a la misma BD).
		if strings.Contains(msg, "no migration found for version") {
			msg += " — tu BD tiene migraciones más nuevas que este código; " +
				"actualizá el código y reintentá (re-corré install.sh, que hace git pull)"
		}
		progress.EndStep(StepFailed, msg)
		progress.Summary()
		return 1
	}
	progress.EndStep(StepOK, "schema up to date")

	// 6. Seeders
	progress.StartStep("Running seeders")
	if err := runSeedersViaRegistry(cfg.DatabaseURL, cfg.Env); err != nil {
		progress.EndStep(StepFailed, err.Error())
		progress.Summary()
		return 1
	}
	progress.EndStep(StepOK, "all catalogs at target version")

	// 7. Env global para domain-mcp y el server systemd: DSN, base-url y
	// puerto. Va ANTES del server para que arranque en el puerto elegido.
	progress.StartStep("Global MCP env")
	envPath, err := writeGlobalMCPEnv(cfg, flags.baseURL)
	if err != nil {
		progress.EndStep(StepWarning, err.Error())
	} else {
		progress.EndStep(StepOK, envPath)
	}

	// 8. Server como systemd user service: queda corriendo siempre y
	// arranca al login (plug-and-play). Va ANTES de la API key porque el
	// flujo OTP de re-installs necesita el server arriba. Skip limpio
	// fuera de Linux o sin user manager (containers/macOS).
	progress.StartStep("Starting server (systemd)")
	serverUp := state.ServerReachable
	switch {
	case flags.noService:
		progress.EndStep(StepSkipped, "--no-service")
	case !systemdUserAvailable():
		progress.EndStep(StepSkipped, "systemd user manager not available (run 'domain server' manually)")
	default:
		if err := installUserService(flags.baseURL); err != nil {
			progress.EndStep(StepWarning, err.Error())
		} else {
			serverUp = true
			progress.EndStep(StepOK, "domain.service enabled + running (starts at login)")
		}
	}

	// 9. API key: tres caminos según estado (sin pasos manuales):
	//    - credentials.json existe → skip
	//    - BD sin usuarios (first-run) → crea org + user con TU email +
	//      key directo a la BD
	//    - BD con usuarios pero sin credentials (re-install) → flujo OTP
	//      real: request-otp → auto-fetch del código en mailpit (local
	//      only) → verify-otp
	progress.StartStep("API key")
	switch how, err := ensureAPIKey(cfg, flags, mode, serverUp); {
	case err != nil:
		progress.EndStep(StepFailed, err.Error())
		progress.Summary()
		return 1
	case how == "":
		progress.EndStep(StepSkipped, "credentials.json already present")
	default:
		progress.EndStep(StepOK, how)
	}

	// 10. Agentes MCP (multi: opencode y/o claude-code). Idempotente.
	progress.StartStep("Configuring MCP agents")
	if len(flags.agents) == 0 {
		progress.EndStep(StepSkipped, "no agents selected")
	} else {
		detail := configureAgents(flags.agents, flags.baseURL)
		progress.EndStep(StepOK, detail)
	}

	// 10.5 Shell wrapper (issue-30.2). Opt-in con confirm explícito.
	progress.StartStep("Config shell wrapper")
	if flags.withWrapper {
		detail, wErr := installShellWrapperStep()
		if wErr != nil {
			progress.EndStep(StepWarning, wErr.Error())
		} else {
			progress.EndStep(StepOK, detail)
		}
	} else if !flags.nonInter {
		detail, wErr := promptShellWrapper()
		if wErr != nil {
			progress.EndStep(StepWarning, wErr.Error())
		} else if detail == "" {
			progress.EndStep(StepSkipped, "user declined")
		} else {
			progress.EndStep(StepOK, detail)
		}
	} else {
		progress.EndStep(StepSkipped, "--non-interactive (use --with-wrapper)")
	}

	// 10.6 Claude Code SessionStart hook (issue-30.3).
	progress.StartStep("Claude Code SessionStart hook")
	if flags.noClaudeHook {
		progress.EndStep(StepSkipped, "--no-claude-hook")
	} else if flags.withClaudeHook {
		action, hErr := claudehook.InstallClaudeHook(false, true)
		if hErr != nil {
			progress.EndStep(StepWarning, hErr.Error())
		} else {
			progress.EndStep(StepOK, action)
		}
	} else if !flags.nonInter {
		action, hErr := claudehook.InstallClaudeHook(false, false)
		if hErr != nil {
			progress.EndStep(StepWarning, hErr.Error())
		} else {
			progress.EndStep(StepOK, action)
		}
	} else {
		progress.EndStep(StepSkipped, "--non-interactive (use --with-claude-hook)")
	}

	// 10.7 Primary memory (issue-35.3). Opt-in: detecta engram/mem0
	// en opencode.json + .claude.json y los desactiva (con backup)
	// para que domain sea la única memory provider visible al LLM.
	progress.StartStep("Primary memory (engram/mem0 detect)")
	if !flags.primaryMemory {
		progress.EndStep(StepSkipped, "use --primary-memory para detectar")
	} else {
		detail, pmErr := runPrimaryMemoryStep(flags)
		switch {
		case pmErr != nil:
			progress.EndStep(StepWarning, pmErr.Error())
		case detail == "":
			progress.EndStep(StepSkipped, "user declined")
		default:
			progress.EndStep(StepOK, detail)
		}
	}

	// 11. Init (.md → BD). Requiere el server HTTP corriendo.
	progress.StartStep("Importing .md files")
	switch {
	case flags.noInit:
		progress.EndStep(StepSkipped, "--no-init")
	case !serverUp:
		progress.EndStep(StepSkipped, "server not running (start 'domain server' and run 'domain init')")
	default:
		runInit(nil)
		progress.EndStep(StepOK, "configs archived to BD")
	}

	progress.Summary()

	if !serverUp {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "⚠ server not running. Start it with:")
		fmt.Fprintln(os.Stderr, "    domain server")
		fmt.Fprintln(os.Stderr, "  (o instalalo como servicio: domain service install)")
	}
	return 0
}

// configureOpencodeMCPServer agrega el MCP server "domain" al opencode.json
// del cwd actual. Es idempotente: si ya esta configurado, no hace nada.
// La API key puede ser "" (fresh install) — el user la setea despues
// con `domain setup opencode --api-key sk_xxx`.
//
// HU-01.14: tambien valida que el "command" del entry no este vacio.
// Si lo esta (por un install previo que fallo al encontrar
// domain-mcp), borra el entry y re-crea con el path correcto.
func configureOpencodeMCPServer(baseURL string) {
	// Reparar tanto el opencode.json del cwd como el global.
	cwd, _ := os.Getwd()
	for _, p := range []string{
		filepath.Join(cwd, "opencode.json"),
		openCodeConfigPath(),
	} {
		if repairOpencodeEmptyCommandAt(p) {
			fmt.Fprintf(os.Stderr, "  (reparado %s con command vacio previo)\n", p)
		}
	}
	key := readAPIKeyFromCredentials() // "" si fresh install
	setupArgs := []string{
		"opencode",
		"--global",
		"--base-url", baseURL,
		"--skip-init",
	}
	if key != "" {
		setupArgs = append(setupArgs, "--api-key", key)
	}
	runSetup(setupArgs)
}

// repairOpencodeEmptyCommand repara el opencode.json del cwd (compat
// con tests de HU-01.14).
func repairOpencodeEmptyCommand() bool {
	cwd, _ := os.Getwd()
	return repairOpencodeEmptyCommandAt(filepath.Join(cwd, "opencode.json"))
}

// repairOpencodeEmptyCommandAt busca en el opencode.json dado el entry
// "mcp.domain" y si su "command" es "" o ["", ...], lo borra.
// Retorna true si reparo algo.
func repairOpencodeEmptyCommandAt(cfgPath string) bool {
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return false
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return false
	}
	mcp, ok := doc["mcp"].(map[string]any)
	if !ok {
		return false
	}
	entry, ok := mcp["domain"].(map[string]any)
	if !ok {
		return false
	}
	cmd, ok := entry["command"].([]any)
	if !ok {
		return false
	}
	// Roto si: slice vacío, primer elemento "" (install previo que no
	// encontró domain-mcp), o el binario apuntado YA NO EXISTE (install
	// legacy en otro dir, e.g. ~/.local/bin borrado). Borramos el entry
	// para que el setup lo recree — sin esto el agente ve "-32000
	// Connection closed" por ENOENT.
	broken := len(cmd) == 0
	if !broken {
		if first, ok := cmd[0].(string); ok {
			if first == "" {
				broken = true
			} else if _, statErr := os.Stat(first); statErr != nil {
				broken = true
			}
		}
	}
	if !broken {
		return false
	}
	delete(mcp, "domain")
	out, _ := json.MarshalIndent(doc, "", "  ")
	return os.WriteFile(cfgPath, out, 0o600) == nil
}

// installFlags son los flags parseados de `domain install`.
type installFlags struct {
	mode           string
	baseURL        string
	dsn            string
	email          string
	nonInter       bool
	noBackup       bool
	noInit         bool
	noService      bool
	agents         []string
	src            string
	withWrapper    bool
	withClaudeHook bool
	noClaudeHook   bool
	primaryMemory  bool // opt-in: detecta engram/mem0 y los desactiva
	pmReactivate   bool // restaurar desde backup en vez de disable
	pmYes          bool // skip confirm en --primary-memory
}

func parseInstallFlags(args []string) (installFlags, error) {
	f := installFlags{
		baseURL: envOr("DOMAIN_BASE_URL", "http://localhost:8000"),
		agents:  []string{"opencode"},
	}
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
		case "--agents":
			if i+1 >= len(args) {
				return f, errors.New("missing value for --agents")
			}
			f.agents = parseAgentsCSV(args[i+1])
			i++
		case "--email":
			if i+1 >= len(args) {
				return f, errors.New("missing value for --email")
			}
			f.email = args[i+1]
			i++
		case "--non-interactive", "-y":
			f.nonInter = true
		case "--no-backup":
			f.noBackup = true
		case "--no-init":
			f.noInit = true
		case "--no-service":
			f.noService = true
		case "--with-wrapper":
			f.withWrapper = true
		case "--with-claude-hook":
			f.withClaudeHook = true
		case "--no-claude-hook":
			f.noClaudeHook = true
		case "--primary-memory":
			f.primaryMemory = true
		case "--reactivate-memory":
			f.pmReactivate = true
		case "--primary-memory-yes":
			f.pmYes = true
		case "--no-opencode":
			// compat HU-01.14: equivale a sacar opencode de --agents
			f.agents = removeAgent(f.agents, "opencode")
		case "--src":
			if i+1 >= len(args) {
				return f, errors.New("missing value for --src")
			}
			f.src = args[i+1]
			i++
		case "--help", "-h":
			printInstallHelp()
			return f, errHelp
		default:
			return f, fmt.Errorf("unknown flag: %s", args[i])
		}
	}
	return f, nil
}

// parseAgentsCSV parsea "opencode,claude-code" filtrando desconocidos
// y vacíos. "" o "none" → lista vacía.
func parseAgentsCSV(csv string) []string {
	var out []string
	for _, a := range strings.Split(csv, ",") {
		a = strings.TrimSpace(a)
		switch a {
		case "opencode", "claude-code":
			out = append(out, a)
		}
	}
	return out
}

func removeAgent(agents []string, target string) []string {
	out := agents[:0]
	for _, a := range agents {
		if a != target {
			out = append(out, a)
		}
	}
	return out
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

// loadEnvCascade carga la config plug-and-play en orden de precedencia
// (sin pisar nada ya exportado en el shell):
//  1. env vars del proceso (ganan siempre)
//  2. .env del cwd (desarrollo dentro del repo)
//  3. ~/.config/domain/env (global, escrito por `domain install`)
//  4. DOMAIN_API_KEY desde credentials.json si sigue faltando
//
// Se invoca al inicio de main() para que TODOS los subcomandos
// (server, projects, onboard, etc.) funcionen desde cualquier
// directorio sin `source .env` manual.
func loadEnvCascade() {
	_ = loadEnvFile(".env")
	if home, err := os.UserHomeDir(); err == nil {
		_ = loadEnvFile(filepath.Join(home, ".config", "domain", "env"))
	}
	if os.Getenv("DOMAIN_API_KEY") == "" {
		if key := readAPIKeyFromCredentials(); key != "" {
			_ = os.Setenv("DOMAIN_API_KEY", key)
		}
	}
}

// loadEnvFile parsea un archivo .env y setea las variables en el
// environment del proceso. Implementacion minima: KEY=VALUE por
// linea, ignora comentarios (#) y lineas vacias. NO soporta
// quoting/escape (es suficiente para .env.example de domain).
//
// Esto evita depender de godotenv o similar. Si el archivo tiene
// cosas raras, las ignora silenciosamente.
func loadEnvFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		// Strip surrounding quotes si los tiene
		val = strings.Trim(val, `"'`)
		// NO pisar env vars que ya existen (el user podria haber
		// seteado algo en su shell que queremos respetar).
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
	return nil
}

// startInfra levanta la infraestructura según el mode. Idempotente:
// docker compose up -d no hace nada si los servicios ya corren.
func startInfra(mode string, f installFlags, state *install.InstallState) error {
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
		// upsert (NO sobrescribir el .env entero: preserva el resto)
		if err := upsertEnvFile(".env", "DOMAIN_DATABASE_URL", dsn); err != nil {
			return fmt.Errorf("write .env: %w", err)
		}
		_ = os.Setenv("DOMAIN_DATABASE_URL", dsn)
		fmt.Fprintln(os.Stderr, "  ✓ DSN valid, .env updated")
	case install.ModeHybrid:
		return errors.New("hybrid mode not available yet; use --mode local or --mode cloud")
	default:
		return fmt.Errorf("invalid mode: %q (expected local/cloud/hybrid)", mode)
	}
	return nil
}

// persistBaseURLEnv guarda DOMAIN_BASE_URL y deriva DOMAIN_HTTP_PORT
// del --base-url elegido (para que 'domain server' escuche ahí).
func persistBaseURLEnv(baseURL string) {
	if baseURL == "" {
		return
	}
	_ = upsertEnvFile(".env", "DOMAIN_BASE_URL", baseURL)
	if u, err := url.Parse(baseURL); err == nil && u.Port() != "" {
		_ = upsertEnvFile(".env", "DOMAIN_HTTP_PORT", u.Port())
		_ = os.Setenv("DOMAIN_HTTP_PORT", u.Port())
	}
	_ = os.Setenv("DOMAIN_BASE_URL", baseURL)
}

// ensureAPIKey resuelve las credenciales sin pasos manuales. Retorna
// (how, err): how=="" significa skip (credentials ya presentes); si no,
// describe el camino usado. La key plaintext NUNCA se imprime: va a
// credentials.json (0600) y a .env como DOMAIN_API_KEY.
func ensureAPIKey(cfg *config.Config, flags installFlags, mode string, serverUp bool) (string, error) {
	if readAPIKeyFromCredentials() != "" {
		return "", nil
	}
	email := flags.email
	if email == "" {
		email = "admin@local.domain"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return "", fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()

	var userCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&userCount); err != nil {
		return "", fmt.Errorf("count users: %w", err)
	}

	// First-run: la BD está vacía → crear la cuenta con el email del
	// user, directo a la BD (no hay nada que validar por OTP todavía).
	if userCount == 0 {
		prefix, err := bootstrapFirstAccount(ctx, pool, email, flags.baseURL)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("account created for %s (key prefix %s)", email, prefix), nil
	}

	// Re-install (hay usuarios pero no credentials): flujo OTP real.
	// SOLO en local buscamos el código automáticamente en mailpit.
	if install.Mode(mode) == install.ModeLocal && serverUp && mailpitAvailable() {
		result, err := otpFlowViaServer(flags.baseURL, email)
		if err != nil {
			return "", fmt.Errorf("OTP flow: %w (alternativa: domain onboard)", err)
		}
		if err := saveCredentialsFromOTP(result, flags.baseURL); err != nil {
			return "", err
		}
		return fmt.Sprintf("OTP verified via mailpit for %s", result.Email), nil
	}

	return "", fmt.Errorf("hay usuarios en la BD pero no credentials.json; "+
		"corré 'domain onboard --base-url %s' para autenticarte (OTP por email)", flags.baseURL)
}

// bootstrapFirstAccount crea el primer user (owner) + API key directo a la BD.
// Single-org: ya no existe la entidad organization. Retorna el prefix de la key.
func bootstrapFirstAccount(ctx context.Context, pool *pgxpool.Pool, email, baseURL string) (string, error) {
	var userID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO users (email, name, role)
		 VALUES ($1, 'Admin Local', 'owner')
		 ON CONFLICT (email) DO UPDATE SET role = 'owner'
		 RETURNING id`, email).Scan(&userID)
	if err != nil {
		return "", fmt.Errorf("create user: %w", err)
	}

	rawKey, prefix, hash, err := apikey.Generate("dev")
	if err != nil {
		return "", fmt.Errorf("generate api_key: %w", err)
	}
	var keyID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO api_keys (user_id, name, key_prefix, key_hash)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		userID, "install-"+time.Now().UTC().Format("20060102-150405"), prefix, hash,
	).Scan(&keyID)
	if err != nil {
		return "", fmt.Errorf("create api_key: %w", err)
	}

	creds := &onboard.Credentials{
		APIKey:   rawKey,
		APIKeyID: keyID,
		UserID:   userID,
		Email:    email,
		BaseURL:  baseURL,
		IssuedAt: time.Now().UTC(),
	}
	return prefix, persistCredentials(creds)
}

// saveCredentialsFromOTP arma y persiste credentials desde el resultado
// del verify-otp.
func saveCredentialsFromOTP(r *otpVerifyResult, baseURL string) error {
	creds := &onboard.Credentials{
		APIKey:   r.APIKey,
		Email:    r.Email,
		BaseURL:  baseURL,
		IssuedAt: time.Now().UTC(),
	}
	if id, err := uuid.Parse(r.APIKeyID); err == nil {
		creds.APIKeyID = id
	}
	if id, err := uuid.Parse(r.UserID); err == nil {
		creds.UserID = id
	}
	if id, err := uuid.Parse(r.OrgID); err == nil {
		creds.OrgID = id
	}
	return persistCredentials(creds)
}

// persistCredentials guarda credentials.json (0600) + DOMAIN_API_KEY en
// .env y en el env del proceso.
func persistCredentials(creds *onboard.Credentials) error {
	if err := onboard.SaveCredentialsDefault(creds); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}
	if err := upsertEnvFile(".env", "DOMAIN_API_KEY", creds.APIKey); err != nil {
		return fmt.Errorf("update .env: %w", err)
	}
	_ = os.Setenv("DOMAIN_API_KEY", creds.APIKey)
	return nil
}

// writeGlobalMCPEnv escribe ~/.config/domain/env con la config que
// domain-mcp necesita para arrancar desde cualquier cwd (los agentes
// lanzan el binario fuera del repo). Sin esto, config.Load() falla y
// el agente ve "MCP error -32000: Connection closed".
func writeGlobalMCPEnv(cfg *config.Config, baseURL string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "domain")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "env")
	pairs := [][2]string{
		{"DOMAIN_DATABASE_URL", cfg.DatabaseURL},
		{"DOMAIN_BASE_URL", baseURL},
	}
	// CRÍTICO: el server de systemd corre con cwd=$HOME y lee SOLO este
	// archivo. Sin DOMAIN_HTTP_PORT acá, escucha en el default 8000
	// aunque el user haya elegido otro puerto — y el health-check del
	// install espera en el puerto elegido → warning falso.
	if u, err := url.Parse(baseURL); err == nil && u.Port() != "" {
		pairs = append(pairs, [2]string{"DOMAIN_HTTP_PORT", u.Port()})
	}
	if cfg.DatabaseAuthURL != "" {
		pairs = append(pairs, [2]string{"DOMAIN_DATABASE_AUTH_URL", cfg.DatabaseAuthURL})
	}
	for _, kv := range pairs {
		if kv[1] == "" {
			continue
		}
		if err := upsertEnvFile(path, kv[0], kv[1]); err != nil {
			return "", err
		}
	}
	return path, nil
}

// runPrimaryMemoryStep ejecuta la lógica de --primary-memory: detecta
// providers de memoria (engram, mem0, etc) en opencode.json y
// ~/.claude.json y los desactiva con backup. Si --reactivate-memory,
// restaura desde el último backup.
//
// Retorna (detalle, error). detalle="" significa que el user declinó.
// El catalog efectivo viene de primarymem.LoadCatalog (hardcoded +
// override en ~/.config/domain/primary-memory-catalog.json).
func runPrimaryMemoryStep(flags installFlags) (string, error) {
	catalog, _ := primarymem.LoadCatalog()
	agents := []string{"opencode", "claude-code"}

	if flags.pmReactivate {
		return reactivateMemoryProviders(agents)
	}

	type hit struct {
		agent      string
		configPath string
		names      []string
	}
	var hits []hit
	for _, agent := range agents {
		path, err := primarymem.DefaultConfigPath(agent)
		if err != nil {
			continue
		}
		providers, err := primarymem.Detect(agent, path)
		if err != nil {
			continue
		}
		var names []string
		for _, p := range providers {
			// Filtrar por el catalog efectivo (override permite override).
			if !catalog[p.Name] {
				continue
			}
			disabled, _ := primarymem.IsAlreadyDisabled(agent, path, p.Name)
			if disabled {
				continue
			}
			names = append(names, p.Name)
		}
		if len(names) > 0 {
			sort.Strings(names)
			hits = append(hits, hit{agent: agent, configPath: path, names: names})
		}
	}

	if len(hits) == 0 {
		return "no other memory providers detected", nil
	}

	// Preview
	fmt.Fprintln(os.Stderr, "  found other memory MCP providers:")
	for _, h := range hits {
		fmt.Fprintf(os.Stderr, "    %s → %s: %s\n", h.agent, h.configPath, strings.Join(h.names, ", "))
	}

	// Confirm si no es --primary-memory-yes ni --non-interactive.
	if !flags.pmYes && !flags.nonInter {
		fmt.Fprint(os.Stderr, "  disable these (backup will be created)? [y/N] ")
		line, err := readLine()
		if err != nil {
			return "", nil
		}
		ans := strings.ToLower(strings.TrimSpace(line))
		if ans != "y" && ans != "yes" {
			return "", nil
		}
	}

	var done []string
	for _, h := range hits {
		if err := primarymem.Disable(h.agent, h.configPath, h.names); err != nil {
			return "", fmt.Errorf("disable %s: %w", h.agent, err)
		}
		done = append(done, fmt.Sprintf("%s[%s]", h.agent, strings.Join(h.names, ",")))
	}
	return "disabled " + strings.Join(done, " "), nil
}

// reactivateMemoryProviders restaura desde el último backup para
// cada agente. No falla si no hay backup en un agente — solo loggea.
func reactivateMemoryProviders(agents []string) (string, error) {
	var restored []string
	var soft []string
	for _, agent := range agents {
		path, err := primarymem.DefaultConfigPath(agent)
		if err != nil {
			continue
		}
		if err := primarymem.Reactivate(agent, path); err != nil {
			soft = append(soft, agent+": "+err.Error())
			continue
		}
		restored = append(restored, agent)
	}
	if len(restored) == 0 {
		if len(soft) > 0 {
			return "", fmt.Errorf("%s", strings.Join(soft, "; "))
		}
		return "no backups to reactivate", nil
	}
	return "reactivated " + strings.Join(restored, ", "), nil
}

// configureAgents corre el setup para cada agente elegido. Retorna un
// detalle human-readable para el progress.
func configureAgents(agents []string, baseURL string) string {
	key := readAPIKeyFromCredentials()
	var done []string
	for _, agent := range agents {
		switch agent {
		case "opencode":
			configureOpencodeMCPServer(baseURL)
			done = append(done, "opencode")
		case "claude-code":
			args := []string{"claude-code", "--global", "--base-url", baseURL, "--skip-init"}
			if key != "" {
				args = append(args, "--api-key", key)
			}
			runSetup(args)
			done = append(done, "claude-code")
		default:
			fmt.Fprintf(os.Stderr, "  (agente desconocido: %s, skipped)\n", agent)
		}
	}
	if len(done) == 0 {
		return "none"
	}
	return strings.Join(done, ", ")
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
	// REQ-42.3: ModelRegistrySeeder removido (model_registry dropeada).
	registry.Register(&seeds.PlatformPoliciesSeeder{})
	registry.Register(&seeds.ProjectTemplatesSeeder{})
	registry.Register(&seeds.MCPProvidersSeeder{})

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

// installShellWrapperStep agrega el shell wrapper al rcfile detectado.
// Retorna detalle del resultado o error.
func installShellWrapperStep() (string, error) {
	shell, rcfile := setuppkg.DetectShell()
	home, err := os.UserHomeDir()
	if err == nil {
		if strings.HasPrefix(rcfile, home) {
			rcfile = strings.Replace(rcfile, home, "~", 1)
		}
	}
	installed, err := setuppkg.InstallShellWrapper(rcfile)
	if err != nil {
		return "", fmt.Errorf("install wrapper: %w", err)
	}
	if !installed {
		return shell + " wrapper already installed in " + rcfile + " (skip)", nil
	}
	return shell + " wrapper added to " + rcfile, nil
}

// promptShellWrapper pregunta al user si quiere el shell wrapper.
// Retorna detalle o "" si decline o error.
func promptShellWrapper() (string, error) {
	_, rcfile := setuppkg.DetectShell()
	home, err := os.UserHomeDir()
	if err == nil {
		if strings.HasPrefix(rcfile, home) {
			rcfile = strings.Replace(rcfile, home, "~", 1)
		}
	}
	fmt.Fprintf(os.Stderr, "  ¿Querés agregar el wrapper de shell para opencode+domain a %s? [y/N] ", rcfile)
	line, err := readLine()
	if err != nil {
		return "", nil
	}
	line = strings.TrimSpace(strings.ToLower(line))
	if line != "y" && line != "yes" {
		return "", nil
	}
	return installShellWrapperStep()
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
	fmt.Println("  --agents LIST                   MCP agents to configure, csv (default: opencode; e.g. opencode,claude-code)")
	fmt.Println("  --email ADDR                    Email de la cuenta (first-run la crea; re-installs reciben OTP ahí)")
	fmt.Println("  --non-interactive, -y           Skip prompts (use defaults or flags)")
	fmt.Println("  --no-backup                     Skip automatic backups before mutations")
	fmt.Println("  --no-init                       Skip init (archiving .md to BD)")
	fmt.Println("  --no-service                    Skip systemd user service (server queda manual)")
	fmt.Println("  --with-wrapper                  Install shell wrapper for opencode+domain (auto-detect on open)")
	fmt.Println("  --no-opencode                   Remove opencode from --agents (compat)")
	fmt.Println("  --primary-memory                Detect and disable other memory MCP providers (engram, mem0, ...)")
	fmt.Println("                                  so domain queda como la única fuente de memoria visible al LLM")
	fmt.Println("  --primary-memory-yes            Skip confirm prompt para --primary-memory")
	fmt.Println("  --reactivate-memory             Restaurar providers desactivados (con --primary-memory) desde backup")
	fmt.Println("  --src PATH                      Project root path (default: cwd). Si cwd no es un root de repo")
	fmt.Println("                                  válido (faltan .env.example / docker-compose.yml), aborta con mensaje claro.")
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
