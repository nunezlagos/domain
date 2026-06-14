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

import "sort"

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
