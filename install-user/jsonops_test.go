package main

import (
	"os"
	"path/filepath"
	"testing"
)

// upsertMCPEntry: migra entry legacy "domain" + planta "domain-mcp" sin
// pisar otros servers del usuario.
func TestUpsertMCPEntry_PreservesOthersAndMigratesLegacy(t *testing.T) {
	m := map[string]any{
		"mcpServers": map[string]any{
			"domain": map[string]any{"url": "OLD"},        // legacy a migrar
			"atlassian": map[string]any{"url": "https://x"}, // del usuario, NO tocar
			"slack":     map[string]any{"command": "node"},  // del usuario, NO tocar
		},
	}
	upsertMCPEntry(m, "mcpServers", map[string]any{
		"url":     "https://new",
		"headers": map[string]any{"Authorization": "Bearer NEW"},
	})
	servers := m["mcpServers"].(map[string]any)
	if _, ok := servers["domain"]; ok {
		t.Error("legacy 'domain' debería haberse migrado/eliminado")
	}
	newEntry, ok := servers["domain-mcp"].(map[string]any)
	if !ok {
		t.Fatal("entry 'domain-mcp' no se creó")
	}
	if newEntry["url"] != "https://new" {
		t.Errorf("url = %v, want https://new", newEntry["url"])
	}
	if _, ok := servers["atlassian"]; !ok {
		t.Error("atlassian del usuario fue pisado!")
	}
	if _, ok := servers["slack"]; !ok {
		t.Error("slack del usuario fue pisado!")
	}
}

// removeMCPEntry: borra domain-mcp Y domain (legacy), preserva otros.
// Si el container queda vacío, lo elimina del JSON.
func TestRemoveMCPEntry_OnlyOurs(t *testing.T) {
	m := map[string]any{
		"mcpServers": map[string]any{
			"domain":     map[string]any{},
			"domain-mcp": map[string]any{},
			"other":      map[string]any{"url": "keep-me"},
		},
	}
	removed := removeMCPEntry(m, "mcpServers")
	if !removed {
		t.Error("removed = false, want true")
	}
	servers := m["mcpServers"].(map[string]any)
	if _, ok := servers["domain"]; ok {
		t.Error("legacy 'domain' debería haberse eliminado")
	}
	if _, ok := servers["domain-mcp"]; ok {
		t.Error("'domain-mcp' debería haberse eliminado")
	}
	if _, ok := servers["other"]; !ok {
		t.Error("'other' del usuario fue eliminado!")
	}
}

func TestRemoveMCPEntry_EmptyContainerIsCleanedUp(t *testing.T) {
	m := map[string]any{
		"mcpServers": map[string]any{
			"domain-mcp": map[string]any{},
		},
	}
	removeMCPEntry(m, "mcpServers")
	if _, ok := m["mcpServers"]; ok {
		t.Error("container vacío debería ser eliminado del JSON top-level")
	}
}

func TestRemoveMCPEntry_NoOpOnMissingContainer(t *testing.T) {
	m := map[string]any{"unrelated": "value"}
	removed := removeMCPEntry(m, "mcpServers")
	if removed {
		t.Error("removed = true sin container — debería ser false")
	}
}

// loadOrEmptyJSON / writeJSON roundtrip
func TestLoadAndWrite_RoundTrip(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.json")
	orig := map[string]any{"a": "b", "n": float64(42)}
	if err := writeJSON(path, orig); err != nil {
		t.Fatalf("write: %v", err)
	}
	loaded, err := loadOrEmptyJSON(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded["a"] != "b" {
		t.Errorf("a = %v, want b", loaded["a"])
	}
}

func TestLoadOrEmptyJSON_NonExistent_ReturnsEmpty(t *testing.T) {
	tmp := t.TempDir()
	m, err := loadOrEmptyJSON(filepath.Join(tmp, "missing.json"))
	if err != nil {
		t.Fatalf("err = %v, want nil para archivo inexistente", err)
	}
	if len(m) != 0 {
		t.Errorf("map = %v, want empty", m)
	}
}

// backupIfExists: archivo presente → backup creado; ausente → no-op.
func TestBackupIfExists(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "config.json")
	if err := os.WriteFile(src, []byte(`{"x":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	backup, err := backupIfExists(src, "20260615T000000Z")
	if err != nil {
		t.Fatalf("backupIfExists: %v", err)
	}
	if backup == "" {
		t.Fatal("backup path vacío")
	}
	if _, err := os.Stat(backup); err != nil {
		t.Errorf("backup no creado: %v", err)
	}


	backup2, err := backupIfExists(filepath.Join(tmp, "ghost.json"), "20260615T000000Z")
	if err != nil {
		t.Errorf("inexistente debería ser no-op, got err: %v", err)
	}
	if backup2 != "" {
		t.Errorf("inexistente debería retornar backup='', got %q", backup2)
	}
}
