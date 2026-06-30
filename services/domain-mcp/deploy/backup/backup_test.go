package backup_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func scriptPath(t *testing.T, name string) string {
	t.Helper()

	wd, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return filepath.Join(wd, "deploy", "backup", name)
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatal("repo root not found")
		}
		wd = parent
	}
}

func TestBackupScript_BashSyntax(t *testing.T) {
	path := scriptPath(t, "backup.sh")
	cmd := exec.Command("bash", "-n", path)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "backup.sh syntax error: %s", string(out))
}

func TestRestoreScript_BashSyntax(t *testing.T) {
	path := scriptPath(t, "test-restore.sh")
	cmd := exec.Command("bash", "-n", path)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "test-restore.sh syntax error: %s", string(out))
}

func TestBackupScript_HasSetEuo(t *testing.T) {
	path := scriptPath(t, "backup.sh")
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(content), "set -euo pipefail")
}
