package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// issue-65.1 (hallazgo juez B): writeSettingsOrWarn debe crear backup del
// settings.json antes de escribirlo (usado al desactivar engram).
func TestWriteSettingsOrWarn_CreaBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(`{"enabledPlugins":{"engram@engram":true}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	writeSettingsOrWarn(path, map[string]any{"enabledPlugins": map[string]any{}})

	backups, _ := filepath.Glob(path + ".backup-*")
	if len(backups) == 0 {
		t.Error("no se creó backup del settings.json antes de escribir (desactivar engram)")
	}
	raw, _ := os.ReadFile(path)
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("settings.json inválido tras write: %v", err)
	}
}
