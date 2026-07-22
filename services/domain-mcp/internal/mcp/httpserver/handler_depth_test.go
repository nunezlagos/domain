package httpserver

import (
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/auth/apikey"
	mcpserver "nunezlagos/domain/internal/mcp/server"
)

func TestParseDepth(t *testing.T) {
	require.Equal(t, 0, parseDepth(""), "ausente = 0")
	require.Equal(t, 0, parseDepth("abc"), "no numérico = 0")
	require.Equal(t, 0, parseDepth("-2"), "negativo = 0")
	require.Equal(t, 1, parseDepth("1"))
	require.Equal(t, 2, parseDepth("  2  "), "trim de espacios")
}

// depth>=1: el Principal efectivo clona y excluye las reentrantes; el base
// queda intacto (no se contamina el cache del resolver).
func TestScopePrincipalByDepth_Header1_ExcludesReentrant(t *testing.T) {
	base := &apikey.Principal{UserID: "u1"}
	got := mustScope(t, base, "1")

	require.NotSame(t, base, got, "debe ser un clon, no el mismo puntero")
	require.Nil(t, base.AllowedTools, "el Principal base no se muta")
	require.NotEmpty(t, got.AllowedTools)

	set := map[string]bool{}
	for _, n := range got.AllowedTools {
		set[n] = true
	}
	require.False(t, set["domain_agent_run"], "agent_run debe quedar fuera")
	require.False(t, set["domain_cron_create"], "cron_create debe quedar fuera")
	require.True(t, set["domain_mem_search"], "mem_search sigue permitida")
}

func TestScopePrincipalByDepth_NoHeader_Intacto(t *testing.T) {
	base := &apikey.Principal{UserID: "u1", AllowedTools: []string{"domain_mem_search"}}
	got := mustScope(t, base, "")
	require.Same(t, base, got, "sin header el Principal queda intacto")
}

func TestScopePrincipalByDepth_HeaderNoNumerico_TratadoComoCero(t *testing.T) {
	base := &apikey.Principal{UserID: "u1"}
	got := mustScope(t, base, "no-num")
	require.Same(t, base, got, "header inválido = depth 0 = intacto")
}

// token ya scoped: depth solo RESTRINGE (intersección), nunca amplía.
func TestScopePrincipalByDepth_TokenScoped_Interseca(t *testing.T) {
	base := &apikey.Principal{
		UserID:       "u1",
		AllowedTools: []string{"domain_mem_search", "domain_agent_run"},
	}
	got := mustScope(t, base, "1")
	require.Equal(t, []string{"domain_mem_search"}, got.AllowedTools,
		"depth quita la reentrante del allowlist preexistente")
}

func mustScope(t *testing.T, base *apikey.Principal, header string) *apikey.Principal {
	t.Helper()
	return scopePrincipalByDepth(base, mcpserver.Deps{}, header)
}
