package flow

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// DOMAINSERV-218: el gate de batch-mode acota las ediciones de una sub-tarea a los
// globs de su allowlist. Para que dos sub-tareas puedan correr en paralelo sin
// pisarse, el solapamiento tiene que rechazarse al EMITIR el token — al editar ya
// es tarde: el conflicto aparece como un deny confuso en un agente cualquiera.

func TestValidarAllowlist_GlobSinPrefijoLiteral_EsRechazado(t *testing.T) {
	casos := []string{"*", "**", "**/*.go", "*.go", "?ervices/**", "[sd]ervices/**"}
	for _, g := range casos {
		err := ValidarAllowlist([]string{g})
		require.ErrorIs(t, err, ErrAllowlistSinPrefijo,
			"%q tiene scope vacío: como allowlist no acota nada y hace que todo se solape", g)
	}
}

func TestValidarAllowlist_GlobVacio_EsRechazado(t *testing.T) {
	require.ErrorIs(t, ValidarAllowlist([]string{""}), ErrAllowlistSinPrefijo)
	require.ErrorIs(t, ValidarAllowlist([]string{"   "}), ErrAllowlistSinPrefijo)
}

func TestValidarAllowlist_GlobConPrefijoDeDirectorio_EsAceptado(t *testing.T) {
	casos := []string{
		"services/domain-mcp/**",
		"services/domain-mcp/internal/observability/*.go",
		"install-user/hooks/*.sh",
		"services/install.sh",
		"docs/README.md",
	}
	for _, g := range casos {
		require.NoError(t, ValidarAllowlist([]string{g}), "%q sí acota un territorio", g)
	}
}

func TestValidarAllowlist_SinGlobs_EsAceptado(t *testing.T) {
	// vacío = flow normal sin batch-mode. La retrocompatibilidad es deliberada:
	// el gate ya trata "sin allowlist" como "sin restricción de path".
	require.NoError(t, ValidarAllowlist(nil))
	require.NoError(t, ValidarAllowlist([]string{}))
}

func TestValidarParticionDisjunta_ScopesDisjuntos_EsAceptada(t *testing.T) {
	err := ValidarParticionDisjunta([][]string{
		{"services/domain-mcp/internal/observability/**"},
		{"install-user/hooks/**"},
		{"services/domain-admin/**"},
	})
	require.NoError(t, err, "tres sub-tareas en directorios distintos pueden correr en paralelo")
}

func TestValidarParticionDisjunta_MismoScope_EsRechazada(t *testing.T) {
	err := ValidarParticionDisjunta([][]string{
		{"services/domain-mcp/internal/observability/**"},
		{"services/domain-mcp/internal/observability/*.go"},
	})
	require.ErrorIs(t, err, ErrAllowlistSolapada,
		"dos allowlists sobre el mismo directorio se pisan aunque los globs difieran")
}

func TestValidarParticionDisjunta_ScopeAncestro_EsRechazada(t *testing.T) {
	err := ValidarParticionDisjunta([][]string{
		{"services/**"},
		{"services/domain-mcp/internal/**"},
	})
	require.ErrorIs(t, err, ErrAllowlistSolapada,
		"un ancestro contiene al descendiente: el agente del scope amplio puede editar lo del otro")
}

// El falso positivo que hay que NO cometer: comparar por prefijo de string en vez
// de por segmentos de path. "services/domain" es prefijo de "services/domain-mcp"
// como texto, pero NO es su directorio ancestro.
func TestValidarParticionDisjunta_PrefijoDeStringQueNoEsAncestro_EsAceptada(t *testing.T) {
	err := ValidarParticionDisjunta([][]string{
		{"services/domain/**"},
		{"services/domain-mcp/**"},
	})
	require.NoError(t, err,
		"services/domain no es ancestro de services/domain-mcp: son hermanos y no se pisan")
}

func TestValidarParticionDisjunta_ElSolapamientoSeReportaConSusIndices(t *testing.T) {
	err := ValidarParticionDisjunta([][]string{
		{"install-user/hooks/**"},
		{"services/domain-mcp/**"},
		{"services/domain-mcp/internal/**"},
	})
	require.ErrorIs(t, err, ErrAllowlistSolapada)
	// quien orquesta necesita saber CUÁLES corregir, no solo que hay conflicto
	require.Contains(t, err.Error(), "#1")
	require.Contains(t, err.Error(), "#2")
	require.NotContains(t, err.Error(), "#0", "la #0 no está en conflicto y no debe aparecer")
}

// Un glob indecidible dentro de una partición se rechaza por su propia
// malformación, antes de intentar compararlo: si se comparara, su scope vacío no
// choca con nada y la partición pasaría en verde con un agente sin acotar.
func TestValidarParticionDisjunta_GlobIndecidible_SeRechazaAntesDeComparar(t *testing.T) {
	err := ValidarParticionDisjunta([][]string{
		{"install-user/hooks/**"},
		{"**/*.go"},
	})
	require.ErrorIs(t, err, ErrAllowlistSinPrefijo)
	require.Contains(t, err.Error(), "#1")
}

func TestScopeDe_DerivaElDirectorioAcotado(t *testing.T) {
	casos := map[string]string{
		"services/domain-mcp/**":                          "services/domain-mcp",
		"services/domain-mcp/internal/observability/*.go": "services/domain-mcp/internal/observability",
		"install-user/hooks/*.sh":                         "install-user/hooks",
		"services/install.sh":                             "services",
		"services/domain-mcp/":                            "services/domain-mcp",
		"**/*.go":                                         "",
		"*":                                               "",
	}
	for glob, esperado := range casos {
		require.Equal(t, esperado, scopeDe(glob), "scope de %q", glob)
	}
}
