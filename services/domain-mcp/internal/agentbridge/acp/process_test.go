package acp

import (
	"context"
	"os"
	"os/exec"
	"strings"
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

// countWorkspaces cuenta los dirs acp-ws-* dentro de dir.
func countWorkspaces(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	n := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "acp-ws-") {
			n++
		}
	}
	return n
}

// Fix DOMAINSERV-85: un Spawn nativo (McpURL set) que falla tras crear el
// workspace NO debe dejar el temp dir acp-ws-* huérfano en /tmp.
func TestSpawn_NativeError_NoLeakWorkspace(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)

	_, err := Spawn(context.Background(), Config{
		Bin:            "/nonexistent/opencode-does-not-exist",
		McpURL:         "http://127.0.0.1:1/mcp",
		McpToken:       "tok",
		PermissionMode: PermissionDenyAll,
	}, nil)
	require.Error(t, err, "spawn con binario inexistente debe fallar")

	require.Equal(t, 0, countWorkspaces(t, tmp),
		"el workspace debe limpiarse en el error-path de Spawn")
}
