package httpserver

import (
	"crypto/subtle"
	"strconv"
	"strings"

	"nunezlagos/domain/internal/auth/apikey"
	mcpserver "nunezlagos/domain/internal/mcp/server"
)

// effectivePrincipal aplica la barrera anti-reentrancia (DOMAINSERV-85) al
// Principal resuelto. Dos defensas complementarias:
//   - header X-Domain-Agent-Depth: lo pone el bridge en el path nativo.
//   - reconocimiento server-side del token ACP nativo: si el bearer entrante es
//     el del mcpServer nativo se fuerza depth>=1 aunque el header NO venga
//     (fail-closed) — opencode podría no reenviarlo. Si el header trae un depth
//     mayor, se respeta (max). Cualquier otro token: manda el header.
func (h *Handler) effectivePrincipal(base *apikey.Principal, deps mcpserver.Deps, token, depthHeader string) *apikey.Principal {
	depth := parseDepth(depthHeader)
	if h.isNativeACPToken(token) && depth < 1 {
		depth = 1
	}
	return scopePrincipalByDepth(base, deps, depth)
}

// isNativeACPToken compara el bearer contra el token ACP nativo en tiempo
// constante (evita side-channel por timing). Token no configurado = inactivo.
func (h *Handler) isNativeACPToken(token string) bool {
	if h.NativeACPToken == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(h.NativeACPToken)) == 1
}

// scopePrincipalByDepth clona el Principal restringiendo su allowlist a las
// tools no reentrantes cuando depth>=1; si ya venía scoped, interseca (nunca
// amplía). depth<1 lo deja intacto. Clonar evita contaminar el cache del resolver.
func scopePrincipalByDepth(base *apikey.Principal, deps mcpserver.Deps, depth int) *apikey.Principal {
	if depth < 1 {
		return base
	}
	clone := *base
	clone.AllowedTools = mcpserver.AllowedToolsForDepthScoped(deps, depth, base.AllowedTools)
	return &clone
}

// parseDepth interpreta el header de profundidad. Ausente/inválido = 0.
func parseDepth(v string) int {
	if v == "" {
		return 0
	}
	d, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || d < 0 {
		return 0
	}
	return d
}
