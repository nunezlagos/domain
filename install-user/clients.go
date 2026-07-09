package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// configResult informa el efecto de configurar un cliente.
type configResult struct {
	Skipped bool   // true si no se tocó nada (ej. ya hay un 'domain' local)
	Reason  string // motivo del skip, para informar al usuario
}

// configureClient escribe el config MCP de domain-mcp en el archivo apropiado
// del cliente, preservando otras entries y migrando entry legacy "domain".
// Devuelve el resultado (incl. skip por dedup) y error.
//
// La estructura del JSON depende del cliente:
//   - claude-code:  ~/.claude.json   { "mcpServers": { "domain-mcp": {type:http, url, headers} } }
//   - cursor/cline: mcp.json         { "mcpServers": { "domain-mcp": {url, headers} } }
//   - opencode:     opencode.json    { "mcp": { "domain-mcp": {type: "remote", url, headers, enabled} } }
//   - continue:                      { "experimental": { "modelContextProtocolServers": [...] } }
//   - claude-desktop:                STDIO-only (no soporta http remoto) → se omite con aviso
func configureClient(c Client, vpsURL, apiKey, timestamp string) (configResult, error) {
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
		res, err := upsertJSON(c.MCPPath, "mcp", entry, timestamp)
		if err != nil {
			return res, err
		}
		writeEnvIfConfigured(c, apiKey, timestamp)
		return res, nil
	case "continue":
		if err := configureContinue(c.MCPPath, vpsURL, apiKey, timestamp); err != nil {
			return configResult{}, err
		}
		return configResult{}, nil
	case "claude-desktop":
		// Claude Desktop NO soporta transporte http remoto en
		// claude_desktop_config.json (solo stdio/command). Inyectar una
		// entry {url, headers} sería descartada silenciosamente por la app.
		// Para conectar el server remoto haría falta un puente stdio
		// (npx mcp-remote <url>), que no asumimos instalado. Lo omitimos
		// con un aviso explícito en vez de escribir config inútil.
		return configResult{
			Skipped: true,
			Reason:  "Claude Desktop no soporta MCP remoto http; usá Claude Code/Cursor/OpenCode, o un puente stdio (mcp-remote)",
		}, nil
	case "claude-code":
		// Claude Code soporta transporte http remoto: requiere type:"http".
		entry["type"] = "http"
		res, err := upsertJSON(c.MCPPath, "mcpServers", entry, timestamp)
		if err != nil {
			return res, err
		}
		writeEnvIfConfigured(c, apiKey, timestamp)
		return res, nil
	default:
		// cursor, cline: mcpServers con {url, headers} (sin type explícito).
		return upsertJSON(c.MCPPath, "mcpServers", entry, timestamp)
	}
}

// writeEnvIfConfigured escribe el .env del cliente si tiene EnvPath seteado.
// Backup del .env viejo antes de pisar (con timestamp). Idempotente.
func writeEnvIfConfigured(c Client, apiKey, timestamp string) {
	if c.EnvPath == "" {
		return
	}
	// backup del .env viejo con dedup + poda (backupIfExists)
	if _, err := backupIfExists(c.EnvPath, timestamp); err != nil {
		warnL("no se pudo backupear " + c.EnvPath + ": " + err.Error())
	}
	if err := writeClientEnv(c.EnvPath, apiKey); err != nil {
		warnL("no se pudo escribir " + c.EnvPath + ": " + err.Error())
	}
}

func upsertJSON(path, container string, entry map[string]any, timestamp string) (configResult, error) {
	// Cargamos primero para decidir si hay que tocar el archivo: si vamos a
	// hacer skip por dedup, no creamos backup ni reescribimos nada.
	m, err := loadOrEmptyJSON(path)
	if err != nil {
		return configResult{}, err
	}
	if skipped := upsertMCPEntry(m, container, entry); skipped {
		return configResult{
			Skipped: true,
			Reason:  "ya existe una entry 'domain' local (instalador del server); no agrego entry remota duplicada",
		}, nil
	}
	if _, err := backupIfExists(path, timestamp); err != nil {
		return configResult{}, fmt.Errorf("backup: %w", err)
	}
	return configResult{}, writeJSON(path, m)
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
	domainURL := vpsURL + "/mcp"
	domainEntry := map[string]any{
		"transport": map[string]any{
			"type": "http",
			"url":  domainURL,
			"headers": map[string]any{
				"Authorization": "Bearer " + apiKey,
			},
		},
	}
	// issue-65.1: MERGE en vez de reemplazar. Preservar los MCP servers ajenos
	// del usuario. Buscar la entrada de domain y actualizarla in-place; si no
	// existe, hacer append. Antes se pisaba todo el array con un solo elemento,
	// borrando los otros servers del usuario. El match reconoce una entrada de
	// domain por su url exacta (mismo VPS) O por su header Authorization con una
	// key domk_ (marca inequívoca de una entrada de domain): así, si el VPS
	// migró de host, se actualiza la entrada vieja en vez de dejar una stale
	// duplicada, sin confundirla con un server ajeno que casualmente use /mcp.
	existing, _ := exp["modelContextProtocolServers"].([]any)
	updated := false
	for i, s := range existing {
		sm, ok := s.(map[string]any)
		if !ok {
			continue
		}
		if isDomainContinueEntry(sm, domainURL) {
			existing[i] = domainEntry
			updated = true
			break
		}
	}
	if !updated {
		existing = append(existing, domainEntry)
	}
	exp["modelContextProtocolServers"] = existing
	m["experimental"] = exp
	return writeJSON(path, m)
}

// isDomainContinueEntry reconoce si una entrada de modelContextProtocolServers
// es la de domain: por url exacta (mismo VPS) o por su header Authorization con
// una key domk_ (marca inequívoca), lo que permite re-identificarla aunque el
// VPS haya migrado de host. NO matchea por sufijo /mcp genérico para no pisar un
// server ajeno del usuario que casualmente use ese path.
func isDomainContinueEntry(sm map[string]any, domainURL string) bool {
	tr, _ := sm["transport"].(map[string]any)
	if tr == nil {
		return false
	}
	if url, _ := tr["url"].(string); url == domainURL {
		return true
	}
	headers, _ := tr["headers"].(map[string]any)
	auth, _ := headers["Authorization"].(string)
	return strings.Contains(auth, "Bearer domk_")
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
	// issue-65.1: backup antes de escribir (antes se escribía sin respaldo si
	// solo se limpiaba la entrada de continue — r3).
	if _, err := backupIfExists(c.MCPPath, Timestamp()); err != nil {
		return false, err
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
