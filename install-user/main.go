// domain-install — instalador cross-platform para el cliente MCP domain.
//
// Reemplaza al script bash en plataformas donde bash no aplica (Windows
// nativo). En Linux/macOS/WSL2 funciona igual y mejor (paths absolutos,
// JSON parsing strict, sin dependencias externas como jq).
//
// Lo que hace (paridad con install-user.sh):
//   - detecta clientes MCP instalados (claude-code, opencode, cursor, cline,
//     continue, claude-desktop) por presencia de paths
//   - escribe el config MCP de domain-mcp en cada uno, preservando otros
//     servers que el usuario haya configurado y migrando entry legacy
//     "domain" si existía
//   - planta skill global (~/.claude/skills/domain/SKILL.md) y subagent
//     (~/.claude/agents/domain-memory.md) — mismos contenidos para todos
//     los clientes, embebidos en el binario
//   - opencode comparte vía symlink (Linux/macOS) o copia (Windows)
//   - persiste VPS_URL + email en ~/.config/domain/install.env (modo 0600)
//     para no re-preguntar en re-ejecuciones
//   - --uninstall: borra solo lo que el installer creó, preserva el resto
//     del archivo del usuario (operación determinista, no restore de backup)
package main

import (
	"bufio"
	"flag"
	"fmt"
	"net/http"
	"os"
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
		vpsURL    string
		email     string
		apiKey    string
		uninstall bool
		dryRun    bool
	)
	flag.StringVar(&vpsURL, "url", "", "URL del VPS (ej. http://1.2.3.4)")
	flag.StringVar(&email, "email", "", "Email del usuario")
	flag.StringVar(&apiKey, "api-key", "", "API key domk_*")
	flag.BoolVar(&uninstall, "uninstall", false, "Desinstala domain-mcp de los clientes")
	flag.BoolVar(&dryRun, "dry-run", false, "Solo detecta clientes, no toca configs")
	flag.Usage = printHelp
	flag.Parse()

	platform := DetectPlatform()
	paths := platform.Paths()

	if uninstall {
		runUninstall(platform, paths)
		return
	}

	runInstall(platform, paths, vpsURL, email, apiKey, dryRun)
}

func printHelp() {
	fmt.Println(`domain-install — instalador cross-platform del cliente MCP domain.

Uso:
  domain-install                                    # interactive (pide URL/email/api-key)
  domain-install --url http://1.2.3.4 \
                 --email u@x.cl \
                 --api-key domk_live_xxx
  domain-install --uninstall                        # deshacer: deja todo como estaba
  domain-install --dry-run                          # solo detecta clientes

Plataformas: Linux (Ubuntu/Debian/Arch), macOS (Intel + Apple Silicon),
Windows (nativo), WSL2.

Re-ejecutable. VPS_URL y email se persisten en ~/.config/domain/install.env
(o %APPDATA%\domain\install.env en Windows) para no re-preguntar.

API_KEY NUNCA se persiste — solo vive en los configs MCP de cada cliente.`)
}

func runInstall(p Platform, paths Paths, urlFlag, emailFlag, apiKeyFlag string, dryRun bool) {
	step("domain-install — install user")

	if p.IsWSL() {
		info("detectado WSL2 — instalando para clientes IDE corriendo en WSL")
	}


	env, _ := loadEnv(paths.GlobalEnv)
	if urlFlag == "" {
		urlFlag = env.VPSURL
	}
	if emailFlag == "" {
		emailFlag = env.Email
	}
	if urlFlag != "" {
		ok("URL del VPS (desde " + paths.GlobalEnv + "): " + urlFlag)
	}
	if emailFlag != "" {
		ok("Email (desde " + paths.GlobalEnv + "): " + emailFlag)
	}


	in := bufio.NewReader(os.Stdin)
	if urlFlag == "" {
		urlFlag = strings.TrimSpace(prompt(in, "  URL del VPS (ej. http://1.2.3.4): "))
	}
	if emailFlag == "" {
		emailFlag = strings.TrimSpace(prompt(in, "  Email: "))
	}
	if apiKeyFlag == "" {
		apiKeyFlag = strings.TrimSpace(promptHidden(in, "  API key: "))
	}

	if urlFlag == "" {
		failL("URL del VPS requerida")
		os.Exit(1)
	}
	if emailFlag == "" {
		failL("Email requerido")
		os.Exit(1)
	}
	if apiKeyFlag == "" {
		failL("API key requerida")
		os.Exit(1)
	}
	urlFlag = strings.TrimRight(urlFlag, "/")

	if err := saveEnv(paths.GlobalEnv, EnvData{VPSURL: urlFlag, Email: emailFlag}); err != nil {
		warnL("no se pudo guardar " + paths.GlobalEnv + ": " + err.Error())
	} else {
		ok("guardado en " + paths.GlobalEnv + " (modo 0600)")
	}


	step("Verificando conexión al VPS")
	if pingVPS(urlFlag) {
		ok("VPS responde en " + urlFlag)
	} else {
		warnL("VPS no responde en " + urlFlag + "/healthz (continuando igual)")
	}


	step("Detectando clientes MCP")
	clients := p.DetectedClients()
	if len(clients) == 0 {
		failL("Ningún cliente MCP detectado.")
		failL("Soportados: claude-code, opencode, cursor, cline, continue, claude-desktop")
		os.Exit(1)
	}
	for _, c := range clients {
		ok(c.Name)
	}
	if dryRun {
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


	step("Configurando clientes (MCP transport)")
	timestamp := Timestamp()
	for _, c := range clients {
		if err := configureClient(c, urlFlag, apiKeyFlag, timestamp); err != nil {
			failL(c.Name + ": " + err.Error())
			continue
		}
		ok(c.Name + ": " + c.MCPPath)
		if c.Name == "opencode" {
			if err := linkOpencodeToGlobal(paths, p.OS); err != nil {
				warnL("opencode symlinks: " + err.Error())
			} else {
				ok("opencode skill/agent: " + linkVerb(p.OS) + " a globales")
			}
		}
	}

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
		cGreen, cBold, cReset, urlFlag, emailFlag,
		paths.GlobalSkillPath, paths.GlobalAgentPath)
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

// promptHidden: en stdin sin tty fallback a ReadString. Para TTY usaríamos
// golang.org/x/term, pero queremos stdlib-only.
func promptHidden(r *bufio.Reader, q string) string {
	fmt.Print(q)
	s, _ := r.ReadString('\n')
	return s
}

func linkVerb(osName string) string {
	if osName == "windows" {
		return "copias"
	}
	return "symlinks"
}

func pingVPS(url string) bool {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url + "/healthz")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 500
}
