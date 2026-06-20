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
		{"Migración de postgres: la migración requiere postgres y pgvector", "migración-postgres-requiere"},
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
	require.Equal(t, a, SuggestTopicKey(content), "determinístico")
	require.NotContains(t, a, " ")
	require.Contains(t, a, "-")
}
