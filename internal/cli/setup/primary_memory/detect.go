package primary_memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Detect escanea el config de un agente y retorna los MCP servers
// detectados con su clasificación (memory vs no-memory).
//
// agent: "opencode" o "claude-code"
// configPath: ruta absoluta al archivo de config.
//
// OpenCode: opencode.json con clave "mcp.<name>".
// Claude Code: claude.json con clave "mcpServers.<name>".
//
// Si el archivo no existe → retorna lista vacía sin error (idempotente).
// Si el JSON está malformado → retorna warning + lista vacía.
func Detect(agent, configPath string) ([]DetectedProvider, error) {
	body, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		// JSON malformado: warning + lista vacía. No fallamos —
		// el operador puede arreglar el JSON y reintentar.
		return nil, nil
	}

	var serversKey string
	switch agent {
	case "opencode":
		serversKey = "mcp"
	case "claude-code":
		serversKey = "mcpServers"
	default:
		return nil, fmt.Errorf("unknown agent: %s", agent)
	}

	servers, _ := doc[serversKey].(map[string]any)
	if servers == nil {
		return nil, nil
	}

	out := make([]DetectedProvider, 0, len(servers))
	for name := range servers {
		out = append(out, DetectedProvider{
			Name:       name,
			ConfigPath: configPath,
			IsMemory:   IsMemoryProvider(name),
			Agent:      agent,
		})
	}
	return out, nil
}

// MemoryProviders filtra los detectados para solo los que son
// memory providers.
func MemoryProviders(providers []DetectedProvider) []DetectedProvider {
	out := make([]DetectedProvider, 0)
	for _, p := range providers {
		if p.IsMemory {
			out = append(out, p)
		}
	}
	return out
}

// IsAlreadyDisabled chequea si un provider está "disabled" según
// convención opencode (command=false o command=[]) vs Claude Code
// (command=false).
//
// Retorna true si el provider está explícitamente deshabilitado.
// En ese caso Disable() es no-op.
func IsAlreadyDisabled(agent, configPath, providerName string) (bool, error) {
	body, err := os.ReadFile(configPath)
	if err != nil {
		return false, err
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return false, nil
	}
	serversKey := "mcp"
	if agent == "claude-code" {
		serversKey = "mcpServers"
	}
	servers, _ := doc[serversKey].(map[string]any)
	if servers == nil {
		return false, nil
	}
	entry, _ := servers[providerName].(map[string]any)
	if entry == nil {
		return false, nil
	}
	// Convención: command=false (opencode) o command=[].
	if cmd, ok := entry["command"]; ok {
		if b, ok := cmd.(bool); ok && !b {
			return true, nil
		}
		if arr, ok := cmd.([]any); ok && len(arr) == 0 {
			return true, nil
		}
	}
	// Convención alternativa: enabled=false.
	if enabled, ok := entry["enabled"].(bool); ok && !enabled {
		return true, nil
	}
	return false, nil
}

// ConfigDir retorna el directorio de configs para un agente.
// Útil para tests y para el comando opencode --global.
func ConfigDir(agent string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch agent {
	case "opencode":
		return filepath.Join(home, ".config", "opencode"), nil
	case "claude-code":
		return home, nil // ~/.claude.json
	}
	return "", fmt.Errorf("unknown agent: %s", agent)
}

// DefaultConfigPath retorna la ruta default del config del agente.
func DefaultConfigPath(agent string) (string, error) {
	dir, err := ConfigDir(agent)
	if err != nil {
		return "", err
	}
	switch agent {
	case "opencode":
		return filepath.Join(dir, "opencode.json"), nil
	case "claude-code":
		return filepath.Join(dir, ".claude.json"), nil
	}
	return "", fmt.Errorf("unknown agent: %s", agent)
}

// IsConfigMissing verifica si el config file no existe (no es error,
// es estado válido para "no hay otros providers").
func IsConfigMissing(err error) bool {
	return err != nil && strings.Contains(err.Error(), "no such file")
}
