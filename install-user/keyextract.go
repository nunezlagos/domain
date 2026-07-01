package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// readAPIKeyFromConfig lee la key de "Authorization: Bearer XXX" del entry
// domain-mcp dentro del container (mcp o mcpServers) de un JSON existente.
// Devuelve "" si el archivo no existe, está corrupto, o no tiene la entry.
func readAPIKeyFromConfig(path, container, entryName string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "" // no existe → sin key
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return "" // corrupto → tratar como sin key (deja que el caller decida)
	}
	cont, ok := m[container].(map[string]any)
	if !ok {
		return ""
	}
	entry, ok := cont[entryName].(map[string]any)
	if !ok {
		return ""
	}
	headers, ok := entry["headers"].(map[string]any)
	if !ok {
		return ""
	}
	auth, _ := headers["Authorization"].(string)
	return extractAPIKeyFromBearer(auth)
}

// resolveAPIKey decide qué API key usar siguiendo la prioridad del contrato:
//
//	1) explicitKey (de --api-key) si no vacío → esa, sin chequear nada más
//	2) OpenCode config (opencode.json → mcp.domain-mcp.headers.Authorization)
//	3) Claude Code config (~/.claude.json → mcpServers.domain-mcp.headers.Authorization)
//	4) Prompt al usuario (interactive, hidden) si no nonInteractive
//
// Devuelve la key final o "" si no se pudo resolver.
// Si las keys de OpenCode y Claude Code son DIFERENTES, prioriza OpenCode
// (somos el mismo usuario) y devuelve (opencodeKey, claudeKey) para que el caller
// pueda loguear el warning.
//
// Nota: el caller debe pasar los paths ya resueltos (p.OpencodePath, p.ClaudeCodePath)
// para no acoplar resolveAPIKey con la detección de plataforma.
func resolveAPIKey(opencodePath, claudeCodePath, explicitKey string, in *bufio.Reader, nonInteractive bool) (key string, sources string, err error) {
	sources = ""

	// 1) Flag explícito gana sobre todo
	if explicitKey != "" {
		return explicitKey, "flag --api-key", nil
	}

	// 2-3) Buscar en configs existentes
	ocKey := readAPIKeyFromConfig(opencodePath, "mcp", "domain-mcp")
	ccKey := readAPIKeyFromConfig(claudeCodePath, "mcpServers", "domain-mcp")

	switch {
	case ocKey != "" && ccKey != "" && ocKey != ccKey:
		// Ambas con keys DIFERENTES: priorizamos OpenCode y avisamos
		return ocKey, fmt.Sprintf("opencode=%s (claudecode=%s distinto, usando opencode)", mask(ocKey), mask(ccKey)), nil
	case ocKey != "":
		return ocKey, "opencode", nil
	case ccKey != "":
		return ccKey, "claudecode", nil
	}

	// 4) Sin key en ningún config → prompt
	if nonInteractive {
		return "", "", fmt.Errorf("no hay API key en configs y --yes activo: pasá --api-key o corré --bootstrap")
	}
	prompted := promptHidden(in, "  API key (domk_live_xxx o domk_test_xxx): ")
	prompted = strings.TrimSpace(prompted)
	if prompted == "" {
		return "", "", fmt.Errorf("API key requerida (vacía)")
	}
	if !apiKeyPattern.MatchString(prompted) {
		return "", "", fmt.Errorf("API key con formato inválido (esperado domk_(live|test)_...)")
	}
	return prompted, "prompt", nil
}

// mask oculta el medio de una key para logging seguro: domk_live_XXXXXX...mnop
func mask(key string) string {
	if len(key) < 12 {
		return "***"
	}
	return key[:10] + "..." + key[len(key)-4:]
}

// apiKeyPattern valida que una key tiene formato real (domk_live_ o domk_test_
// seguido de sufijo aleatorio ≥ 20 chars). NO acepta placeholders tipo
// "domk_live_REEMPLAZAR_*" o "domk_live_xxx" — esos son claramente ficticios.
// apiKeyPattern valida el formato base: domk_(live|test)_ + 16+ chars.
// El check adicional (digito + minuscula) descarta placeholders.
var apiKeyPattern = regexp.MustCompile(`^domk_(live|test)_[A-Za-z0-9_-]{16,}$`)

// extractAPIKeyFromBearer toma un header "Authorization: Bearer XYZ" y devuelve
// la key XYZ si tiene formato válido. Devuelve "" si el header está vacío,
// no tiene prefijo "Bearer ", o la key no matchea apiKeyPattern (placeholders).
func extractAPIKeyFromBearer(header string) string {
	const prefix = "Bearer "
	if len(header) <= len(prefix) || header[:len(prefix)] != prefix {
		return ""
	}
	key := header[len(prefix):]
	if !apiKeyPattern.MatchString(key) {
		return ""
	}
	// Check adicional: el sufijo debe tener al menos un digito y una minuscula.
	// Descarta placeholders tipo REEMPLAZAR_CON_TU_API_KEY (todo MAYUSCULAS).
	suffix := key
	for _, p := range []string{"domk_live_", "domk_test_"} {
		if len(key) > len(p) && key[:len(p)] == p {
			suffix = key[len(p):]
			break
		}
	}
	hasDigit, hasLower := false, false
	for _, c := range suffix {
		if c >= '0' && c <= '9' {
			hasDigit = true
		}
		if c >= 'a' && c <= 'z' {
			hasLower = true
		}
	}
	if !hasDigit || !hasLower {
		return ""
	}
	return key
}

// writeClientEnv escribe el .env del cliente con DOMAIN_MCP_API_KEY.
// Path se crea si no existe. chmod 0600. Idempotente (sobrescribe siempre).
// El comentario al inicio explica que es la misma key para todos los clientes.
func writeClientEnv(path, apiKey string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	body := fmt.Sprintf(`# domain MCP — API key para %s
# Esta key es la MISMA para todos tus clientes (somos el mismo usuario).
# Para cambiarla: editá este archivo y re-corré domain-install.
DOMAIN_MCP_API_KEY=%s
`, filepath.Base(filepath.Dir(path)), apiKey)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}