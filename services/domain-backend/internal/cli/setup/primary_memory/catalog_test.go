package primary_memory

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsMemoryProvider_Known(t *testing.T) {
	for _, name := range []string{"engram", "mem0", "memory", "knowledge", "recall", "cognee", "graphiti"} {
		require.True(t, IsMemoryProvider(name), "%s should be a memory provider", name)
	}
}

func TestIsMemoryProvider_Unknown(t *testing.T) {
	for _, name := range []string{"filesystem", "github", "fetch", "git", "time", "domain"} {
		require.False(t, IsMemoryProvider(name), "%s should NOT be a memory provider", name)
	}
}

func TestSortedNames_DeterministicOrder(t *testing.T) {
	providers := []DetectedProvider{
		{Name: "mem0"}, {Name: "engram"}, {Name: "cognee"},
	}
	got := SortedNames(providers)
	require.Equal(t, []string{"cognee", "engram", "mem0"}, got)
}

func TestSortedNames_Empty(t *testing.T) {
	require.Empty(t, SortedNames(nil))
	require.Empty(t, SortedNames([]DetectedProvider{}))
}
