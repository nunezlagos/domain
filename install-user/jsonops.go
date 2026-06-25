package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)









const entryNameNew = "domain-mcp" // nombre actual del MCP entry
const entryNameLegacy = "domain"  // entries de instalaciones previas

// LoadOrEmptyJSON lee el archivo como map. Si no existe, devuelve {}.
// Si existe pero no parsea, error.
func loadOrEmptyJSON(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("parse JSON %s: %w", path, err)
	}
	return m, nil
}

// writeJSON escribe formato bonito con indent 2 (igual que jq default).
func writeJSON(path string, m map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

// backupIfExists copia path → path.backup-YYYYMMDDTHHMMSSZ si existe.
func backupIfExists(path, timestamp string) (string, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", nil
	}
	backup := path + ".backup-" + timestamp
	src, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(backup, src, 0o600); err != nil {
		return "", err
	}
	return backup, nil
}

// Timestamp ISO compacto (sin separadores) para nombres de backup.
func Timestamp() string {
	return time.Now().UTC().Format("20060102T150405Z")
}

// Install entry: dado un map JSON ya cargado, sirve para:
//   1. borrar entry legacy "domain"
//   2. setear entry "domain-mcp" con la nueva URL+headers
//
// container es la key superior (ej: "mcpServers" o "mcp"). Si no existe,
// se crea. Otras entries del container se preservan.
func upsertMCPEntry(m map[string]any, container string, entry map[string]any) {
	c, ok := m[container].(map[string]any)
	if !ok {
		c = map[string]any{}
	}
	delete(c, entryNameLegacy) // migración
	c[entryNameNew] = entry
	m[container] = c
}

// removeMCPEntry borra entry "domain-mcp" Y "domain" del container.
// Si el container queda vacío, lo elimina (más limpio).
func removeMCPEntry(m map[string]any, container string) bool {
	c, ok := m[container].(map[string]any)
	if !ok {
		return false
	}
	_, hadNew := c[entryNameNew]
	_, hadLegacy := c[entryNameLegacy]
	delete(c, entryNameNew)
	delete(c, entryNameLegacy)
	if len(c) == 0 {
		delete(m, container)
	} else {
		m[container] = c
	}
	return hadNew || hadLegacy
}
