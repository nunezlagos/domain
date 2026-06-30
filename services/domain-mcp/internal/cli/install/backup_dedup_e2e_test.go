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


	envPath := filepath.Join(dir, ".env")
	require.NoError(t, os.WriteFile(envPath, []byte("KEY=value"), 0o600))


	originalCwd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalCwd) }()
	require.NoError(t, os.Chdir(dir))


	matches, _ := filepath.Glob(filepath.Join(dir, ".env.bak.*"))
	for _, m := range matches {
		_ = os.Remove(m)
	}




	time.Sleep(1100 * time.Millisecond)





	for i := 0; i < 10; i++ {
		_, _ = BackupEnv(0) // keepLast=0 = sin prune
		time.Sleep(1100 * time.Millisecond)
	}


	matches, _ = filepath.Glob(filepath.Join(dir, ".env.bak.*"))
	require.Equal(t, 1, len(matches),
		"con dedup, 10 corridas sin cambios deben producir 1 .bak; got %d", len(matches))
}
