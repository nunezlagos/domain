// Package primary_memory — issue-35.3 detect and disable other memory
// providers (engram, mem0, etc) en los configs de opencode y
// claude-code. Resuelve técnicamente la "pelea de protocolos" con
// engram: si el LLM no ve a engram como MCP server disponible, no
// tiene otra opción que usar domain.
//
// Es opt-in (no auto-ejecuta al setup). El user decide explícitamente.
//
// Reversible: --reactivate restaura desde el último backup.
package primary_memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

// KnownMemoryProviders es el catalog hardcoded de providers de
// memoria. El user puede extender vía ~/.config/domain/primary-memory-catalog.json.
//
// Fuente: revisión manual de los 7-10 vendors de memoria más comunes
// (2026-06). Cuando aparezca un nuevo provider conocido, agregarlo
// acá + entry de changelog.
var KnownMemoryProviders = map[string]bool{
	"engram":   true,
	"mem0":     true,
	"memory":   true, // built-in MCP memory server
	"knowledge": true,
	"recall":   true,
	"cognee":   true,
	"graphiti": true,
}

// DetectedProvider un MCP server detectado en el config de un agente.
type DetectedProvider struct {
	Name       string // ej "engram", "mem0"
	ConfigPath string // path del config escaneado
	IsMemory   bool   // true si está en el catalog
	Agent      string // "opencode" | "claude-code"
}

// SortedNames devuelve los nombres de providers ordenados alfabéticamente.
// Útil para output determinístico.
func SortedNames(providers []DetectedProvider) []string {
	names := make([]string, 0, len(providers))
	for _, p := range providers {
		names = append(names, p.Name)
	}
	sort.Strings(names)
	return names
}

// IsMemoryProvider consulta el catalog hardcoded. El override
// via JSON se aplica en Detect (caller), no acá.
func IsMemoryProvider(name string) bool {
	return KnownMemoryProviders[name]
}

// catalogOverride es el formato del JSON override.
//
// Se permite agregar providers (memory_providers) o forzar que
// algunos NO sean tratados como memoria (non_memory_providers).
// Útil cuando un user tiene un MCP server cuyo nombre colisiona
// con el catalog pero no es realmente memoria, o al revés.
type catalogOverride struct {
	MemoryProviders    []string `json:"memory_providers"`
	NonMemoryProviders []string `json:"non_memory_providers"`
}

// CatalogPath devuelve la ruta del override. Útil para tests.
func CatalogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "domain", "primary-memory-catalog.json"), nil
}

// LoadCatalog devuelve el catalog efectivo: hardcoded + override
// desde `~/.config/domain/primary-memory-catalog.json`.
//
// Si el archivo no existe → retorna copia del hardcoded.
// Si el JSON es inválido → log silencioso + hardcoded (no rompe el
// install).
//
// Semántica del override:
//   - memory_providers: se MERGEAN con el hardcoded (set union).
//   - non_memory_providers: REMUEVEN del catalog (gana siempre).
func LoadCatalog() (map[string]bool, error) {
	return loadCatalogFromPath("")
}

// loadCatalogFromPath es la implementación; path="" usa la default.
func loadCatalogFromPath(path string) (map[string]bool, error) {
	catalog := make(map[string]bool, len(KnownMemoryProviders))
	for k, v := range KnownMemoryProviders {
		catalog[k] = v
	}

	if path == "" {
		p, err := CatalogPath()
		if err != nil {
			return catalog, nil
		}
		path = p
	}

	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return catalog, nil
		}
		return catalog, nil
	}
	var ov catalogOverride
	if err := json.Unmarshal(body, &ov); err != nil {
		return catalog, nil
	}
	for _, name := range ov.MemoryProviders {
		catalog[name] = true
	}
	for _, name := range ov.NonMemoryProviders {
		delete(catalog, name)
	}
	return catalog, nil
}
