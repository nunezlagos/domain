package main

import (
	"fmt"
)

// domainPermissionAllows son las reglas de permisos que domain allowlistea en
// ~/.claude/settings.json → permissions.allow, para que el protocolo NO dependa
// del clasificador LLM del permission mode "auto" (DOMAINSERV-35): si ese
// clasificador cae, sin estas reglas orchestrate/tickets/mem_save quedan
// inusables. Edit/Write siguen gateados por el hook SDD (domain-pre-edit.sh);
// esto solo saca al clasificador del camino, no el enforcement.
var domainPermissionAllows = []string{
	"mcp__domain-mcp",
	"Read(**)",
	"Edit(**)",
	"Write(**)",
}

// installClaudePermissions agrega las reglas de domainPermissionAllows a
// permissions.allow en ~/.claude/settings.json, preservando las entradas del
// usuario y sin tocar defaultMode. Idempotente: re-ejecutar no duplica ni
// acumula backups. Crea permissions/allow si no existen.
func installClaudePermissions(home, timestamp string) error {
	path := claudeSettingsPath(home)
	m, err := loadOrEmptyJSON(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	perms, ok := m["permissions"].(map[string]any)
	if !ok {
		perms = map[string]any{}
		m["permissions"] = perms
	}
	mutated := false
	for _, rule := range domainPermissionAllows {
		if upsertStringInArray(perms, "allow", rule) {
			mutated = true
		}
	}
	if !mutated {
		return nil
	}
	if _, err := backupIfExists(path, timestamp); err != nil {
		return fmt.Errorf("backup settings.json: %w", err)
	}
	return writeJSON(path, m)
}
