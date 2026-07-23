package main

import (
	"context"
	"strings"
	"time"
)

// checkMCPEntry verifica que el MCP entry exista en los archivos de configuración
// de los clientes detectados (Claude Code y OpenCode) con url + header Authorization.
// Sin entry MCP, las tools domain_* no están disponibles (DOMAINSERV-76).
func checkMCPEntry(paths Paths) int {
	step("Entry MCP (clients.json)")
	fails := 0

	for _, path := range []string{
		paths.ClaudeCodeMCP,
		paths.OpencodeMCP,
	} {
		if !fileExists(path) {
			continue
		}
		cfg, err := loadOrEmptyJSON(path)
		if err != nil {
			failL(path + " ilegible: " + err.Error())
			fails++
			continue
		}
		// Claude Code: top-level "mcpServers"; OpenCode: top-level "mcp"
		var servers map[string]any
		if s, _ := cfg["mcpServers"].(map[string]any); s != nil {
			servers = s
		} else if s, _ := cfg["mcp"].(map[string]any); s != nil {
			servers = s
		}
		if servers == nil {
			failL(path + ": sin mcpServers/mcp")
			fails++
			continue
		}
		entry, found := servers["domain-mcp"].(map[string]any)
		if !found || entry == nil {
			failL(path + ": falta entry 'domain-mcp' en mcpServers/mcp")
			fails++
			continue
		}
		urlVal, _ := entry["url"].(string)
		headers, _ := entry["headers"].(map[string]any)
		authVal, _ := headers["Authorization"].(string)
		if urlVal == "" {
			failL(path + ": entry domain-mcp sin url")
			fails++
		}
		if authVal == "" {
			failL(path + ": entry domain-mcp sin header Authorization (Bearer)")
			fails++
		}
		if urlVal != "" && authVal != "" {
			ok(path + ": entry domain-mcp presente (url + Authorization)")
		}
	}
	return fails
}

// checkMCPHealth chequea, best-effort, que el VPS del MCP domain responda.
// NO es crítico: si no hay VPS_URL o el VPS no responde, avisa y sigue (la
// instalación local puede estar intacta con el VPS temporalmente caído).
func checkMCPHealth(paths Paths) {
	step("Salud del MCP domain")
	env, err := loadEnv(paths.GlobalEnv)
	if err != nil || env.VPSURL == "" {
		warnL("sin VPS_URL en install.env — chequeo omitido (no crítico)")
		return
	}
	url := strings.TrimRight(env.VPSURL, "/")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := pingVPS(ctx, url); err != nil {
		warnL("VPS no responde en " + url + ": " + err.Error() + " (no crítico: degrada gracioso)")
		return
	}
	ok("VPS responde en " + url)
}
