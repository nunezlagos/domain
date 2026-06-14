package install

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestBackupEnv_10RunsOneBackup es el e2e sabotaje-resistente
// del spec T-e2e-1: 10 corridas de BackupEnv sin cambios deben
// producir 1 solo .bak.<ts> (no 10).
//
// Si la dedup está deshabilitada (sabotaje), este test DEBE FALLAR.
func TestBackupEnv_10RunsOneBackup(t *testing.T) {
	dir := t.TempDir()

	// Crear un .env simulado
	envPath := filepath.Join(dir, ".env")
	require.NoError(t, os.WriteFile(envPath, []byte("KEY=value"), 0o600))

	// Cambiar el cwd para que BackupEnv (que lee ".env" relativo) lo encuentre.
	originalCwd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalCwd) }()
	require.NoError(t, os.Chdir(dir))

	// Limpiar cualquier .env.bak.* previo
	matches, _ := filepath.Glob(filepath.Join(dir, ".env.bak.*"))
	for _, m := range matches {
		_ = os.Remove(m)
	}

	// Sleep 1.1s para que las 10 corridas tengan timestamps
	// DISTINTOS. Sin esto, las 10 podrían caer en el mismo segundo
	// y el comportamiento preexistente sobrescribiría.
	time.Sleep(1100 * time.Millisecond)

	// Correr BackupEnv 10 veces (con sleep entre cada una para
	// que tengan timestamps distintos — el formato .bak tiene
	// precisión de segundos y el comportamiento preexistente
	// sobrescribe si el nombre coincide)
	for i := 0; i < 10; i++ {
		_, _ = BackupEnv(0) // keepLast=0 = sin prune
		time.Sleep(1100 * time.Millisecond)
	}

	// Verificar que hay 1 solo .bak.<ts>
	matches, _ = filepath.Glob(filepath.Join(dir, ".env.bak.*"))
	require.Equal(t, 1, len(matches),
		"con dedup, 10 corridas sin cambios deben producir 1 .bak; got %d", len(matches))
}
