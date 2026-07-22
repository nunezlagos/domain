package acp

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConfig_WithDefaults_FillsEmptyFields(t *testing.T) {
	got := Config{}.withDefaults()
	require.Equal(t, "opencode", got.Bin)
	require.Equal(t, []string{"acp"}, got.Args)
	require.Equal(t, defaultTimeout, got.Timeout)
}

func TestConfig_WithDefaults_KeepsProvidedValues(t *testing.T) {
	got := Config{Bin: "custom", Args: []string{"run"}, Timeout: time.Second}.withDefaults()
	require.Equal(t, "custom", got.Bin)
	require.Equal(t, []string{"run"}, got.Args)
	require.Equal(t, time.Second, got.Timeout)
}

func TestSpawn_MissingBinary_ReturnsError(t *testing.T) {
	_, err := Spawn(context.Background(), Config{Bin: "domain-acp-nonexistent-xyz"}, nil)
	require.Error(t, err)
}

func TestProcess_Close_Idempotent(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep no disponible")
	}
	p, err := Spawn(context.Background(), Config{Bin: "sleep", Args: []string{"30"}, Cwd: "/tmp"}, nil)
	require.NoError(t, err)

	require.NotPanics(t, func() {
		_ = p.Close()
		_ = p.Close()
	})
}
