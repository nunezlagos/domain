package main

import (
	"fmt"
	"path/filepath"
)

// localInstructionGlobs son los archivos de instrucciones LOCALES de proyecto que
// domain neutraliza por DEFAULT (modo agresivo, REQ-54 / issue-54.1) para que en
// cualquier proyecto apliquen SOLO las reglas globales de domain.
//
// OJO (verificar contra Claude Code vivo antes de shippear): el bloque global de
// domain vive en ~/.claude/CLAUDE.md (user scope, lo carga Claude Code aparte). Hay
// que confirmar que el glob **/CLAUDE.md NO excluya ese global propio; si el matcher
// de claudeMdExcludes lo alcanzara, restringir el patrón a CLAUDE.md de proyecto.
var localInstructionGlobs = []string{
	"**/CLAUDE.md",
	"**/CLAUDE.local.md",
	"**/.claude/CLAUDE.md",
	"**/AGENTS.md",
	"**/.cursorrules",
	"**/.windsurfrules",
	"**/.github/copilot-instructions.md",
	"**/.claude/rules/**",
}

// claudeSettingsPath es el settings.json de Claude Code (~/.claude/settings.json).
func claudeSettingsPath(home string) string {
	return filepath.Join(home, ".claude", "settings.json")
}

// installClaudeMdExcludes agrega (por DEFAULT) los globs de instrucciones locales a
// claudeMdExcludes en ~/.claude/settings.json, preservando las entradas del usuario,
// con backup previo. Idempotente: re-ejecutar no duplica ni acumula backups.
//
// keepLocal=true (flag --keep-local-rules) lo hace no-op: el usuario conserva las
// instrucciones locales de proyecto.
func installClaudeMdExcludes(home, timestamp string, keepLocal bool) error {
	if keepLocal {
		return nil
	}
	path := claudeSettingsPath(home)
	m, err := loadOrEmptyJSON(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	mutated := false
	for _, g := range localInstructionGlobs {
		if upsertStringInArray(m, "claudeMdExcludes", g) {
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
