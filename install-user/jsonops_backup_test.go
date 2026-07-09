package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBackupIfExists_DedupAndPrune verifica que backupIfExists (1) no crea un
// backup nuevo si el contenido no cambió respecto del último backup, y (2) poda
// a los 3 backups más recientes.
func TestBackupIfExists_DedupAndPrune(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	// dedup: mismo contenido en archivo y en el último backup → 0 backups nuevos
	if err := os.WriteFile(path, []byte("KEY=abc"), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}
	if err := os.WriteFile(path+".backup-20260101T000000Z", []byte("KEY=abc"), 0o600); err != nil {
		t.Fatalf("write backup previo: %v", err)
	}
	if _, err := backupIfExists(path, "20260101T000010Z"); err != nil {
		t.Fatalf("backupIfExists dedup: %v", err)
	}
	if n := countBackups(t, dir); n != 1 {
		t.Fatalf("dedup: esperaba 1 backup (sin nuevo), hay %d", n)
	}

	// poda: 5 backups distintos + contenido nuevo → quedan 3
	if err := os.WriteFile(path, []byte("KEY=nuevo"), 0o600); err != nil {
		t.Fatalf("write env nuevo: %v", err)
	}
	stamps := []string{
		"20260101T000001Z", "20260101T000002Z", "20260101T000003Z",
		"20260101T000004Z", "20260101T000005Z",
	}
	// borrar el previo del bloque dedup para partir limpio con 5
	_ = os.Remove(path + ".backup-20260101T000000Z")
	for _, ts := range stamps {
		if err := os.WriteFile(path+".backup-"+ts, []byte("viejo "+ts), 0o600); err != nil {
			t.Fatalf("write backup %s: %v", ts, err)
		}
	}
	if _, err := backupIfExists(path, "20260101T000006Z"); err != nil {
		t.Fatalf("backupIfExists prune: %v", err)
	}
	if n := countBackups(t, dir); n != 3 {
		t.Fatalf("poda: esperaba 3 backups, hay %d", n)
	}
}

func countBackups(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	var n int
	for _, e := range entries {
		if strings.Contains(e.Name(), ".backup-") {
			n++
		}
	}
	return n
}
