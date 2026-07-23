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

func TestDedupEnv_HomeDuplicado_GanaElUltimo(t *testing.T) {
	got := dedupEnv([]string{"HOME=/a", "PATH=/bin", "HOME=/b"})

	require.Contains(t, got, "HOME=/b", "el HOME de cfg.Env (último) gana")
	require.NotContains(t, got, "HOME=/a")
	require.Contains(t, got, "PATH=/bin")

	homeCount := 0
	for _, kv := range got {
		if strings.HasPrefix(kv, "HOME=") {
			homeCount++
		}
	}
	require.Equal(t, 1, homeCount, "HOME aparece una sola vez")
}
