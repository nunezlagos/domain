package mcpserver

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// REQ-54 issue-54.4: la clasificación del prompt en el capture es lo que el
// hook UserPromptSubmit convierte en señal de orquestación. Estos tests fijan
// el mapeo complejidad→acción.

func TestClassifyCapturedPrompt_Complex_SuggestsOrchestrate(t *testing.T) {
	t.Parallel()
	c := classifyCapturedPrompt("implementar un nuevo módulo de pagos con migración de esquema y refactor del servicio")
	require.Equal(t, "complex", c["complexity"])
	require.Equal(t, "orchestrate", c["suggested_action"])
	require.Equal(t, "full", c["suggested_mode"])
}

func TestClassifyCapturedPrompt_Moderate_SuggestsOrchestrateLite(t *testing.T) {
	t.Parallel()
	// Sin intent match y corto → moderate (default del heurístico).
	c := classifyCapturedPrompt("necesito revisar el flujo de autenticación del panel")
	require.Equal(t, "moderate", c["complexity"])
	require.Equal(t, "orchestrate", c["suggested_action"])
	require.Equal(t, "lite", c["suggested_mode"])
}

func TestClassifyCapturedPrompt_Trivial_SuggestsNone(t *testing.T) {
	t.Parallel()
	c := classifyCapturedPrompt("fix typo en el README")
	require.Equal(t, "trivial", c["complexity"])
	require.Equal(t, "none", c["suggested_action"])
	require.NotContains(t, c, "suggested_mode")
}

func TestClassifyCapturedPrompt_Simple_SuggestsTicket(t *testing.T) {
	t.Parallel()
	c := classifyCapturedPrompt("fix bug in el endpoint de login que devuelve 500")
	require.Equal(t, "simple", c["complexity"])
	require.Equal(t, "ticket", c["suggested_action"])
}
