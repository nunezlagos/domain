package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func execLookPath(name string) (string, error) { return exec.LookPath(name) }

// Platform abstrae diferencias OS (paths de configs por cliente).
type Platform struct {
	OS     string // "linux" | "darwin" | "windows"
	Distro string // linux only: "arch" | "debian" | "ubuntu" | ""
}

func DetectPlatform() Platform {
	p := Platform{OS: runtime.GOOS}
	if p.OS == "linux" {
		p.Distro = detectDistro()
	}
	return p
}

// Home devuelve $HOME (Unix) o %USERPROFILE% (Windows). Falla si no se puede
// resolver (raro, pero el binario debe abortar limpio).
func (p Platform) Home() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}

	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	if h := os.Getenv("USERPROFILE"); h != "" {
		return h
	}
	return ""
}

// AppData devuelve %APPDATA% en Windows (~AppData/Roaming) o vacío en
// otros OS — algunos clientes lo usan (Claude Desktop, Cline).
func (p Platform) AppData() string {
	if p.OS != "windows" {
		return ""
	}
	if a := os.Getenv("APPDATA"); a != "" {
		return a
	}
	return filepath.Join(p.Home(), "AppData", "Roaming")
}

// LocalAppData = %LOCALAPPDATA% en Windows (~AppData/Local).
func (p Platform) LocalAppData() string {
	if p.OS != "windows" {
		return ""
	}
	if a := os.Getenv("LOCALAPPDATA"); a != "" {
		return a
	}
	return filepath.Join(p.Home(), "AppData", "Local")
}

// Paths agrupa las rutas de configs MCP de cada cliente para esta plataforma.
type Paths struct {
	GlobalEnv       string // ~/.config/domain/install.env (Linux/macOS) o %APPDATA%\domain\install.env (Windows)
	GlobalSkillPath string // ~/.claude/skills/domain/SKILL.md (todos los OS)
	// DOMAINSERV-137: directorio, no archivo — el catálogo tiene N agentes y el installer
	// ya no nombra ninguno.
	GlobalAgentsDir  string // ~/.claude/agents/
	ClaudeCodeMCP    string // ~/.claude.json (config global real que lee Claude Code; top-level "mcpServers")
	OpencodeMCP      string // ~/.config/opencode/opencode.json
	CursorMCP        string // ~/.cursor/mcp.json
	ClineMCP         string // path largo bajo VS Code data dir
	ContinueMCP      string // ~/.continue/config.json
	ClaudeDesktopMCP string // macOS: ~/Library/Application Support/Claude/claude_desktop_config.json; linux: ~/.config/Claude/...; windows: %APPDATA%\Claude\...
	OpencodeDir      string
	OpencodeSkillsLn string // ~/.config/opencode/skills/domain/SKILL.md (symlink al global)
	// Los agentes de OpenCode NO son symlinks: cada harness recibe su propio template
	// porque los esquemas de frontmatter son incompatibles (ver embed.go).
	OpencodeAgentsDir string // ~/.config/opencode/agents/
	// AgentHooksDir es donde viven los guards PreToolUse que un agente declare, junto a
	// los demás hooks de domain. El frontmatter no puede referenciarlos con ruta relativa:
	// se resuelve contra el cwd, así que el guard solo se encontraría si el proyecto actual
	// casualmente lo tuviera — y un guard que no se encuentra no bloquea nada.
	AgentHooksDir string // ~/.local/share/domain/hooks/
	// AgentsManifest registra el sha256 del template que domain escribió por agente. Sin ese
	// registro no se puede distinguir una edición del usuario de un template actualizado, y
	// quedan solo dos comportamientos, los dos malos: pisar siempre (se pierde la edición) o
	// no pisar nunca (el catálogo se congela y un fix de template no llega al cliente).
	AgentsManifest string // ~/.local/share/domain/agents-manifest.json
	// HooksManifest hace por los lifecycle hooks lo que AgentsManifest por los agentes, y por
	// la misma razón (DOMAINSERV-267). Sin él, instalarHookEmbebido solo sabía que el disco
	// DIFIERE del binario, no de quién es esa diferencia, así que elegía no pisar nunca: un
	// fix de hook no llegaba a ninguna máquina y el install lo reportaba como éxito.
	HooksManifest string // ~/.local/share/domain/hooks-manifest.json
}

// HooksDirDelSistema resuelve el directorio de los lifecycle hooks para los callers que no
// tienen un Paths a mano (DOMAINSERV-239).
//
// Existe para que nadie vuelva a armar esa ruta con filepath.Join: en Windows la base es
// %APPDATA% y no ~/.local/share, así que dos sitios que la construyan por su cuenta pueden
// quedar apuntando a lugares distintos. Cuando eso pasa entre el registro de settings.json y el
// chequeo del doctor, claudeHookGetMatcher no encuentra los hooks y el doctor reporta fallos
// críticos en una instalación sana. Hay un guard de fuente que lo impide.
func HooksDirDelSistema() string {
	return DetectPlatform().Paths().AgentHooksDir
}

func (p Platform) Paths() Paths {
	home := p.Home()
	configDir := filepath.Join(home, ".config")
	if p.OS == "windows" {
		configDir = p.AppData() // %APPDATA% sirve como "config dir" en Windows
	}

	dataDir := filepath.Join(home, ".local", "share")
	if p.OS == "windows" {
		dataDir = p.AppData()
	}

	out := Paths{
		GlobalEnv:       filepath.Join(configDir, "domain", "install.env"),
		GlobalSkillPath: filepath.Join(home, ".claude", "skills", "domain", "SKILL.md"),
		GlobalAgentsDir: filepath.Join(home, ".claude", "agents"),
		AgentHooksDir:   filepath.Join(dataDir, "domain", "hooks"),
		AgentsManifest:  filepath.Join(dataDir, "domain", "agents-manifest.json"),
		HooksManifest:   filepath.Join(dataDir, "domain", "hooks-manifest.json"),
		ClaudeCodeMCP:   filepath.Join(home, ".claude.json"),
		OpencodeDir:     filepath.Join(configDir, "opencode"),
		OpencodeMCP:     filepath.Join(configDir, "opencode", "opencode.json"),
		CursorMCP:       filepath.Join(home, ".cursor", "mcp.json"),
		ContinueMCP:     filepath.Join(home, ".continue", "config.json"),
	}
	out.OpencodeSkillsLn = filepath.Join(out.OpencodeDir, "skills", "domain", "SKILL.md")
	out.OpencodeAgentsDir = filepath.Join(out.OpencodeDir, "agents")

	switch p.OS {
	case "darwin":
		out.ClineMCP = filepath.Join(home, "Library", "Application Support", "Code",
			"User", "globalStorage", "saoudrizwan.claude-dev", "settings",
			"cline_mcp_settings.json")
		out.ClaudeDesktopMCP = filepath.Join(home, "Library", "Application Support",
			"Claude", "claude_desktop_config.json")
	case "linux":
		out.ClineMCP = filepath.Join(home, ".config", "Code", "User", "globalStorage",
			"saoudrizwan.claude-dev", "settings", "cline_mcp_settings.json")
		out.ClaudeDesktopMCP = filepath.Join(home, ".config", "Claude",
			"claude_desktop_config.json")
	case "windows":
		out.ClineMCP = filepath.Join(p.AppData(), "Code", "User", "globalStorage",
			"saoudrizwan.claude-dev", "settings", "cline_mcp_settings.json")
		out.ClaudeDesktopMCP = filepath.Join(p.AppData(), "Claude",
			"claude_desktop_config.json")
	}
	return out
}

// IsWSL reporta si estamos corriendo dentro de WSL (Linux con kernel WSL).
// Útil para el README/warning — no cambia lógica, pero informa al usuario.
func (p Platform) IsWSL() bool {
	if p.OS != "linux" {
		return false
	}
	b, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	v := strings.ToLower(string(b))
	return strings.Contains(v, "microsoft") || strings.Contains(v, "wsl")
}

// DetectedClients chequea qué clientes están instalados (paths existen).
// El return mantiene orden estable para output predecible.
func (p Platform) DetectedClients() []Client {
	paths := p.Paths()
	// EnvPath: directorio del cliente donde escribimos el .env con la API key.
	// Solo los 2 clientes oficialmente soportados tienen .env (los demas son legacy).
	// GlobalSkillPath = ~/.claude/skills/domain/SKILL.md → 3 Dir arriba da ~/.claude
	claudeEnv := filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(paths.GlobalSkillPath))), ".env") // ~/.claude/.env
	opencodeEnv := filepath.Join(filepath.Dir(paths.OpencodeMCP), ".env")                                // ~/.config/opencode/.env
	candidates := []Client{
		// claude-code: el config global vive en ~/.claude.json (file), pero el
		// dir ~/.claude (skills/agents/sessions) es el signal de instalación.
		// FileHint cubre el caso de un install fresco con .claude.json y sin dir.
		{Name: "claude-code", MCPPath: paths.ClaudeCodeMCP, EnvPath: claudeEnv, RootHint: filepath.Dir(filepath.Dir(paths.GlobalSkillPath)), FileHint: paths.ClaudeCodeMCP},
		{Name: "cursor", MCPPath: paths.CursorMCP, RootHint: filepath.Dir(paths.CursorMCP)},
		{Name: "cline", MCPPath: paths.ClineMCP, RootHint: filepath.Dir(paths.ClineMCP)},
		{Name: "continue", MCPPath: paths.ContinueMCP, RootHint: filepath.Dir(paths.ContinueMCP)},
		{Name: "claude-desktop", MCPPath: paths.ClaudeDesktopMCP, RootHint: filepath.Dir(paths.ClaudeDesktopMCP)},
		{Name: "opencode", MCPPath: paths.OpencodeMCP, EnvPath: opencodeEnv, RootHint: paths.OpencodeDir},
	}
	out := []Client{}
	for _, c := range candidates {
		if dirExists(c.RootHint) || (c.FileHint != "" && fileExists(c.FileHint)) {
			out = append(out, c)
		}
	}

	hasOpencode := false
	for _, c := range out {
		if c.Name == "opencode" {
			hasOpencode = true
			break
		}
	}
	if !hasOpencode && commandExists("opencode") {
		out = append(out, Client{Name: "opencode", MCPPath: paths.OpencodeMCP, EnvPath: opencodeEnv, RootHint: paths.OpencodeDir})
	}
	return out
}

type Client struct {
	Name     string
	MCPPath  string
	EnvPath  string // .env del cliente (donde se persiste la API key). "" si no aplica.
	RootHint string // dir cuya existencia indica que el cliente está instalado
	FileHint string // archivo alternativo que también indica instalación (opcional)
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func commandExists(name string) bool {
	_, err := execLookPath(name)
	return err == nil
}

// detectDistro lee /etc/os-release y devuelve el ID (lowercased).
// Vacío si no se puede parsear.
func detectDistro() string {
	return detectDistroFromFile("/etc/os-release")
}

// detectDistroFromFile lee un archivo os-release-formatted y devuelve
// el campo ID. testeable con t.TempDir.
func detectDistroFromFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ID=") {
			id := strings.TrimPrefix(line, "ID=")
			id = strings.Trim(id, `"`)
			return strings.ToLower(id)
		}
	}
	return ""
}

// FindOpencode localiza el binario opencode. Orden:
//  1. PATH (exec.LookPath)
//  2. Paths comunes por OS (npm-global, homebrew, etc.)
//
// Retorna path absoluto o error "opencode not found ...".
func FindOpencode() (string, error) {
	if p, err := execLookPath("opencode"); err == nil {
		return p, nil
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		home = os.Getenv("HOME")
	}
	candidates := candidateOpencodePaths(home)
	for _, c := range candidates {
		if fileExists(c) {
			return c, nil
		}
	}
	return "", &notFoundError{}
}

// candidateOpencodePaths devuelve paths comunes donde opencode podría
// estar instalado fuera de PATH. Orden = prioridad.
func candidateOpencodePaths(home string) []string {
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/opt/homebrew/bin/opencode" + suffix,
			"/usr/local/bin/opencode" + suffix,
			filepath.Join(home, ".npm-global", "bin", "opencode"+suffix),
			filepath.Join(home, ".local", "bin", "opencode"+suffix),
		}
	case "windows":
		local := os.Getenv("LOCALAPPDATA")
		if local == "" && home != "" {
			local = filepath.Join(home, "AppData", "Local")
		}
		return []string{
			filepath.Join(local, "Programs", "opencode", "opencode"+suffix),
			filepath.Join(local, "Microsoft", "WindowsApps", "opencode"+suffix),
			filepath.Join(home, ".npm-global", "bin", "opencode"+suffix),
		}
	default:
		return []string{
			filepath.Join(home, ".npm-global", "bin", "opencode"+suffix),
			filepath.Join(home, ".local", "bin", "opencode"+suffix),
			"/usr/local/bin/opencode" + suffix,
			"/usr/bin/opencode" + suffix,
		}
	}
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

type notFoundError struct{}

func (e *notFoundError) Error() string {
	return "opencode not found in PATH ni en paths comunes. " +
		"Instalá opencode (https://opencode.ai) o usá --install-opencode"
}

// InstallCmd es un comando de instalación con fallback opcional.
type InstallCmd struct {
	Primary  []string // argv nativo del OS (pacman/brew/winget)
	Fallback []string // argv a usar si Primary falla (npm install -g)
}

// String devuelve la representación "Primary|Fallback" para logs.
func (c InstallCmd) String() string {
	return joinCmd(c.Primary) + "|" + joinCmd(c.Fallback)
}

func joinCmd(argv []string) string {
	if len(argv) == 0 {
		return ""
	}
	out := argv[0]
	for _, a := range argv[1:] {
		out += " " + a
	}
	return out
}

// InstallOpencodeCmd devuelve Primary + Fallback según OS/Distro.
// Primary solo se setea cuando el package manager nativo es seguro de asumir
// (pacman en Arch, brew en macOS, winget en Windows). En Linux no-Arch
// (Debian/Ubuntu/Fedora/etc) salta directo al fallback npm porque no
// podemos saber si el binario opencode-ai está en los repos oficiales.
func InstallOpencodeCmd(p Platform) InstallCmd {
	fallback := []string{"npm", "install", "-g", "opencode-ai@latest"}
	switch p.OS {
	case "linux":
		if p.Distro == "arch" {
			return InstallCmd{
				Primary:  []string{"pacman", "-S", "--needed", "--noconfirm", "opencode-ai"},
				Fallback: fallback,
			}
		}
	case "darwin":
		return InstallCmd{
			Primary:  []string{"brew", "install", "opencode-ai"},
			Fallback: fallback,
		}
	case "windows":
		return InstallCmd{
			Primary:  []string{"winget", "install", "--id=opencode.opencode", "-e"},
			Fallback: fallback,
		}
	}
	return InstallCmd{Fallback: fallback}
}
