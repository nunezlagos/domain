// issue-12.5 — targets de setup por agente: Claude Code (.mcp.json de
// proyecto), OpenCode (opencode.json) y Claude Desktop (config global).
// Incluye Status (detección) y Uninstall por agente.
package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"nunezlagos/domain/internal/agentprotocol"
)

// Agent identifica un agente externo soportado.
type Agent string

const (
	AgentClaudeCode    Agent = "claude-code"
	AgentOpenCode      Agent = "opencode"
	AgentClaudeDesktop Agent = "claude-desktop"
)

// SupportedAgents en orden de preferencia para Status.
var SupportedAgents = []Agent{AgentClaudeCode, AgentOpenCode, AgentClaudeDesktop}

// ConfigPath retorna la ruta del config para el agente. Los agentes de
// proyecto (claude-code, opencode) reciben dir; claude-desktop es global.
func ConfigPath(agent Agent, dir string) (string, error) {
	switch agent {
	case AgentClaudeCode:
		return filepath.Join(dir, ".mcp.json"), nil
	case AgentOpenCode:
		return filepath.Join(dir, "opencode.json"), nil
	case AgentClaudeDesktop:
		return ClaudeDesktopConfigPath()
	}
	return "", fmt.Errorf("%w: %s", ErrUnsupportedAgent, agent)
}

// readJSONFile lee y parsea un JSON a map genérico. No-existe → map vacío.
func readJSONFile(path string) (map[string]any, []byte, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, nil, fmt.Errorf("config existente inválido (%s): %w", path, err)
	}
	return doc, raw, nil
}

// writeJSONWithBackup escribe el doc, respaldando el original si existía.
func writeJSONWithBackup(path string, original []byte, doc map[string]any) error {
	if len(original) > 0 {
		backup := path + ".backup-" + time.Now().UTC().Format("20060102T150405Z")
		if err := os.WriteFile(backup, original, 0o600); err != nil {
			return fmt.Errorf("backup: %w", err)
		}
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(path, out, 0o600)
}

func domainEnv(apiKey, baseURL string) map[string]any {
	env := map[string]any{}
	if apiKey != "" {
		env["DOMAIN_API_KEY"] = apiKey
	}
	if baseURL != "" {
		env["DOMAIN_BASE_URL"] = baseURL
	}
	return env
}

// SetupClaudeCode registra domain-mcp en el .mcp.json del proyecto (formato
// estándar de Claude Code para servers MCP project-scope, commiteable).
func SetupClaudeCode(dir, mcpBinaryPath, apiKey, baseURL string) (string, error) {
	path, _ := ConfigPath(AgentClaudeCode, dir)
	doc, original, err := readJSONFile(path)
	if err != nil {
		return "", err
	}
	servers, _ := doc["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	if _, ok := servers["domain"]; ok {
		// Ya configurado. Aun asi, asegura que el slash command este
		// instalado (issue-01.9).
		if _, err := InstallSlashCommand(AgentClaudeCode); err != nil {
			return path, fmt.Errorf("install slash command: %w", err)
		}
		return path, ErrAlreadyConfigured
	}
	servers["domain"] = map[string]any{
		"command": mcpBinaryPath,
		"args":    []any{},
		"env":     domainEnv(apiKey, baseURL),
	}
	doc["mcpServers"] = servers
	if err := writeJSONWithBackup(path, original, doc); err != nil {
		return "", err
	}
	// issue-01.9: instala el slash command /domain-login.
	if _, err := InstallSlashCommand(AgentClaudeCode); err != nil {
		return path, fmt.Errorf("install slash command: %w", err)
	}
	return path, nil
}

// SetupOpenCode registra domain-mcp en el opencode.json del proyecto
// (formato OpenCode: mcp.<name> con type local + command array).
func SetupOpenCode(dir, mcpBinaryPath, apiKey, baseURL string) (string, error) {
	path, _ := ConfigPath(AgentOpenCode, dir)
	doc, original, err := readJSONFile(path)
	if err != nil {
		return "", err
	}
	if _, ok := doc["$schema"]; !ok {
		doc["$schema"] = "https://opencode.ai/config.json"
	}
	mcp, _ := doc["mcp"].(map[string]any)
	if mcp == nil {
		mcp = map[string]any{}
	}
	if _, ok := mcp["domain"]; ok {
		// Ya configurado. Aun asi, asegura slash command + instrucciones
		// del agente (pueden faltar en configs creados por versiones
		// previas del setup — es el caso "engram gana en jerarquía").
		if _, err := InstallSlashCommand(AgentOpenCode); err != nil {
			return path, fmt.Errorf("install slash command: %w", err)
		}
		if changed, err := ensureOpenCodeInstructions(path, doc, original); err != nil {
			return path, fmt.Errorf("install instructions: %w", err)
		} else if changed {
			return path, nil // hubo upgrade real, no "nada que hacer"
		}
		return path, ErrAlreadyConfigured
	}
	mcp["domain"] = map[string]any{
		"type":        "local",
		"command":     []any{mcpBinaryPath},
		"enabled":     true,
		"environment": domainEnv(apiKey, baseURL),
	}
	doc["mcp"] = mcp
	if _, err := ensureOpenCodeInstructions(path, doc, nil); err != nil {
		return "", fmt.Errorf("install instructions: %w", err)
	}
	if err := writeJSONWithBackup(path, original, doc); err != nil {
		return "", err
	}
	// issue-01.9: instala el slash command /domain-login.
	if _, err := InstallSlashCommand(AgentOpenCode); err != nil {
		// No es fatal — el MCP server sigue funcionando, pero el slash
		// command no estara disponible. Loguear via return.
		return path, fmt.Errorf("install slash command: %w", err)
	}
	return path, nil
}

// DomainProtocolInstructions es el STUB que se instala a nivel agente
// (capa que el LLM prioriza; el handshake MCP es la capa débil). El
// protocolo COMPLETO vive en BD como policy 'agent-protocol' (seedeada,
// editable, versionada): el stub solo le dice al agente que la cargue
// con domain_policy_get — editar la policy cambia el comportamiento de
// todos los agentes sin tocar archivos.
const DomainProtocolInstructions = agentprotocol.Stub

// ensureOpenCodeInstructions escribe el protocolo en
// ~/.config/opencode/instructions/domain.md y lo agrega al array
// "instructions" del opencode.json dado (mutando doc). Retorna si hubo
// cambio (para distinguir upgrade de no-op). Si doc venía de disco y
// cambió, lo persiste.
func ensureOpenCodeInstructions(cfgPath string, doc map[string]any, original []byte) (bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, err
	}
	instrDir := filepath.Join(home, ".config", "opencode", "instructions")
	if err := os.MkdirAll(instrDir, 0o755); err != nil {
		return false, err
	}
	instrPath := filepath.Join(instrDir, "domain.md")
	changed := false
	if existing, readErr := os.ReadFile(instrPath); readErr != nil || string(existing) != DomainProtocolInstructions {
		if err := os.WriteFile(instrPath, []byte(DomainProtocolInstructions), 0o644); err != nil {
			return false, err
		}
		changed = true
	}

	// Merge en doc["instructions"] sin duplicar.
	var list []any
	if raw, ok := doc["instructions"].([]any); ok {
		list = raw
	}
	for _, item := range list {
		if s, ok := item.(string); ok && s == instrPath {
			if changed && original != nil {
				// El array ya estaba pero el .md cambió: nada que escribir
				// en el json; el cambio del archivo alcanza.
				return true, nil
			}
			return changed, nil
		}
	}
	doc["instructions"] = append(list, instrPath)
	if original != nil {
		// doc venía de un config ya escrito (camino already-configured):
		// persistir el agregado del array.
		if err := writeJSONWithBackup(cfgPath, original, doc); err != nil {
			return false, err
		}
	}
	return true, nil
}

// OpenCodeGlobalDir retorna el dir del config global de OpenCode
// (~/.config/opencode). El opencode.json ahí aplica a todos los proyectos.
func OpenCodeGlobalDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "opencode"), nil
}

// SetupClaudeCodeGlobal registra domain-mcp en ~/.claude.json (user scope
// de Claude Code: mcpServers disponible en todos los proyectos).
func SetupClaudeCodeGlobal(mcpBinaryPath, apiKey, baseURL string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(home, ".claude.json")
	doc, original, err := readJSONFile(path)
	if err != nil {
		return "", err
	}
	servers, _ := doc["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	if _, ok := servers["domain"]; ok {
		if _, err := InstallSlashCommand(AgentClaudeCode); err != nil {
			return path, fmt.Errorf("install slash command: %w", err)
		}
		return path, ErrAlreadyConfigured
	}
	servers["domain"] = map[string]any{
		"command": mcpBinaryPath,
		"args":    []any{},
		"env":     domainEnv(apiKey, baseURL),
	}
	doc["mcpServers"] = servers
	if err := writeJSONWithBackup(path, original, doc); err != nil {
		return "", err
	}
	if _, err := InstallSlashCommand(AgentClaudeCode); err != nil {
		return path, fmt.Errorf("install slash command: %w", err)
	}
	return path, nil
}

// AgentStatus estado de configuración de un agente.
type AgentStatus struct {
	Agent      Agent  `json:"agent"`
	ConfigPath string `json:"config_path"`
	Exists     bool   `json:"config_exists"`
	Configured bool   `json:"domain_configured"`
}

// Status reporta para cada agente soportado si domain está registrado.
func Status(dir string) []AgentStatus {
	var out []AgentStatus
	for _, agent := range SupportedAgents {
		st := AgentStatus{Agent: agent}
		path, err := ConfigPath(agent, dir)
		if err != nil {
			continue
		}
		st.ConfigPath = path
		doc, raw, err := readJSONFile(path)
		if err == nil && raw != nil {
			st.Exists = true
			st.Configured = hasDomainServer(agent, doc)
		}
		out = append(out, st)
	}
	return out
}

func hasDomainServer(agent Agent, doc map[string]any) bool {
	key := "mcpServers"
	if agent == AgentOpenCode {
		key = "mcp"
	}
	servers, _ := doc[key].(map[string]any)
	_, ok := servers["domain"]
	return ok
}

// Uninstall quita el server domain del config del agente (con backup).
// Retorna el path tocado, o ErrUnsupportedAgent / no-op si no estaba.
func Uninstall(agent Agent, dir string) (string, bool, error) {
	path, err := ConfigPath(agent, dir)
	if err != nil {
		return "", false, err
	}
	doc, original, err := readJSONFile(path)
	if err != nil {
		return "", false, err
	}
	if raw := original; raw == nil {
		return path, false, nil // config no existe
	}
	key := "mcpServers"
	if agent == AgentOpenCode {
		key = "mcp"
	}
	servers, _ := doc[key].(map[string]any)
	if _, ok := servers["domain"]; !ok {
		return path, false, nil
	}
	delete(servers, "domain")
	doc[key] = servers
	if err := writeJSONWithBackup(path, original, doc); err != nil {
		return "", false, err
	}
	return path, true, nil
}

// issue-01.9 — slash command /domain-login. Se instala en el dir de
// commands del agente (opencode o claude-code) cuando se hace
// `domain setup [agent]`. El user puede entonces invocar el wizard
// desde dentro del chat del agente con /domain-login.

// CommandsDir retorna el dir donde se ponen los slash commands para el
// agente. Para opencode: ~/.config/opencode/commands. Para claude-code:
// ~/.claude/commands. Para claude-desktop: no aplica (no tiene slash
// commands en el mismo sentido — retorna "").
func CommandsDir(agent Agent) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch agent {
	case AgentOpenCode:
		return filepath.Join(home, ".config", "opencode", "commands"), nil
	case AgentClaudeCode:
		return filepath.Join(home, ".claude", "commands"), nil
	case AgentClaudeDesktop:
		return "", fmt.Errorf("claude-desktop no soporta slash commands custom")
	}
	return "", fmt.Errorf("agent %s no soportado", agent)
}

// DomainLoginCommandContent es el contenido del slash command /domain-login.
// Es markdown que el agente lee como system prompt cuando el user
// invoca /domain-login. Usa \u0060 (Unicode escape) en vez de backticks
// para evitar cerrar prematuramente el raw string de Go.
const DomainLoginCommandContent = `---
description: Trigger the Domain onboarding wizard (login or bootstrap first user)
agent: build
---

# /domain-login

The user wants to authenticate with the Domain MCP server.

## Steps

1. **Run the onboard wizard** in a TTY-attached way. The wizard will:
   - Detect if the DB is empty (first-run): if so, it auto-creates the first user + org + API key.
   - Otherwise, it sends a 6-digit OTP code to the user's email and prompts for it.
   - Saves the API key to \u0060~/.config/domain/credentials.json\u0060 (mode 0600).

   \u0060\u0060\u0060bash
   # Detect the domain binary path: usually the domain-mcp binary's sibling.
   DOMAIN_BIN="$(dirname "$(command -v domain-mcp 2>/dev/null || echo /usr/local/bin/domain-mcp)")/domain"
   if [ ! -x "$DOMAIN_BIN" ]; then
     DOMAIN_BIN="$(command -v domain 2>/dev/null || echo domain)"
   fi
   "$DOMAIN_BIN" onboard --base-url "${DOMAIN_BASE_URL:-http://localhost:8000}"
   \u0060\u0060\u0060

2. **If the wizard asked the user for the server URL**, the user can type the URL directly into the terminal.

3. **Re-configure opencode** to use the new key:

   \u0060\u0060\u0060bash
   "$DOMAIN_BIN" setup opencode --api-key "$(jq -r '.api_key' \u0060~/.config/domain/credentials.json\u0060)" --base-url "${DOMAIN_BASE_URL:-http://localhost:8000}"
   \u0060\u0060\u0060

4. **Confirm to the user**:
   - Print: \u2705 Logged in to Domain. You can now use domain_mem_save, domain_policy_list, etc.
   - **DO NOT** print the API key in the chat (it would be logged in the agent's history).
   - **DO NOT** echo any environment variables that contain the API key.

5. **If the wizard fails**, surface the error verbatim to the user (without paraphrasing or adding commentary).

## Security notes

- The API key is sensitive. Never echo it in chat, never write it to files that the agent could read, never include it in tool call results that the user could accidentally copy.
- The user sees the key once during the wizard on their terminal; that's the only place it should appear.
- The wizard may take 30-60 seconds if the user has to check their email for the OTP code. Be patient.

## When to use this command

- The user just installed the Domain MCP server and needs to authenticate.
- The user got an "API key invalid" error from a tool call.
- The user wants to switch to a different account.
- The user is a brand new user (first run of the system) and needs to bootstrap the first user.
`

// InstallSlashCommand escribe el .md del slash command /domain-login en
// el dir del agente. Si el archivo ya existe, hace backup.
func InstallSlashCommand(agent Agent) (string, error) {
	dir, err := CommandsDir(agent)
	if err != nil {
		return "", err
	}
	if dir == "" {
		return "", fmt.Errorf("agent %s no soporta slash commands", agent)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "domain-login.md")
	if _, err := os.Stat(path); err == nil {
		backup := path + ".backup-" + time.Now().UTC().Format("20060102T150405Z")
		if err := os.Rename(path, backup); err != nil {
			return "", err
		}
	}
	if err := os.WriteFile(path, []byte(DomainLoginCommandContent), 0o644); err != nil {
		return "", err
	}
	return path, nil
}
