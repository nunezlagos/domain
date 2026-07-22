package acp

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProcess_Env_NoServerSecrets(t *testing.T) {
	t.Setenv("DOMAIN_ANTHROPIC_KEY", "sk-fake-server-secret")
	t.Setenv("PATH", "/usr/bin")

	env := scrubbedEnv([]string{"OPENCODE_MCP_URL=http://127.0.0.1:8000/mcp"})

	joined := strings.Join(env, "\n")
	require.NotContains(t, joined, "DOMAIN_ANTHROPIC_KEY", "ningún secreto del server debe heredarse")
	require.NotContains(t, joined, "sk-fake-server-secret")
	require.Contains(t, joined, "PATH=/usr/bin", "PATH está en la allowlist")
	require.Contains(t, joined, "OPENCODE_MCP_URL=http://127.0.0.1:8000/mcp", "cfg.Env sí pasa")
}
