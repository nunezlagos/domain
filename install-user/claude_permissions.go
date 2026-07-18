package main

import (
	"fmt"
	"strings"
)

// domainPermissionAllows son las reglas de permisos que domain allowlistea en
// ~/.claude/settings.json → permissions.allow, para que el protocolo NO dependa
// del clasificador LLM del permission mode "auto" (DOMAINSERV-35): si ese
// clasificador cae, sin estas reglas orchestrate/tickets/mem_save quedan
// inusables. Edit/Write siguen gateados por el hook SDD (domain-pre-edit.sh);
// esto solo saca al clasificador del camino, no el enforcement.
// Edit(**) cubre Write/NotebookEdit para el chequeo de permisos de archivos
// de Claude Code — una regla Write(path) ahí es muerta (ver migrateStaleWriteRules).
var domainPermissionAllows = []string{
	"mcp__domain-mcp",
	"Read(**)",
	"Edit(**)",
}

// domainPermissionDenies son bloqueos DUROS y deterministas en
// ~/.claude/settings.json → permissions.deny. A diferencia del hook SDD (que
// puede fallar o exit-0 con flow activo), un deny se evalúa PRIMERO (orden
// deny → ask → allow) y se hereda por los subagentes — es la única barrera que
// habría prevenido el incidente de `git reset --hard`/`git stash` de un
// subagente. Se elige el set destructivo-al-worktree sin sobre-bloquear el
// flujo normal: NO se deniega `git checkout <rama>` (cambio de rama legítimo),
// solo el descarte de archivos (`git checkout --` / `git checkout .`). Si el
// usuario los necesita, los corre con el prefijo `!` en el prompt.
var domainPermissionDenies = []string{
	"Bash(git reset --hard:*)",
	"Bash(git clean:*)",
	"Bash(git stash:*)",
	"Bash(git checkout --:*)",
	"Bash(git checkout .:*)",
}

// migrateStaleWriteRules convierte reglas Write(<path>) muertas de
// permissions.allow a su equivalente Edit(<path>): el chequeo de permisos de
// archivos de Claude Code solo honra reglas Edit(path), así que una Write(path)
// quedó sin efecto y además dispara un warning al arrancar. Es sintáctico y
// lossless: elimina cada Write(X) y garantiza Edit(X). Devuelve true si mutó.
func migrateStaleWriteRules(perms map[string]any) bool {
	raw, ok := perms["allow"].([]any)
	if !ok {
		return false
	}
	kept := make([]any, 0, len(raw))
	var migrated []string
	for _, e := range raw {
		s, isStr := e.(string)
		if isStr && strings.HasPrefix(s, "Write(") && strings.HasSuffix(s, ")") {
			inner := s[len("Write(") : len(s)-1]
			migrated = append(migrated, "Edit("+inner+")")
			continue
		}
		kept = append(kept, e)
	}
	if len(migrated) == 0 {
		return false
	}
	perms["allow"] = kept
	for _, rule := range migrated {
		upsertStringInArray(perms, "allow", rule)
	}
	return true
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
	if migrateStaleWriteRules(perms) {
		mutated = true
	}
	for _, rule := range domainPermissionAllows {
		if upsertStringInArray(perms, "allow", rule) {
			mutated = true
		}
	}
	for _, rule := range domainPermissionDenies {
		if upsertStringInArray(perms, "deny", rule) {
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
