// Package setup — issue-12.5 wizard CLI para configurar agentes externos
// (Claude Code, OpenCode, etc.) para usar domain-mcp como servidor MCP.
//
// Versión inicial: solo Claude Desktop (claude_desktop_config.json).
// El wizard:
//   1. Detecta el path del config según OS
//   2. Lee config existente o crea uno nuevo
//   3. Inserta server "domain" apuntando al binario domain-mcp
//   4. Escribe back con backup automático
//   5. Crea .ai/directives.md en cwd con instrucciones de uso
//
// SEGURIDAD: NUNCA modifica .env, .git/, *.pem, *.key, credentials.*
package setup

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

var (
	ErrAlreadyConfigured = errors.New("domain server already configured")
	ErrUnsupportedAgent  = errors.New("agent not supported")
)

// ClaudeConfig representa la estructura mínima de claude_desktop_config.json.
type ClaudeConfig struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

type MCPServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// ClaudeDesktopConfigPath retorna la ruta esperada del config según OS.
func ClaudeDesktopConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json"), nil
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), "Claude", "claude_desktop_config.json"), nil
	default: // linux + others
		return filepath.Join(home, ".config", "Claude", "claude_desktop_config.json"), nil
	}
}

// SetupClaudeDesktop agrega el server "domain" al config de Claude Desktop.
//
// Args:
//   - mcpBinaryPath: ruta absoluta al binario domain-mcp
//   - apiKey: DOMAIN_API_KEY a pasar como env var (opcional, puede estar en env shell)
//   - baseURL: DOMAIN_BASE_URL (opcional)
//
// Si el server "domain" ya está registrado → ErrAlreadyConfigured.
// Hace backup del config previo con timestamp.
func SetupClaudeDesktop(mcpBinaryPath, apiKey, baseURL string) (string, error) {
	configPath, err := ClaudeDesktopConfigPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return "", fmt.Errorf("mkdir config dir: %w", err)
	}

	cfg := ClaudeConfig{MCPServers: map[string]MCPServerConfig{}}
	if data, err := os.ReadFile(configPath); err == nil {
		// Config existente: parsear. Si parseo falla, no sobrescribimos.
		if err := json.Unmarshal(data, &cfg); err != nil {
			return "", fmt.Errorf("config existente inválido: %w", err)
		}
		if cfg.MCPServers == nil {
			cfg.MCPServers = map[string]MCPServerConfig{}
		}
		if _, ok := cfg.MCPServers["domain"]; ok {
			return configPath, ErrAlreadyConfigured
		}
		// Backup con timestamp.
		backupPath := configPath + ".backup-" + time.Now().UTC().Format("20060102T150405Z")
		if err := os.WriteFile(backupPath, data, 0o600); err != nil {
			return "", fmt.Errorf("backup: %w", err)
		}
	}

	env := map[string]string{}
	if apiKey != "" {
		env["DOMAIN_API_KEY"] = apiKey
	}
	if baseURL != "" {
		env["DOMAIN_BASE_URL"] = baseURL
	}
	cfg.MCPServers["domain"] = MCPServerConfig{
		Command: mcpBinaryPath,
		Args:    []string{},
		Env:     env,
	}

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}
	if err := os.WriteFile(configPath, out, 0o600); err != nil {
		return "", fmt.Errorf("write config: %w", err)
	}
	return configPath, nil
}

// CreateAIDirectives crea (si no existe) un .ai/directives.md en cwd con
// instrucciones para que el agente use tools Domain.
func CreateAIDirectives(cwd string) (string, error) {
	aiDir := filepath.Join(cwd, ".ai")
	if err := os.MkdirAll(aiDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir .ai: %w", err)
	}
	path := filepath.Join(aiDir, "directives.md")
	if _, err := os.Stat(path); err == nil {
		// Ya existe — no sobrescribimos.
		return path, nil
	}
	content := `# Directivas para agentes AI — proyecto Domain

Este proyecto tiene **Domain MCP** registrado. Usá las tools ` + "`domain_*`" + ` para:

- **Memoria persistente**: ` + "`domain_mem_save`" + ` para guardar observaciones,
  ` + "`domain_mem_search`" + ` para recuperar contexto previo.
- **Agents**: ` + "`domain_agent_run`" + ` para ejecutar agentes definidos
  en la plataforma con tools + skills.
- **Flows**: ` + "`domain_flow_run`" + ` para orquestaciones multi-step.
- **Skills**: ` + "`domain_skill_execute`" + ` para invocar skills nativos
  (prompts versionados, API calls validadas).
- **Prompts**: ` + "`domain_prompt_render`" + ` con templates parametrizados.

**Antes de actuar**: chequeá memoria con ` + "`domain_mem_search`" + ` para evitar
trabajo duplicado y mantener continuidad cross-sesión.

**Al terminar tareas no triviales**: guardá un resumen con
` + "`domain_mem_save`" + ` (decisión, bug fix, convention, gotcha) — esto
preserva contexto para futuras sesiones del mismo proyecto.

## Configs sensibles

NUNCA modificar archivos:
- ` + "`.env`" + `, ` + "`.env.*`" + ` (secrets locales)
- ` + "`.git/`" + ` (estado del repo)
- ` + "`*.pem`" + `, ` + "`*.key`" + ` (private keys)
- ` + "`credentials.*`" + `, ` + "`*credentials*`" + `
- ` + "`*.kube/config`" + ` (cluster creds)

Si una tarea requiere modificar uno de esos → preguntá explícito al humano antes.
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write directives: %w", err)
	}
	return path, nil
}
