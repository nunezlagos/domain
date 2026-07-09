package install

import (
	"os"
	"path/filepath"
	"testing"
)

// TestBackupFile_Rotation_FiveExisting_KeepsThree verifica que BackupFile
// (wrapper genérico) poda a los 3 backups más recientes. Antes del fix el
// wrapper pasaba keepLast=0 y no podaba.
func TestBackupFile_Rotation_FiveExisting_KeepsThree(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(path, []byte("contenido actual"), 0o600); err != nil {
		t.Fatalf("write original: %v", err)
	}

	// 5 backups previos con timestamps crecientes (lex = cronológico).
	stamps := []string{
		"20260101T000001Z", "20260101T000002Z", "20260101T000003Z",
		"20260101T000004Z", "20260101T000005Z",
	}
	for _, ts := range stamps {
		if err := os.WriteFile(path+".bak."+ts, []byte("viejo "+ts), 0o600); err != nil {
			t.Fatalf("write backup %s: %v", ts, err)
		}
	}

	if _, err := BackupFile(path); err != nil {
		t.Fatalf("BackupFile: %v", err)
	}

	backups, err := ListBackups(path)
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}
	if len(backups) != 3 {
		t.Fatalf("esperaba 3 backups tras la poda, hay %d: %v", len(backups), backups)
	}
}
