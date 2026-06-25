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
	OS string // "linux" | "darwin" | "windows"
}

func DetectPlatform() Platform {
	return Platform{OS: runtime.GOOS}
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
	ClaudeCodeMCP    string // ~/.claude/mcp_servers.json (todos los OS)
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
		ClaudeCodeMCP:   filepath.Join(home, ".claude", "mcp_servers.json"),
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
		{Name: "claude-code", MCPPath: paths.ClaudeCodeMCP, RootHint: filepath.Dir(filepath.Dir(paths.GlobalSkillPath))},
		{Name: "cursor", MCPPath: paths.CursorMCP, RootHint: filepath.Dir(paths.CursorMCP)},
		{Name: "cline", MCPPath: paths.ClineMCP, RootHint: filepath.Dir(paths.ClineMCP)},
		{Name: "continue", MCPPath: paths.ContinueMCP, RootHint: filepath.Dir(paths.ContinueMCP)},
		{Name: "claude-desktop", MCPPath: paths.ClaudeDesktopMCP, RootHint: filepath.Dir(paths.ClaudeDesktopMCP)},
		{Name: "opencode", MCPPath: paths.OpencodeMCP, RootHint: paths.OpencodeDir},
	}
	out := []Client{}
	for _, c := range candidates {


		if dirExists(c.RootHint) {
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
	RootHint string
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func commandExists(name string) bool {
	_, err := execLookPath(name)
	return err == nil
}
