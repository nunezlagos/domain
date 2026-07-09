package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// errInvalidAPIKey indica que el server rechazo la key (HTTP 401). Distinto
// de errores de red (que se loguean como warning y NO bloquean el install).
var errInvalidAPIKey = errors.New("API key invalida o revocada")

// validateAPIKeyHTTPTimeout es el timeout total para el round-trip de
// validacion. Corto a proposito: validar contra el server no debe ser
// un cuello de botella del install. Si el server esta lento/caido, skip
// con warning en vez de romper el flujo offline-first.
const validateAPIKeyHTTPTimeout = 5 * time.Second

// validateAPIKey hace GET <vpsURL>/api/v1/auth/validate con la key en
// Authorization: Bearer. El server (endpoint nuevo en domain-services)
// valida contra el middleware apikey y retorna 200 si la key resuelve a
// un Principal vivo, 401 si no.
//
// Comportamiento por codigo de respuesta / error:
//   - 200          → nil (key valida)
//   - 401          → errInvalidAPIKey (key rechazada por el server)
//   - timeout/red  → nil + warning (best-effort, no bloquea installs offline)
//   - 5xx/otro     → nil + warning (best-effort)
//
// Rationale best-effort: si el server esta caido pero la key tiene formato
// valido y matchea apiKeyPattern, preferimos seguir con el install (queda
// coherente con el contrato actual de `resolveAPIKey`). El usuario se va a
// enterar del problema cuando intente usar el MCP, igual que antes de este
// feature. Solo cambia el caso "401 explicito": ese SI bloquea porque es
// indicacion clara de key revocada o mal copiada.
func validateAPIKey(vpsURL, apiKey string) error {
	if vpsURL == "" {
		return nil // sin URL no hay que validar (defer al ping general)
	}
	ctx, cancel := context.WithTimeout(context.Background(), validateAPIKeyHTTPTimeout)
	defer cancel()

	url := strings.TrimRight(vpsURL, "/") + "/api/v1/auth/validate"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		warnL("no pude construir request de validacion (continuando): " + err.Error())
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		warnL("no pude validar la key contra " + vpsURL + " (continuando): " + err.Error())
		return nil
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	switch {
	case resp.StatusCode == http.StatusOK:
		return nil
	case resp.StatusCode == http.StatusUnauthorized:
		return errInvalidAPIKey
	default:
		warnL("validacion de key retorno HTTP " + strconv.Itoa(resp.StatusCode) + " (continuando)")
		return nil
	}
}

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

// validateMaxAttempts es la cantidad maxima de re-intentos cuando la key
// ingresada por prompt es rechazada por el server. Despues de N 401s,
// abortamos con error claro (no loop infinito).
const validateMaxAttempts = 3

// resolveAPIKey decide qué API key usar siguiendo la prioridad del contrato:
//
//	1) explicitKey (de --api-key) si no vacío → esa
//	2) OpenCode config (opencode.json → mcp.domain-mcp.headers.Authorization)
//	3) Claude Code config (~/.claude.json → mcpServers.domain-mcp.headers.Authorization)
//	4) Prompt al usuario (interactive, hidden) si no nonInteractive
//
// Si vpsURL != "", ademas valida la key contra el server con
// validateAPIKey. Si la validacion retorna errInvalidAPIKey:
//   - key de --api-key     → error directo (no re-prompteamos, el user la eligio)
//   - key de configs       → re-prompteamos hasta validateMaxAttempts
//   - key de prompt        → re-prompteamos hasta validateMaxAttempts
//
// Errores de red/timeout en la validacion NO bloquean (best-effort): si el
// server esta caido y la key tiene formato valido, dejamos pasar.
//
// Devuelve la key final o "" si no se pudo resolver.
// Si las keys de OpenCode y Claude Code son DIFERENTES, prioriza OpenCode
// (somos el mismo usuario) y devuelve (opencodeKey, claudeKey) para que el caller
// pueda loguear el warning.
//
// Nota: el caller debe pasar los paths ya resueltos (p.OpencodePath, p.ClaudeCodePath)
// para no acoplar resolveAPIKey con la detección de plataforma.
func resolveAPIKey(opencodePath, claudeCodePath, explicitKey, vpsURL string, in *bufio.Reader, nonInteractive bool) (key string, sources string, err error) {
	sources = ""

	// 1) Flag explícito gana sobre todo
	if explicitKey != "" {
		key, sources = explicitKey, "flag --api-key"
		if vpsURL == "" {
			return key, sources, nil
		}
		vErr := validateAPIKey(vpsURL, key)
		if vErr == nil {
			return key, sources, nil
		}
		if !errors.Is(vErr, errInvalidAPIKey) {
			// network/timeout: best-effort, aceptamos la key
			return key, sources, nil
		}
		// 401 con key explicita: error claro, sin re-prompt (user la puso a mano)
		return "", "", fmt.Errorf("API key de --api-key rechazada por el server (%s). Verifica que la copiaste bien o rotala en el VPS", vErr)
	}

	// 2-3) Buscar en configs existentes
	ocKey := readAPIKeyFromConfig(opencodePath, "mcp", "domain-mcp")
	ccKey := readAPIKeyFromConfig(claudeCodePath, "mcpServers", "domain-mcp")

	switch {
	case ocKey != "" && ccKey != "" && ocKey != ccKey:
		key, sources = ocKey, fmt.Sprintf("opencode=%s (claudecode=%s distinto, usando opencode)", mask(ocKey), mask(ccKey))
	case ocKey != "":
		key, sources = ocKey, "opencode"
	case ccKey != "":
		key, sources = ccKey, "claudecode"
	}

	if key != "" {
		if vpsURL != "" {
			vErr := validateAPIKey(vpsURL, key)
			if errors.Is(vErr, errInvalidAPIKey) {
				// Key de config invalida: caemos al prompt para re-intentar
				warnL("API key de " + sources + " fue rechazada por el server (revocada o desactualizada). Volvemos a pedirla.")
				return resolveFromPrompt(vpsURL, in, nonInteractive, validateMaxAttempts)
			}
			// nil (ok) o warning (network): aceptamos
		}
		return key, sources, nil
	}

	// 4) Sin key en ningún config → prompt (con loop de validacion)
	return resolveFromPrompt(vpsURL, in, nonInteractive, validateMaxAttempts)
}

// resolveFromPrompt maneja el flujo de pedir la key al usuario + re-intentar
// si el server la rechaza. Separado de resolveAPIKey para no anidar la
// recursion. Devuelve (key, "prompt", nil) en exito, o error tras agotar
// intentos / entrar en modo non-interactive.
func resolveFromPrompt(vpsURL string, in *bufio.Reader, nonInteractive bool, maxAttempts int) (string, string, error) {
	if nonInteractive {
		return "", "", fmt.Errorf("no hay API key en configs y --yes activo: pasá --api-key o corré --bootstrap")
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		prompted := promptHidden(in, "  API key (domk_live_xxx o domk_test_xxx): ")
		prompted = strings.TrimSpace(prompted)
		if prompted == "" {
			failL("API key vacia (intento " + strconv.Itoa(attempt) + "/" + strconv.Itoa(maxAttempts) + ")")
			continue
		}
		if !apiKeyPattern.MatchString(prompted) {
			failL("API key con formato invalido (esperado domk_(live|test)_...) (intento " + strconv.Itoa(attempt) + "/" + strconv.Itoa(maxAttempts) + ")")
			continue
		}

		// Formato OK. Validar contra server si tenemos URL.
		if vpsURL == "" {
			return prompted, "prompt", nil
		}
		vErr := validateAPIKey(vpsURL, prompted)
		if vErr == nil {
			return prompted, "prompt", nil
		}
		if !errors.Is(vErr, errInvalidAPIKey) {
			// network/timeout: best-effort, aceptamos (mismo criterio que arriba)
			warnL("no pude confirmar la key contra el server (continuando con la que diste)")
			return prompted, "prompt", nil
		}
		failL("API key rechazada por el server (invalida o revocada) (intento " + strconv.Itoa(attempt) + "/" + strconv.Itoa(maxAttempts) + ")")
	}
	return "", "", fmt.Errorf("API key sigue invalida tras %d intentos", maxAttempts)
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