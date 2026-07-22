package agentrunner

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunner_Run_ACPNativeNil_UsesLegacy(t *testing.T) {
	t.Parallel()
	// Sin ACPNative cableado, Run debe seguir el path legacy (tool-loop).
	r := &Runner{}
	require.False(t, r.usesNativeACP(), "ACPNative=nil no debe activar el path nativo")
}

func TestRunner_Run_ACPNativeSet_UsesNative(t *testing.T) {
	t.Parallel()
	r := &Runner{ACPNative: func(context.Context) (ACPTurn, error) { return nil, nil }}
	require.True(t, r.usesNativeACP(), "ACPNative cableado debe activar el path nativo")
}

func TestNativePrompt_ComposesSystemAndUser(t *testing.T) {
	t.Parallel()
	require.Equal(t, "sys\n\nuser", nativePrompt("sys", "user"))
	require.Equal(t, "user", nativePrompt("", "user"))
	require.Equal(t, "sys", nativePrompt("  sys  ", ""))
}
