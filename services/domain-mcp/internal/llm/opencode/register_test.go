package opencode

import (
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/llm"
)

func TestRegister_NoURL_Skips(t *testing.T) {
	t.Setenv(URLEnv, "")
	f := llm.NewFactory()
	require.False(t, Register(f, nil, nil))
	_, err := f.Get(ProviderName)
	require.Error(t, err)
}

func TestRegister_WithURL_RegistersAndSetsDefault(t *testing.T) {
	t.Setenv(URLEnv, "http://opencode:4096")
	t.Setenv("DOMAIN_LLM_PROVIDER", "")
	f := llm.NewFactory()
	require.True(t, Register(f, nil, nil))

	p, err := f.Get(ProviderName)
	require.NoError(t, err)
	require.Equal(t, ProviderName, p.Name())

	def, err := f.GetDefault()
	require.NoError(t, err)
	require.Equal(t, ProviderName, def.Name())
}
