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
		return path, ErrAlreadyConfigured
	}
	mcp["domain"] = map[string]any{
		"type":        "local",
		"command":     []any{mcpBinaryPath},
		"enabled":     true,
		"environment": domainEnv(apiKey, baseURL),
	}
	doc["mcp"] = mcp
	if err := writeJSONWithBackup(path, original, doc); err != nil {
		return "", err
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
