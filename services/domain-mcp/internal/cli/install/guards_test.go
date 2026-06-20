package install

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestIsProjectRoot_OK: tempdir con ambos archivos retorna (true, nil, nil).
func TestIsProjectRoot_OK(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env.example"), []byte("EXAMPLE"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte("COMPOSE"), 0o600))

	ok, missing, err := IsProjectRoot(dir)
	require.NoError(t, err)
	require.True(t, ok, "expected IsProjectRoot to be true for a valid project root")
	require.Empty(t, missing, "expected no missing files for a valid project root")
}

// TestIsProjectRoot_MissingOne: tempdir con solo .env.example -> (false, ["docker-compose.yml"], nil).
func TestIsProjectRoot_MissingOne(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env.example"), []byte("EXAMPLE"), 0o600))

	ok, missing, err := IsProjectRoot(dir)
	require.NoError(t, err)
	require.False(t, ok)
	require.Equal(t, []string{"docker-compose.yml"}, missing)
}

// TestIsProjectRoot_Empty: tempdir vacío -> (false, [".env.example", "docker-compose.yml"], nil).
func TestIsProjectRoot_Empty(t *testing.T) {
	dir := t.TempDir()

	ok, missing, err := IsProjectRoot(dir)
	require.NoError(t, err)
	require.False(t, ok)
	require.ElementsMatch(t, []string{".env.example", "docker-compose.yml"}, missing)
}

// TestIsProjectRoot_NotExist: path inexistente -> error real.
func TestIsProjectRoot_NotExist(t *testing.T) {
	dir := t.TempDir()
	bogus := filepath.Join(dir, "no-existe")

	ok, missing, err := IsProjectRoot(bogus)
	require.Error(t, err, "path inexistente debe retornar error")
	require.False(t, ok)
	require.Nil(t, missing)
}
