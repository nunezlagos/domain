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
// habría prevenido el incidente de `git reset --hard` de un subagente.
//
// DOMAINSERV-278: el criterio es la RECUPERABILIDAD, no "es mutante". Acá queda
// solo lo que git no puede revertir; lo recuperable vive en
// domainPermissionAsks. El set anterior denegaba 8 reglas en duro y eso dejaba
// sin camino a casos de riesgo cero (un `git worktree remove` sobre un worktree
// limpio no se podía ejecutar ni aprobar, porque deny gana antes del diálogo).
//
// El `--force` de worktree acá es defensa PARCIAL a propósito: la regla es
// prefix-match, así que no captura el flag en segunda posición
// (`git worktree remove x --force`). La barrera completa de ese caso es el
// regex del hook (domain-pre-edit.sh), que mira el comando entero.
var domainPermissionDenies = []string{
	"Bash(git reset --hard:*)",
	"Bash(git clean:*)",
	"Bash(git stash drop:*)",
	"Bash(git stash clear:*)",
	"Bash(git worktree remove --force:*)",
	"Bash(git worktree remove -f:*)",
}

// domainPermissionAsks son mutaciones del working tree que git PUEDE revertir:
// el humano las aprueba en el diálogo en vez de quedarse sin camino. Van a
// ~/.claude/settings.json → permissions.ask, que se evalúa después de deny y
// antes de allow. NO se pide por `git checkout <rama>` (cambio de rama
// legítimo), solo por el descarte de archivos (`git checkout --` / `.`).
var domainPermissionAsks = []string{
	"Bash(git stash:*)",
	"Bash(git checkout --:*)",
	"Bash(git checkout .:*)",
	"Bash(git restore:*)",
	"Bash(git rm:*)",
	"Bash(git worktree remove:*)",
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

// pruneStaleDenies saca de permissions.deny las reglas que domain reclasificó a
// ask (DOMAINSERV-278). Es obligatorio para que el upgrade tenga efecto: el orden
// es deny → ask → allow, así que una regla que quedó en deny de un install previo
// gana sobre su propia versión en ask y el usuario no ve ningún cambio.
//
// Compara por igualdad exacta, que es justo lo que hace falta para no arrastrar de
// más: saca "Bash(git stash:*)" y deja intacta "Bash(git stash drop:*)", que sigue
// siendo un deny legítimo. Solo toca reglas gestionadas por domain; las del usuario
// no se comparan y sobreviven. Devuelve true si mutó.
func pruneStaleDenies(perms map[string]any, reclassified []string) bool {
	raw, ok := perms["deny"].([]any)
	if !ok {
		return false
	}
	stale := make(map[string]bool, len(reclassified))
	for _, r := range reclassified {
		stale[r] = true
	}
	kept := make([]any, 0, len(raw))
	removed := false
	for _, e := range raw {
		if s, isStr := e.(string); isStr && stale[s] {
			removed = true
			continue
		}
		kept = append(kept, e)
	}
	if !removed {
		return false
	}
	perms["deny"] = kept
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
	for _, rule := range domainPermissionAsks {
		if upsertStringInArray(perms, "ask", rule) {
			mutated = true
		}
	}
	// DOMAINSERV-278: un install previo dejó estas reglas en deny. Sin arrastrarlas
	// fuera, deny gana sobre el ask nuevo y el upgrade no cambia nada observable.
	if pruneStaleDenies(perms, domainPermissionAsks) {
		mutated = true
	}
	if !mutated {
		return nil
	}
	if _, err := backupIfExists(path, timestamp); err != nil {
		return fmt.Errorf("backup settings.json: %w", err)
	}
	return writeJSON(path, m)
}
