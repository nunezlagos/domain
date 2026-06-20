package primary_memory

import (
	"os"
	"path/filepath"
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

func TestLoadCatalog_NoOverride_ReturnsHardcoded(t *testing.T) {
	tmp := t.TempDir()
	cat, err := loadCatalogFromPath(filepath.Join(tmp, "no-such-file.json"))
	require.NoError(t, err)
	for k := range KnownMemoryProviders {
		require.True(t, cat[k], "%s del hardcoded debe estar presente", k)
	}
}

func TestCatalog_OverrideFromJSON(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "primary-memory-catalog.json")
	body := `{
		"memory_providers": ["mycompany_memory"],
		"non_memory_providers": ["memory"]
	}`
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))

	cat, err := loadCatalogFromPath(path)
	require.NoError(t, err)
	// El nuevo provider del override está.
	require.True(t, cat["mycompany_memory"], "override debe agregar mycompany_memory")
	// engram (del hardcoded) sigue ahí.
	require.True(t, cat["engram"], "hardcoded debe persistir")
	// "memory" del hardcoded fue removido por non_memory_providers.
	require.False(t, cat["memory"], "non_memory_providers debe ganar")
}

func TestLoadCatalog_MalformedJSON_FallsBackToHardcoded(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "bad.json")
	require.NoError(t, os.WriteFile(path, []byte(`{{{invalid`), 0o600))
	cat, err := loadCatalogFromPath(path)
	require.NoError(t, err)
	require.True(t, cat["engram"], "fallback al hardcoded en JSON malformado")
}
