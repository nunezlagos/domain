package httpserver

import (
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/auth/apikey"
	mcpserver "nunezlagos/domain/internal/mcp/server"
)

const nativeTok = "sk-domain-native-acp-token"

func allowedSet(p *apikey.Principal) map[string]bool {
	set := map[string]bool{}
	for _, n := range p.AllowedTools {
		set[n] = true
	}
	return set
}

// CLAVE (cierra el fail-open): el bearer == token ACP nativo SIN header de depth
// igual queda acotado (fail-closed), aunque opencode no reenvíe el header.
func TestEffectivePrincipal_NativeToken_NoHeader_FailClosed(t *testing.T) {
	h := &Handler{NativeACPToken: nativeTok}
	base := &apikey.Principal{UserID: "acp"}
	got := h.effectivePrincipal(base, mcpserver.Deps{}, nativeTok, "")

	require.NotSame(t, base, got, "el token nativo debe clonar y restringir")
	require.NotEmpty(t, got.AllowedTools)
	set := allowedSet(got)
	require.False(t, set["domain_agent_run"], "reentrante excluida sin header")
	require.False(t, set["domain_orchestrate"], "reentrante excluida sin header")
	require.True(t, set["domain_mem_search"], "las seguras siguen permitidas")
}

// otro bearer sin header → sin restricción por este mecanismo.
func TestEffectivePrincipal_OtroToken_NoHeader_Intacto(t *testing.T) {
	h := &Handler{NativeACPToken: nativeTok}
	base := &apikey.Principal{UserID: "u1"}
	got := h.effectivePrincipal(base, mcpserver.Deps{}, "sk-otro-token", "")
	require.Same(t, base, got, "token no nativo sin header = Principal intacto")
}

// token nativo + header con depth explícito → sigue restringido (no amplía).
func TestEffectivePrincipal_NativeToken_ConHeader_SigueRestringido(t *testing.T) {
	h := &Handler{NativeACPToken: nativeTok}
	base := &apikey.Principal{UserID: "acp"}
	got := h.effectivePrincipal(base, mcpserver.Deps{}, nativeTok, "2")
	set := allowedSet(got)
	require.False(t, set["domain_agent_run"], "reentrante excluida con header")
	require.True(t, set["domain_mem_search"])
}

// token nativo NO configurado (vacío) → el mecanismo no aplica.
func TestEffectivePrincipal_NativeTokenVacio_Inactivo(t *testing.T) {
	h := &Handler{NativeACPToken: ""}
	base := &apikey.Principal{UserID: "u1"}
	got := h.effectivePrincipal(base, mcpserver.Deps{}, "cualquier-token", "")
	require.Same(t, base, got, "sin token nativo configurado no hay restricción")
}

// la comparación del token es en tiempo constante (subtle.ConstantTimeCompare):
// exacto matchea; distinto o longitud distinta no; vacío inactivo.
func TestIsNativeACPToken_ConstantTimeCompare(t *testing.T) {
	h := &Handler{NativeACPToken: nativeTok}
	require.True(t, h.isNativeACPToken(nativeTok), "match exacto")
	require.False(t, h.isNativeACPToken("sk-otro"), "distinto no matchea")
	require.False(t, h.isNativeACPToken(nativeTok+"x"), "longitud distinta no matchea")
	require.False(t, (&Handler{NativeACPToken: ""}).isNativeACPToken(""), "vacío = inactivo")
}
