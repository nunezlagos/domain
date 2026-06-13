package install

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestBackupFile_Dedup_SameContent: 5 corridas sin cambios -> 1 solo .bak
// y Deduplicated: true en cada llamada.
func TestBackupFile_Dedup_SameContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello"), 0o600))

	// Primer backup: crea .bak.<ts1>
	res1, err := backupFile(path, 0)
	require.NoError(t, err)
	require.NotNil(t, res1)
	require.False(t, res1.Deduplicated, "primer backup no es dedup")

	// Contar archivos .bak.* actuales
	countBak := func() int {
		matches, _ := filepath.Glob(path + ".bak.*")
		return len(matches)
	}
	require.Equal(t, 1, countBak(), "debe haber 1 .bak después del primer backup")

	// 4 corridas más sin cambios -> todas dedup, sin nuevos .bak
	for i := 0; i < 4; i++ {
		res, err := backupFile(path, 0)
		require.NoError(t, err)
		require.NotNil(t, res)
		require.True(t, res.Deduplicated, "corrida #%d debería ser dedup", i+2)
		require.Equal(t, 1, countBak(), "no debe haber nuevos .bak files después de la corrida #%d", i+2)
	}
}

// TestBackupFile_Dedup_ChangedContent: cambio el archivo -> 2 .bak.
func TestBackupFile_Dedup_ChangedContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(path, []byte("v1"), 0o600))

	res1, err := backupFile(path, 0)
	require.NoError(t, err)
	require.False(t, res1.Deduplicated)

	// Sleep para garantizar distinto timestamp (formato .bak.YYYYMMDDTHHMMSSZ
	// tiene precisión de segundos; dos corridas en el mismo segundo
	// sobrescribirían el archivo, comportamiento preexistente).
	time.Sleep(1100 * time.Millisecond)

	// Cambiar el contenido
	require.NoError(t, os.WriteFile(path, []byte("v2"), 0o600))

	res2, err := backupFile(path, 0)
	require.NoError(t, err)
	require.False(t, res2.Deduplicated, "cambio de contenido no debe ser dedup")

	matches, _ := filepath.Glob(path + ".bak.*")
	require.Equal(t, 2, len(matches), "debe haber 2 .bak (v1 y v2)")
}

// TestBackupFile_Dedup_FirstTime: sin backups previos -> 1 .bak, Deduplicated=false.
func TestBackupFile_Dedup_FirstTime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(path, []byte("first"), 0o600))

	res, err := backupFile(path, 0)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.False(t, res.Deduplicated, "primer backup no es dedup")
}
