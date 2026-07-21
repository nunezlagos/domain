package agentrunner

import (
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/llm"
)

// Verifica que, sin provider LLM registrado, la resolución degrada a un error
// estructurado y detectable (ErrAgentLLMUnavailable / ErrProviderMissing) en
// vez de crashear. Path aislado de la DB (DOMAINSERV-58).
func TestRunner_ResolveProvider_EmptyFactory_ReturnsStructuredError(t *testing.T) {
	r := &Runner{Factory: llm.NewFactory()}
	_, err := r.resolveProvider("anthropic")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrAgentLLMUnavailable)
	require.ErrorIs(t, err, ErrProviderMissing)
}
