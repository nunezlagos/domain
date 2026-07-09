package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

// keepLastBackups: cada archivo conserva como máximo los 3 backups más recientes.
const keepLastBackups = 3

// backupIfExists copia path → path.backup-YYYYMMDDTHHMMSSZ si existe. Deduplica
// (no crea backup si el contenido coincide con el último) y poda a los
// keepLastBackups más recientes. No importa el paquete install del server:
// install-user es un módulo Go propio.
func backupIfExists(path, timestamp string) (string, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", nil
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	backups := listBackups(path)
	if len(backups) > 0 {
		if lastHash, err := fileSHA256(backups[len(backups)-1]); err == nil {
			if lastHash == sha256Hex(src) {
				return backups[len(backups)-1], nil
			}
		}
	}

	backup := path + ".backup-" + timestamp
	if err := os.WriteFile(backup, src, 0o600); err != nil {
		return "", err
	}
	pruneBackups(path, keepLastBackups)
	return backup, nil
}

// listBackups devuelve los .backup-* de path ordenados del más viejo al más
// nuevo (el timestamp compacto ordena lexicográficamente = cronológico).
func listBackups(path string) []string {
	matches, err := filepath.Glob(path + ".backup-*")
	if err != nil {
		return nil
	}
	sort.Strings(matches)
	return matches
}

// pruneBackups deja solo los keepLast backups más recientes de path.
func pruneBackups(path string, keepLast int) {
	backups := listBackups(path)
	if len(backups) <= keepLast {
		return
	}
	for _, p := range backups[:len(backups)-keepLast] {
		_ = os.Remove(p)
	}
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:])
}

func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return sha256Hex(data), nil
}

// Timestamp ISO compacto (sin separadores) para nombres de backup.
func Timestamp() string {
	return time.Now().UTC().Format("20060102T150405Z")
}

// Install entry: dado un map JSON ya cargado, sirve para:
//  1. borrar entry legacy "domain" (solo si NO es una instalación local viva)
//  2. setear entry "domain-mcp" con la nueva URL+headers
//
// container es la key superior (ej: "mcpServers" o "mcp"). Si no existe,
// se crea. Otras entries del container se preservan.
//
// Devuelve skipped=true cuando ya existe una entry LOCAL funcional del server
// (key "domain" con transporte local/command): en ese caso NO tocamos nada
// para no romper esa instalación ni crear un duplicado remoto contradictorio.
// El instalador del SERVER usa transporte local; el del USER, remoto. Si el
// usuario corrió el del server, esa es la fuente de verdad y la respetamos.
func upsertMCPEntry(m map[string]any, container string, entry map[string]any) (skipped bool) {
	c, ok := m[container].(map[string]any)
	if !ok {
		c = map[string]any{}
	}
	// Dedup local↔remoto: si hay un 'domain' local vivo, no lo pisamos ni
	// agregamos un 'domain-mcp' remoto contradictorio.
	if isLocalEntry(c[entryNameLegacy]) {
		return true
	}
	// Si ya hay un 'domain-mcp' local (raro, pero posible), idem.
	if isLocalEntry(c[entryNameNew]) {
		return true
	}
	delete(c, entryNameLegacy) // migración de entry legacy remota
	c[entryNameNew] = entry
	m[container] = c
	return false
}

// isLocalEntry reporta si una entry MCP corresponde a un transporte LOCAL
// (command/stdio), señal de que el server domain está instalado localmente.
// Cubre el formato de Claude Code/Cursor ("command": "...") y el de OpenCode
// ("type": "local" o "command": [...]). Una entry remota (solo "url") → false.
func isLocalEntry(v any) bool {
	e, ok := v.(map[string]any)
	if !ok {
		return false
	}
	if t, ok := e["type"].(string); ok && t == "local" {
		return true
	}
	if _, ok := e["command"]; ok {
		// command presente (string o array) → transporte local.
		// Excluimos command:false (deshabilitado) que no es "vivo".
		if b, isBool := e["command"].(bool); isBool && !b {
			return false
		}
		return true
	}
	return false
}

// removeMCPEntry borra la entry remota "domain-mcp" del container. También
// borra la entry legacy "domain" PERO solo si es remota (creada por una
// versión vieja de este instalador). Una entry "domain" LOCAL es del
// instalador del SERVER y NO la tocamos: el uninstall del user no debe
// romper una instalación local ajena.
// Si el container queda vacío, lo elimina (más limpio).
func removeMCPEntry(m map[string]any, container string) bool {
	c, ok := m[container].(map[string]any)
	if !ok {
		return false
	}
	_, hadNew := c[entryNameNew]
	delete(c, entryNameNew)

	hadLegacy := false
	if legacy, exists := c[entryNameLegacy]; exists && !isLocalEntry(legacy) {
		delete(c, entryNameLegacy)
		hadLegacy = true
	}

	if len(c) == 0 {
		delete(m, container)
	} else {
		m[container] = c
	}
	return hadNew || hadLegacy
}
