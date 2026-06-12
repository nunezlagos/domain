// Tests para ensureLocalEnvFile (HU-01.13 commit 1/3).

package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnsureLocalEnvFile_SkipsIfEnvExists(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(oldWd)

	require.NoError(t, os.WriteFile(".env", []byte("EXISTING=1"), 0o600))

	require.NoError(t, ensureLocalEnvFile())

	// .env NO debe haber sido sobreescrito
	data, _ := os.ReadFile(".env")
	require.Equal(t, "EXISTING=1", string(data))
}

func TestEnsureLocalEnvFile_CopiesExampleIfMissing(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(oldWd)

	require.NoError(t, os.WriteFile(".env.example", []byte("KEY=value\nFOO=bar"), 0o600))

	require.NoError(t, ensureLocalEnvFile())

	data, err := os.ReadFile(".env")
	require.NoError(t, err)
	require.Equal(t, "KEY=value\nFOO=bar", string(data))

	// Permisos deben ser 0600 (secretos)
	info, err := os.Stat(".env")
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestEnsureLocalEnvFile_FailsIfExampleMissing(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(oldWd)

	err := ensureLocalEnvFile()
	require.Error(t, err)
	require.Contains(t, err.Error(), ".env.example not found")
	require.Contains(t, err.Error(), "domain project root")
}
