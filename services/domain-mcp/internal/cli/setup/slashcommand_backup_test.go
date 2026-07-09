package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInstallSlashCommand_SameContent_NoBackup verifica que correr
// InstallSlashCommand dos veces con el mismo contenido NO crea un backup en la
// segunda corrida. Antes del fix renombraba incondicionalmente en cada corrida
// (el hook SessionStart la dispara cada sesión → cientos de backups).
func TestInstallSlashCommand_SameContent_NoBackup(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, err := InstallSlashCommand(AgentClaudeCode); err != nil {
		t.Fatalf("primera corrida: %v", err)
	}
	if _, err := InstallSlashCommand(AgentClaudeCode); err != nil {
		t.Fatalf("segunda corrida: %v", err)
	}

	cmdDir := filepath.Join(home, ".claude", "commands")
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		t.Fatalf("read commands dir: %v", err)
	}
	var backups int
	for _, e := range entries {
		if strings.Contains(e.Name(), "domain-login.md.") {
			backups++
		}
	}
	if backups != 0 {
		t.Fatalf("esperaba 0 backups con contenido idéntico, hay %d", backups)
	}
}

// TestInstallSlashCommand_UsesBakConvention verifica que cuando el contenido
// cambia, el backup usa la convención .bak.<ts> (reconocible por el restore del
// server), no .backup-<ts>.
func TestInstallSlashCommand_UsesBakConvention(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cmdDir := filepath.Join(home, ".claude", "commands")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(cmdDir, "domain-login.md")
	if err := os.WriteFile(path, []byte("contenido viejo distinto"), 0o644); err != nil {
		t.Fatalf("write viejo: %v", err)
	}

	if _, err := InstallSlashCommand(AgentClaudeCode); err != nil {
		t.Fatalf("InstallSlashCommand: %v", err)
	}

	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		t.Fatalf("read commands dir: %v", err)
	}
	var found bool
	for _, e := range entries {
		if strings.Contains(e.Name(), "domain-login.md.bak.") {
			found = true
		}
		if strings.Contains(e.Name(), "domain-login.md.backup-") {
			t.Fatalf("no debe usar la convención .backup-<ts>: %s", e.Name())
		}
	}
	if !found {
		t.Fatalf("esperaba un backup .bak.<ts> del contenido viejo")
	}
}
