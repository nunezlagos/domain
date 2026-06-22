package mcpserver

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSuggestTopicKey(t *testing.T) {
	cases := []struct {
		content string
		want    string
	}{
		{"Migracion de postgres: la migracion requiere postgres y pgvector", "migracion-postgres-requiere"},
		{"", "general"},
		{"el la de y o", "general"},
		{"deploy", "deploy"},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, SuggestTopicKey(tc.content), tc.content)
	}
}

func TestSuggestTopicKey_StableAndKebab(t *testing.T) {
	content := "Circuit breaker para webhooks: el circuit breaker abre tras N fallos de webhooks"
	a := SuggestTopicKey(content)
	require.Equal(t, a, SuggestTopicKey(content), "deterministico")
	require.NotContains(t, a, " ")
	require.Contains(t, a, "-")
}
