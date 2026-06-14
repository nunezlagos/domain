package primary_memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Disable marca los providers indicados como "disabled" en el config
// del agente. Estrategia: cambiar `command` a `false` (opencode y
// claude-code aceptan ambos). Backup del archivo pre-cambio.
//
// agent: "opencode" | "claude-code"
// configPath: ruta absoluta al archivo de config.
// providers: lista de nombres de MCP servers a deshabilitar.
//
// Si un provider no existe en el config → no-op para ese provider
// (no es error, el caller puede haber detectado providers que
// cambiaron entre el scan y el disable).
func Disable(agent, configPath string, providers []string) error {
	body, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return fmt.Errorf("config inválido: %w", err)
	}

	serversKey := "mcp"
	if agent == "claude-code" {
		serversKey = "mcpServers"
	}
	servers, _ := doc[serversKey].(map[string]any)
	if servers == nil {
		return nil // nada que hacer
	}

	// Backup ANTES de cualquier cambio.
	backupPath := fmt.Sprintf("%s.bak.%s", configPath,
		time.Now().UTC().Format("20060102T150405Z"))
	if err := os.WriteFile(backupPath, body, 0o600); err != nil {
		return fmt.Errorf("backup: %w", err)
	}

	// Aplicar disable a cada provider.
	providerSet := make(map[string]bool, len(providers))
	for _, p := range providers {
		providerSet[p] = true
	}
	for name, entryAny := range servers {
		if !providerSet[name] {
			continue
		}
		entry, ok := entryAny.(map[string]any)
		if !ok {
			continue
		}
		entry["command"] = false
		servers[name] = entry
	}

	// Re-serializar.
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	out = append(out, '\n')
	if err := os.WriteFile(configPath, out, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// Reactivate restaura el archivo de config desde el backup más
// reciente (ordenado por timestamp).
//
// Busca el patrón: <configPath>.bak.YYYYMMDDTHHMMSSZ y toma el
// lexicográficamente mayor (timestamps en formato ISO básico son
// ordenables). Después de restaurar, ELIMINA el backup usado — así
// el user puede llamar reactivate N veces para deshacer N cambios
// progresivamente.
//
// Si el backup no existe → error (no es fatal: el user puede
// estar corriendo reactivate cuando no hay nada que deshacer).
func Reactivate(agent, configPath string) error {
	dir := filepath.Dir(configPath)
	base := filepath.Base(configPath)
	pattern := filepath.Join(dir, base+".bak.*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("glob: %w", err)
	}
	if len(matches) == 0 {
		return fmt.Errorf("no backup found for %s (pattern: %s)", configPath, pattern)
	}
	// Ordenar lexicográficamente — el formato YYYYMMDDTHHMMSSZ es
	// comparable directamente.
	sort.Strings(matches)
	latest := matches[len(matches)-1]
	body, err := os.ReadFile(latest)
	if err != nil {
		return fmt.Errorf("read backup: %w", err)
	}
	if err := os.WriteFile(configPath, body, 0o600); err != nil {
		return fmt.Errorf("restore: %w", err)
	}
	// Eliminar el backup usado para que la próxima reactivate tome
	// el anterior. Esto da semántica "1 reactivate = 1 undo".
	if err := os.Remove(latest); err != nil {
		return fmt.Errorf("remove used backup: %w", err)
	}
	return nil
}
