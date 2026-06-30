package main

import (
	"fmt"
	"path/filepath"
)

// localInstructionGlobs son los archivos de instrucciones LOCALES de proyecto que
// domain neutraliza por DEFAULT (modo agresivo, REQ-54 / issue-54.1) para que en
// cualquier proyecto apliquen SOLO las reglas globales de domain.
//
// VERIFICADO (docs Claude Code + fnmatch): claudeMdExcludes matchea contra paths
// ABSOLUTOS. Por eso NO se incluyen globs de CLAUDE.md (**/CLAUDE.md,
// **/.claude/CLAUDE.md, **/CLAUDE.local.md): matchearían también el bloque global
// propio de domain en ~/.claude/CLAUDE.md y se AUTO-NEUTRALIZARÍA. Excluir el
// CLAUDE.md de proyecto sin matar el global requiere project-scope settings o mover
// el bloque domain fuera de ~/.claude/CLAUDE.md — decisión abierta (REQ-54 design).
// Estos globs cubren los archivos de instrucciones de OTRAS herramientas + .claude/rules,
// que NO colisionan con el global de domain.
var localInstructionGlobs = []string{
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
