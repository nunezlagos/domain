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
	GlobalEnv        string // ~/.config/domain/install.env (Linux/macOS) o %APPDATA%\domain\install.env (Windows)
	GlobalSkillPath  string // ~/.claude/skills/domain/SKILL.md (todos los OS)
	GlobalAgentPath  string // ~/.claude/agents/domain-memory.md
	ClaudeCodeMCP    string // ~/.claude.json (config global real que lee Claude Code; top-level "mcpServers")
	OpencodeMCP      string // ~/.config/opencode/opencode.json
	CursorMCP        string // ~/.cursor/mcp.json
	ClineMCP         string // path largo bajo VS Code data dir
	ContinueMCP      string // ~/.continue/config.json
	ClaudeDesktopMCP string // macOS: ~/Library/Application Support/Claude/claude_desktop_config.json; linux: ~/.config/Claude/...; windows: %APPDATA%\Claude\...
	OpencodeDir      string
	OpencodeSkillsLn string // ~/.config/opencode/skills/domain/SKILL.md (symlink al global)
	OpencodeAgentsLn string
}

func (p Platform) Paths() Paths {
	home := p.Home()
	configDir := filepath.Join(home, ".config")
	if p.OS == "windows" {
		configDir = p.AppData() // %APPDATA% sirve como "config dir" en Windows
	}

	out := Paths{
		GlobalEnv:       filepath.Join(configDir, "domain", "install.env"),
		GlobalSkillPath: filepath.Join(home, ".claude", "skills", "domain", "SKILL.md"),
		GlobalAgentPath: filepath.Join(home, ".claude", "agents", "domain-memory.md"),
		ClaudeCodeMCP:   filepath.Join(home, ".claude.json"),
		OpencodeDir:     filepath.Join(configDir, "opencode"),
		OpencodeMCP:     filepath.Join(configDir, "opencode", "opencode.json"),
		CursorMCP:       filepath.Join(home, ".cursor", "mcp.json"),
		ContinueMCP:     filepath.Join(home, ".continue", "config.json"),
	}
	out.OpencodeSkillsLn = filepath.Join(out.OpencodeDir, "skills", "domain", "SKILL.md")
	out.OpencodeAgentsLn = filepath.Join(out.OpencodeDir, "agents", "domain-memory.md")

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
	candidates := []Client{
		// claude-code: el config global vive en ~/.claude.json (file), pero el
		// dir ~/.claude (skills/agents/sessions) es el signal de instalación.
		// FileHint cubre el caso de un install fresco con .claude.json y sin dir.
		{Name: "claude-code", MCPPath: paths.ClaudeCodeMCP, RootHint: filepath.Dir(filepath.Dir(paths.GlobalSkillPath)), FileHint: paths.ClaudeCodeMCP},
		{Name: "cursor", MCPPath: paths.CursorMCP, RootHint: filepath.Dir(paths.CursorMCP)},
		{Name: "cline", MCPPath: paths.ClineMCP, RootHint: filepath.Dir(paths.ClineMCP)},
		{Name: "continue", MCPPath: paths.ContinueMCP, RootHint: filepath.Dir(paths.ContinueMCP)},
		{Name: "claude-desktop", MCPPath: paths.ClaudeDesktopMCP, RootHint: filepath.Dir(paths.ClaudeDesktopMCP)},
		{Name: "opencode", MCPPath: paths.OpencodeMCP, RootHint: paths.OpencodeDir},
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
		out = append(out, Client{Name: "opencode", MCPPath: paths.OpencodeMCP, RootHint: paths.OpencodeDir})
	}
	return out
}

type Client struct {
	Name     string
	MCPPath  string
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
