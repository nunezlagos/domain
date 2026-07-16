// domain-install — instalador cross-platform para el cliente MCP domain.
//
// Reemplaza al script bash en plataformas donde bash no aplica (Windows
// nativo). En Linux/macOS/WSL2 funciona igual y mejor (paths absolutos,
// JSON parsing strict, sin dependencias externas como jq).
//
// Lo que hace:
//   - detecta clientes MCP instalados (claude-code, opencode, cursor, cline,
//     continue, claude-desktop) por presencia de paths
//   - si no hay ninguno Y --install-opencode: sugiere el comando para instalar
//     opencode y aborta pidiendo confirmación humana
//   - --target: filtra a un cliente específico (opencode|claude-code|...)
//   - escribe el config MCP de domain-mcp en cada uno, preservando otros
//     servers y migrando entry legacy "domain" REMOTA si existía
//   - planta skill global (~/.claude/skills/domain/SKILL.md) y subagent
//     (~/.claude/agents/domain-memory.md) — embebidos en el binario
//   - opencode comparte vía symlink (Linux/macOS) o copia (Windows)
//   - persiste VPS_URL + email en ~/.config/domain/install.env (modo 0600)
//     para no re-preguntar en re-ejecuciones
//   - si install.env ya tiene URL distinta, avisa antes de sobrescribir
//   - --uninstall: borra solo lo que el installer creó, preserva el resto
//     del archivo del usuario (operación determinista, no restore de backup)
//
// Convención de entry names (documentada y deliberada):
//   - "domain"     → instalación LOCAL (instalador del SERVER: cmd/domain +
//     internal/cli/setup). transport=local, command=binario
//     domain-mcp. Vive en ~/.claude.json y opencode.json.
//   - "domain-mcp" → instalación REMOTA (ESTE instalador de usuario).
//     transport=http/remote, url=VPS/mcp + Bearer api-key.
//
// Dedup local↔remoto: si ya hay un "domain" LOCAL vivo, NO se agrega un
// "domain-mcp" remoto contradictorio (la instalación local es la fuente de
// verdad y se respeta). El uninstall del user nunca toca un "domain" local.
//
// Config MCP que lee cada cliente (verificado):
//   - claude-code:    ~/.claude.json           top-level "mcpServers" (type:http)
//   - opencode:       ~/.config/opencode/opencode.json  "mcp" (type:remote)
//   - cursor:         ~/.cursor/mcp.json        "mcpServers"
//   - cline:          .../saoudrizwan.claude-dev/settings/cline_mcp_settings.json  "mcpServers"
//   - continue:       ~/.continue/config.json   experimental.modelContextProtocolServers
//   - claude-desktop: NO soporta http remoto (solo stdio) → se omite con aviso
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	cReset  = "\033[0m"
	cBold   = "\033[1m"
	cGreen  = "\033[32m"
	cYellow = "\033[33m"
	cRed    = "\033[31m"
	cDim    = "\033[2m"
)

func step(s string)  { fmt.Printf("\n%s==> %s%s\n", cBold, s, cReset) }
func ok(s string)    { fmt.Printf("%s    ✓%s %s\n", cGreen, cReset, s) }
func warnL(s string) { fmt.Printf("%s    !%s %s\n", cYellow, cReset, s) }
func failL(s string) { fmt.Fprintf(os.Stderr, "%s    ✗%s %s\n", cRed, cReset, s) }
func info(s string)  { fmt.Printf("%s    ·%s %s\n", cDim, cReset, s) }

func main() {
	var (
		vpsURL          string
		email           string
		apiKey          string
		uninstall       bool
		dryRun          bool
		target          string
		installOpencode bool
		bootstrapGuided bool
		yes             bool
		keepLocalRules  bool
		removeEngram    bool
	)
	flag.StringVar(&vpsURL, "url", "", "URL del VPS (ej. http://1.2.3.4)")
	flag.StringVar(&email, "email", "", "Email del usuario")
	flag.StringVar(&apiKey, "api-key", "", "API key domk_* (si ya la tenés)")
	flag.BoolVar(&uninstall, "uninstall", false, "Desinstala domain-mcp de los clientes")
	flag.BoolVar(&dryRun, "dry-run", false, "Solo detecta clientes, no toca configs")
	flag.StringVar(&target, "target", "", "Cliente único (opencode|claude-code|...)")
	flag.BoolVar(&installOpencode, "install-opencode", false,
		"Si no hay clientes MCP, sugerir el comando para instalar opencode")
	flag.BoolVar(&bootstrapGuided, "bootstrap", false,
		"Modo guiado: el operador genera la API key con `domain bootstrap` y la pega")
	flag.BoolVar(&yes, "yes", false, "Asumir 'sí' a prompts no destructivos")
	flag.BoolVar(&keepLocalRules, "keep-local-rules", false,
		"NO neutralizar instrucciones locales de proyecto (CLAUDE.md/AGENTS.md/.claude). Por default domain las excluye para que solo apliquen las reglas globales")
	flag.BoolVar(&removeEngram, "remove-engram", false,
		"Deshabilita el plugin engram si está activo (sistema de memoria legacy, reemplazado por domain)")
	flag.Usage = printHelp
	flag.Parse()

	platform := DetectPlatform()
	paths := platform.Paths()

	if uninstall {
		runUninstall(platform, paths)
		return
	}

	if bootstrapGuided {
		runBootstrapGuided(&platform, paths, keepLocalRules)
		return
	}

	runInstall(platform, paths, installOptions{
		URL:            vpsURL,
		Email:          email,
		APIKey:         apiKey,
		DryRun:         dryRun,
		Target:         target,
		AutoInstall:    installOpencode,
		NonInteractive: yes,
		KeepLocalRules: keepLocalRules,
		RemoveEngram:   removeEngram,
	})
}

func printHelp() {
	fmt.Println(`domain-install — instalador cross-platform del cliente MCP domain.

Uso:
  domain-install                                          # interactive
  domain-install --url http://1.2.3.4 \
                  --email u@x.cl \
                  --api-key domk_live_xxx
  domain-install --bootstrap                              # guiado: te ayuda a obtener la key
  domain-install --target opencode                        # solo configura opencode
  domain-install --install-opencode                       # si no hay clientes, sugiere instalar
  domain-install --remove-engram                          # deshabilita plugin engram (memoria legacy)
  domain-install --uninstall                              # deshacer
  domain-install --dry-run                                # solo detectar

Plataformas: Linux (Ubuntu/Debian/Arch), macOS (Intel + Apple Silicon),
Windows (nativo), WSL2.

Re-ejecutable. VPS_URL y email se persisten en ~/.config/domain/install.env
(o %APPDATA%\domain\install.env en Windows) para no re-preguntar.

API_KEY NUNCA se persiste — solo vive en los configs MCP de cada cliente.

Para usar por primera vez:
  1. Operador: ssh vps-domain "cd /path/to/services && domain bootstrap --email u@x.cl"
     (devuelve API key en stdout)
  2. Vos: domain-install --url http://vps --email u@x.cl --api-key domk_live_xxx
  3. Reiniciá tus clientes MCP`)
}

type installOptions struct {
	URL            string
	Email          string
	APIKey         string
	DryRun         bool
	Target         string
	AutoInstall    bool
	NonInteractive bool
	KeepLocalRules bool // --keep-local-rules: NO neutralizar instrucciones locales (issue-54.1)
	RemoveEngram   bool // --remove-engram: deshabilita plugin engram si está activo
}

func runInstall(p Platform, paths Paths, opts installOptions) {
	step("domain-install — install user")

	if p.IsWSL() {
		info("detectado WSL2 — instalando para clientes IDE corriendo en WSL")
	}

	env, _ := loadEnv(paths.GlobalEnv)
	if opts.URL == "" {
		opts.URL = env.VPSURL
	}
	if opts.Email == "" {
		opts.Email = env.Email
	}
	if opts.URL != "" {
		ok("URL del VPS (desde " + paths.GlobalEnv + "): " + opts.URL)
	}
	if opts.Email != "" {
		ok("Email (desde " + paths.GlobalEnv + "): " + opts.Email)
	}

	in := bufio.NewReader(os.Stdin)

	if opts.URL == "" {
		opts.URL = strings.TrimSpace(prompt(in, "  URL del VPS (ej. http://1.2.3.4): "))
	}
	if opts.Email == "" {
		opts.Email = strings.TrimSpace(prompt(in, "  Email: "))
	}

	// API key: prioridad flag --api-key > configs existentes (opencode/claudecode) > prompt.
	// En re-install, esto preserva la key sin que el usuario tenga que pegarla de nuevo.
	// Si opts.URL != "", resolveAPIKey valida la key contra el server (ver keyextract.go).
	if !opts.DryRun && opts.APIKey == "" {
		apiKey, src, err := resolveAPIKey(paths.OpencodeMCP, paths.ClaudeCodeMCP, "", opts.URL, in, opts.NonInteractive)
		if err != nil {
			failL(err.Error())
			os.Exit(1)
		}
		opts.APIKey = apiKey
		ok("API key (de " + src + ")")
	}

	if opts.URL == "" {
		failL("URL del VPS requerida")
		os.Exit(1)
	}
	if opts.Email == "" {
		failL("Email requerido")
		os.Exit(1)
	}
	if !opts.DryRun && opts.APIKey == "" {
		failL("API key requerida. Usá --bootstrap para modo guiado o corré 'domain bootstrap --email ...' en el VPS primero.")
		os.Exit(1)
	}
	opts.URL = strings.TrimRight(opts.URL, "/")

	if warning, _ := detectURLMismatch(paths.GlobalEnv, opts.URL); warning != "" {
		if !opts.DryRun && !opts.NonInteractive && !confirm(in, "  "+warning+" ") {
			failL("abortado por el usuario")
			os.Exit(1)
		}
		warnL("re-apuntando install.env a " + opts.URL)
	}

	if !opts.DryRun {
		if err := saveEnv(paths.GlobalEnv, EnvData{VPSURL: opts.URL, Email: opts.Email}); err != nil {
			warnL("no se pudo guardar " + paths.GlobalEnv + ": " + err.Error())
		} else {
			ok("guardado en " + paths.GlobalEnv + " (modo 0600)")
		}
	} else {
		info("DRY-RUN: no modifico " + paths.GlobalEnv)
	}

	step("Verificando conexión al VPS")
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 6*time.Second)
	if err := pingVPS(pingCtx, opts.URL); err != nil {
		warnL("VPS no responde en " + opts.URL + "/healthz: " + err.Error() + " (continuando igual)")
	} else {
		ok("VPS responde en " + opts.URL)
	}
	pingCancel()

	step("Detectando clientes MCP")
	plan, err := BuildInstallPlan(InstallOptions{
		Target:              opts.Target,
		AutoInstallOpencode: opts.AutoInstall,
		NonInteractive:      opts.NonInteractive,
	})
	if err != nil {
		failL(err.Error())
		os.Exit(1)
	}

	if plan.NeedsOpencode {
		failL("Ningún cliente MCP detectado.")
		failL("Soportados: claude-code, opencode, cursor, cline, continue, claude-desktop")
		if cmd := plan.OpencodeCmd.Primary; len(cmd) > 0 {
			info("para instalar opencode en este OS:")
			info("  " + joinCmd(cmd))
		} else {
			info("para instalar opencode via npm:")
			info("  " + joinCmd(plan.OpencodeCmd.Fallback))
		}
		failL("volvé a correr con --install-opencode y el operador corre el comando sugerido,")
		failL("o instalá manualmente desde https://opencode.ai")
		os.Exit(1)
	}

	for _, c := range plan.Targets {
		ok(c.Name)
	}

	if opts.DryRun {
		warnL("DRY-RUN: terminando sin tocar configs")
		return
	}

	step("Plantando skill + subagent globales")
	if err := installGlobalAssets(paths); err != nil {
		failL("install global assets: " + err.Error())
		os.Exit(1)
	}
	ok("skill: " + paths.GlobalSkillPath)
	ok("agent: " + paths.GlobalAgentPath)

	// 5b. Precedencia global en ~/.claude/CLAUDE.md (+ instruction de opencode)
	step("Escribiendo precedencia global de domain")
	if err := installGlobalInstructions(paths, p.Home(), Timestamp()); err != nil {
		warnL("precedencia global: " + err.Error())
	} else {
		ok("CLAUDE.md global: " + claudeGlobalPath(p.Home()))
	}

	// 5c. Domain global-first agresivo (issue-54.1): neutralizar instrucciones
	// locales de proyecto vía claudeMdExcludes, salvo --keep-local-rules.
	step("Neutralizando instrucciones locales (claudeMdExcludes)")
	if err := installClaudeMdExcludes(p.Home(), Timestamp(), opts.KeepLocalRules); err != nil {
		warnL("claudeMdExcludes: " + err.Error())
	} else if opts.KeepLocalRules {
		info("--keep-local-rules: se conservan las instrucciones locales de proyecto")
	} else {
		ok("claudeMdExcludes global: " + claudeSettingsPath(p.Home()))
	}

	// 5d. Allowlist de domain (DOMAINSERV-35): permissions.allow para que el
	// protocolo no dependa del clasificador del permission mode "auto".
	step("Allowlisteando domain en permissions.allow")
	if err := installClaudePermissions(p.Home(), Timestamp()); err != nil {
		warnL("permissions.allow: " + err.Error())
	} else {
		ok("permissions.allow global: " + claudeSettingsPath(p.Home()))
	}

	step("Configurando clientes (MCP transport)")
	results, err := Apply(plan, opts.URL, opts.APIKey)
	if err != nil {
		failL(err.Error())
		os.Exit(1)
	}
	skipByClient := map[string]ApplyResult{}
	for _, r := range results {
		skipByClient[r.Client] = r
	}
	for _, c := range plan.Targets {
		if r, found := skipByClient[c.Name]; found && r.Skipped {
			warnL(c.Name + ": omitido — " + r.Reason)
		} else {
			ok(c.Name + ": " + c.MCPPath)
		}
		if c.Name == "opencode" {
			if err := linkOpencodeToGlobal(paths, p.OS); err != nil {
				warnL("opencode symlinks: " + err.Error())
			} else {
				ok("opencode skill/agent: " + linkVerb(p.OS) + " a globales")
			}
		}
	}

	step("Detectando sistemas de memoria legacy (engram)")
	maybeRemoveEngram(p.Home(), opts, in)

	step("Instalando hook de SessionStart (Claude Code)")
	installClaudeSessionStartHook()

	step("Listo")
	fmt.Printf(`
  %s%sdomain MCP configurado%s

  VPS:    %s
  Email:  %s

  Archivos en disco (totales en este sistema):
    · %s
    · %s
    · 1 archivo de config MCP por cliente detectado (transport-only)

  Protocolo de uso: vive en BD como policy 'agent-protocol' (editable
  con domain_policy_update). El MCP server lo inyecta en cada
  initialize via instructions; no hay archivos rules sueltos.

  Próximos pasos:
    1. Reiniciá tus clientes MCP.
    2. Mandá un mensaje al LLM → debe llamar domain_session_bootstrap
       y usar tools domain_*.

  Para desinstalar:  domain-install --uninstall

`,
		cGreen, cBold, cReset, opts.URL, opts.Email,
		paths.GlobalSkillPath, paths.GlobalAgentPath)
}

// runBootstrapGuided: el operador ya generó la key con `domain bootstrap`
// en el VPS, ahora solo necesita pegarla. La URL la pedimos, la key la pide.
func runBootstrapGuided(p *Platform, paths Paths, keepLocalRules bool) {
	step("domain-install — modo bootstrap guiado")

	in := bufio.NewReader(os.Stdin)

	env, _ := loadEnv(paths.GlobalEnv)
	url := env.VPSURL
	if url == "" {
		url = strings.TrimSpace(prompt(in, "  URL del VPS (ej. http://1.2.3.4): "))
	} else {
		ok("URL del VPS (desde " + paths.GlobalEnv + "): " + url)
	}
	if url == "" {
		failL("URL requerida")
		os.Exit(1)
	}
	url = strings.TrimRight(url, "/")

	fmt.Printf(`
  Pasos para obtener la API key:

    1. Conectate al VPS:
         ssh vps-domain
    2. Corré en el VPS:
         cd /path/to/services && domain bootstrap --email tu@email.cl
       (la key aparece en stdout)
    3. Pegala abajo:

`)
	key := strings.TrimSpace(promptHidden(in, "  API key: "))
	if key == "" {
		failL("API key requerida")
		os.Exit(1)
	}
	email := env.Email
	if email == "" {
		email = strings.TrimSpace(prompt(in, "  Email: "))
	}

	if err := saveEnv(paths.GlobalEnv, EnvData{VPSURL: url, Email: email}); err != nil {
		warnL("no se pudo guardar install.env: " + err.Error())
	} else {
		ok("install.env actualizado")
	}

	runInstall(*p, paths, installOptions{
		URL:            url,
		Email:          email,
		APIKey:         key,
		KeepLocalRules: keepLocalRules,
	})
}

func runUninstall(p Platform, paths Paths) {
	step("Desinstalando domain MCP")
	clients := p.DetectedClients()
	for _, c := range clients {
		removed, err := uninstallClient(c)
		if err != nil {
			failL(c.Name + ": " + err.Error())
			continue
		}
		if removed {
			ok(c.Name + ": entries domain/domain-mcp removidas (resto del archivo intacto)")
		} else {
			info(c.Name + ": sin entry domain en el config")
		}
		if c.Name == "opencode" {
			removeOpencodeLinks(paths)
			ok("opencode: symlinks/copies limpiados")
		}
	}
	removeGlobalAssets(paths)
	ok("skill + agent globales removidos")
	removeEnv(paths.GlobalEnv)
	ok(".env global removido")

	step("Listo")
	fmt.Println("  Reiniciá tus clientes MCP. Backups *.backup-<timestamp> quedan en disco si necesitás revertir manualmente.")
}

func prompt(r *bufio.Reader, q string) string {
	fmt.Print(q)
	s, _ := r.ReadString('\n')
	return s
}

// promptHidden pide un secreto sin eco en terminal cuando es posible
// (Unix: stty -echo; best-effort). Si no hay TTY o falla, cae al modo visible
// para no romper pipes/CI. Stdlib puro + 'stty' (presente en Unix).
func promptHidden(r *bufio.Reader, q string) string {
	fmt.Print(q)
	restore := disableEcho()
	s, _ := r.ReadString('\n')
	if restore != nil {
		restore()
		fmt.Println() // newline que el eco habría producido al presionar Enter
	}
	return s
}

// confirm devuelve true para 'y'/'yes' y, dado que los prompts usan "(Y/n)"
// (default afirmativo), también para input vacío (Enter). Solo 'n'/'no' o
// cualquier otra respuesta explícita niega.
func confirm(r *bufio.Reader, q string) bool {
	fmt.Print(q)
	s, _ := r.ReadString('\n')
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return true // default-yes, consistente con "(Y/n)"
	}
	return s == "y" || s == "yes"
}

func linkVerb(osName string) string {
	if osName == "windows" {
		return "copias"
	}
	return "symlinks"
}

// disableEcho apaga el eco de la terminal (Unix) para leer secretos sin
// mostrarlos. Devuelve una función para restaurar el estado, o nil si no se
// pudo (sin TTY, no-Unix, falta 'stty'): en ese caso el caller lee en claro,
// que es preferible a romper el flujo en CI/pipes.
func disableEcho() func() {
	if runtime.GOOS == "windows" {
		return nil // Windows: sin soporte stdlib-only; lectura visible
	}
	if !isTTY() {
		return nil // pipe/redirección: no hay terminal que silenciar
	}
	// Guardar estado actual y apagar eco.
	saved, err := sttyState()
	if err != nil {
		return nil
	}
	if err := sttyApply("-echo"); err != nil {
		return nil
	}
	return func() { _ = sttyApply(saved) }
}

func isTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func sttyState() (string, error) {
	cmd := exec.Command("stty", "-g")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func sttyApply(arg string) error {
	cmd := exec.Command("stty", arg)
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
