package acp

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfig_LogValue_RedactsMcpToken(t *testing.T) {
	cfg := Config{
		Bin:      "opencode",
		McpURL:   "http://127.0.0.1:8000/mcp",
		McpToken: "domk_supersecret_token",
		Env:      []string{"DOMAIN_ANTHROPIC_KEY=sk-should-not-leak"},
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	logger.Info("acp cfg", slog.Any("cfg", cfg))
	out := buf.String()

	require.NotContains(t, out, "domk_supersecret_token", "el token no debe loguearse")
	require.NotContains(t, out, "sk-should-not-leak", "el env no debe loguearse")
	require.Contains(t, out, "***", "el token presente debe aparecer redactado")
	require.Contains(t, out, "http://127.0.0.1:8000/mcp")
}

func TestConfig_WithDefaults_PermissionModeDenyAll(t *testing.T) {
	require.Equal(t, PermissionDenyAll, Config{}.withDefaults().PermissionMode)
	require.Equal(t, "custom", Config{PermissionMode: "custom"}.withDefaults().PermissionMode)
}
