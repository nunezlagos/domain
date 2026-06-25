package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// configureClient escribe el config MCP de domain-mcp en el archivo apropiado
// del cliente, preservando otras entries y migrando entry legacy "domain".
// Devuelve (path_efectivo, error).
//
// La estructura del JSON depende del cliente:
//   - claude-code/cursor/cline/claude-desktop: { "mcpServers": { "domain-mcp": {url, headers} } }
//   - opencode:                                { "mcp":        { "domain-mcp": {type: "remote", url, headers, enabled} } }
//   - continue:                                { "experimental": { "modelContextProtocolServers": [...] } }
func configureClient(c Client, vpsURL, apiKey, timestamp string) error {
	entry := map[string]any{
		"url": vpsURL + "/mcp",
		"headers": map[string]any{
			"Authorization": "Bearer " + apiKey,
		},
	}
	switch c.Name {
	case "opencode":

		entry["type"] = "remote"
		entry["enabled"] = true
		return upsertJSON(c.MCPPath, "mcp", entry, timestamp)
	case "continue":

		return configureContinue(c.MCPPath, vpsURL, apiKey, timestamp)
	default:
		return upsertJSON(c.MCPPath, "mcpServers", entry, timestamp)
	}
}

func upsertJSON(path, container string, entry map[string]any, timestamp string) error {
	if _, err := backupIfExists(path, timestamp); err != nil {
		return fmt.Errorf("backup: %w", err)
	}
	m, err := loadOrEmptyJSON(path)
	if err != nil {
		return err
	}
	upsertMCPEntry(m, container, entry)
	return writeJSON(path, m)
}

func configureContinue(path, vpsURL, apiKey, timestamp string) error {
	if _, err := backupIfExists(path, timestamp); err != nil {
		return err
	}
	m, err := loadOrEmptyJSON(path)
	if err != nil {
		return err
	}
	exp, _ := m["experimental"].(map[string]any)
	if exp == nil {
		exp = map[string]any{}
	}
	exp["modelContextProtocolServers"] = []any{
		map[string]any{
			"transport": map[string]any{
				"type": "http",
				"url":  vpsURL + "/mcp",
				"headers": map[string]any{
					"Authorization": "Bearer " + apiKey,
				},
			},
		},
	}
	m["experimental"] = exp
	return writeJSON(path, m)
}

// uninstallClient remueve entries "domain-mcp" + "domain" del config sin
// tocar otras entries. Si el container queda vacío, lo elimina.
func uninstallClient(c Client) (removed bool, err error) {
	if _, err := os.Stat(c.MCPPath); os.IsNotExist(err) {
		return false, nil
	}
	m, err := loadOrEmptyJSON(c.MCPPath)
	if err != nil {
		return false, err
	}
	r1 := removeMCPEntry(m, "mcpServers")
	r2 := removeMCPEntry(m, "mcp")



	r3 := false
	if c.Name == "continue" {
		if exp, ok := m["experimental"].(map[string]any); ok {
			if arr, ok := exp["modelContextProtocolServers"].([]any); ok && len(arr) > 0 {



				if len(arr) == 1 {
					delete(exp, "modelContextProtocolServers")
					r3 = true
				}
			}
			if len(exp) == 0 {
				delete(m, "experimental")
			} else {
				m["experimental"] = exp
			}
		}
	}
	if !(r1 || r2 || r3) {
		return false, nil
	}
	return true, writeJSON(c.MCPPath, m)
}

// installGlobalAssets escribe el skill + subagent globales. Idempotente.
func installGlobalAssets(paths Paths) error {
	if err := os.MkdirAll(filepath.Dir(paths.GlobalSkillPath), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(paths.GlobalAgentPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(paths.GlobalSkillPath, skillDomainMD, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(paths.GlobalAgentPath, agentDomainMemoryMD, 0o644); err != nil {
		return err
	}
	return nil
}

// linkOpencodeToGlobal: en Linux/macOS hace symlink. En Windows, donde
// los symlinks requieren permisos especiales, COPIA el contenido.
func linkOpencodeToGlobal(paths Paths, osName string) error {
	pairs := []struct {
		target string
		link   string
	}{
		{paths.GlobalSkillPath, paths.OpencodeSkillsLn},
		{paths.GlobalAgentPath, paths.OpencodeAgentsLn},
	}
	for _, p := range pairs {
		if err := os.MkdirAll(filepath.Dir(p.link), 0o755); err != nil {
			return err
		}

		_ = os.Remove(p.link)
		if osName == "windows" {
			b, err := os.ReadFile(p.target)
			if err != nil {
				return err
			}
			if err := os.WriteFile(p.link, b, 0o644); err != nil {
				return err
			}
		} else {
			if err := os.Symlink(p.target, p.link); err != nil {
				return err
			}
		}
	}
	return nil
}

func removeGlobalAssets(paths Paths) {
	_ = os.Remove(paths.GlobalSkillPath)
	_ = os.Remove(paths.GlobalAgentPath)

	_ = os.Remove(filepath.Dir(paths.GlobalSkillPath))
	_ = os.Remove(filepath.Dir(paths.GlobalAgentPath))
}

func removeOpencodeLinks(paths Paths) {
	_ = os.Remove(paths.OpencodeSkillsLn)
	_ = os.Remove(paths.OpencodeAgentsLn)
}
