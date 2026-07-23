package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// checkMCPEntry verifica que el MCP entry exista en los archivos de configuración
// de los clientes detectados (Claude Code y OpenCode) con url + header Authorization.
// Sin entry MCP, las tools domain_* no están disponibles (DOMAINSERV-76).
func checkMCPEntry(home string) int {
	step("Entry MCP (clients.json)")
	fails := 0

	for _, path := range []string{
		filepath.Join(home, ".claude.json"),
		filepath.Join(home, ".config", "opencode", "opencode.json"),
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

// checkOpencodePermission verifica que opencode.json tenga el bloque
// `permission` con las reglas deny de git destructivo (DOMAINSERV-69).
func checkOpencodePermission(home string) int {
	step("Permisos OpenCode (opencode.json)")
	path := filepath.Join(home, ".config", "opencode", "opencode.json")
	if !fileExists(path) {
		info("opencode no detectado — chequeo omitido")
		return 0
	}
	m, err := loadOrEmptyJSON(path)
	if err != nil {
		failL(path + " ilegible: " + err.Error())
		return 1
	}
	perm, _ := m["permission"].(map[string]any)
	if perm == nil {
		failL(path + ": falta bloque 'permission'")
		return 1
	}
	bashRules, _ := perm["bash"].(map[string]any)
	if bashRules == nil {
		failL(path + ": permission falta 'bash'")
		return 1
	}
	var missing []string
	for _, rule := range opencodeGitDenyRules {
		if v, ok := bashRules[rule]; !ok || fmt.Sprint(v) != "deny" {
			missing = append(missing, rule)
		}
	}
	if len(missing) > 0 {
		failL(fmt.Sprintf("faltan reglas deny en permission.bash: %v", missing))
		return 1
	}
	ok("todas las reglas git deny presentes en permission.bash")
	return 0
}

// checkMCPHealth chequea, best-effort, que el VPS del MCP domain responda.
// NO es crítico: si no hay VPS_URL o el VPS no responde, avisa y sigue (la
// instalación local puede estar intacta con el VPS temporalmente caído).
func checkMCPHealth(home string) {
	step("Salud del MCP domain")
	envPath := filepath.Join(home, ".config", "domain", "install.env")
	env, err := loadEnv(envPath)
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
