package issuebuilder

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// DOMAINSERV-210: el ordinal salía de COUNT(*) de los issues del REQ, así que borrar
// un issue hace que el siguiente reuse un ordinal ya emitido y dos changes distintos
// terminan compartiendo el número. El ordinal deja de identificar unívocamente un
// issue bajo un REQ, que es justamente para lo que sirve.
func TestOrdinalDesdeSlug_ExtraeElNumeroDeIssueBajoElREQ(t *testing.T) {
	casos := []struct {
		slug     string
		esperado int
	}{
		{"issue-54.1-tool-channels-sync", 1},
		{"issue-54.12-algo", 12},
		{"issue-3.7", 7},
		{"issue-161.2-fan-out", 2},
	}
	for _, c := range casos {
		t.Run(c.slug, func(t *testing.T) {
			require.Equal(t, c.esperado, ordinalDesdeSlug(c.slug))
		})
	}
}

// Un slug que no sigue el patrón no puede contaminar el máximo con un número al azar.
func TestOrdinalDesdeSlug_SlugAjeno_DevuelveCero(t *testing.T) {
	ajenos := []string{
		"",
		"HU-54.1-viejo-formato",
		"issue-sin-numero",
		"issue-54-sin-ordinal",
		"prefijo-issue-54.9-no-ancla",
	}
	for _, s := range ajenos {
		t.Run(s, func(t *testing.T) {
			require.Zero(t, ordinalDesdeSlug(s), "%q no es un slug de issue bajo un REQ", s)
		})
	}
}

// El invariante que el COUNT rompía: el siguiente ordinal siempre es mayor que
// TODOS los ya emitidos, aunque falten números en el medio por borrados.
func TestSiguienteOrdinal_ConHuecos_NoReusaUnNumeroYaEmitido(t *testing.T) {
	emitidos := []string{
		"issue-54.1-tool-channels-sync",
		"issue-54.3-otro",
	}

	siguiente := siguienteOrdinal(emitidos)

	require.Equal(t, 4, siguiente,
		"con 2 issues pero máximo 3, el COUNT+1 daría 3 y colisionaría con issue-54.3")
}

func TestSiguienteOrdinal_SinIssues_EmpiezaEnUno(t *testing.T) {
	require.Equal(t, 1, siguienteOrdinal(nil))
	require.Equal(t, 1, siguienteOrdinal([]string{}))
}

func TestSiguienteOrdinal_SlugsAjenos_NoInflanElMaximo(t *testing.T) {
	require.Equal(t, 1, siguienteOrdinal([]string{"HU-viejo", "issue-sin-numero"}))
}
