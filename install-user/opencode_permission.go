package main

import (
	"fmt"
)

// opencodeGitDenyRules son las reglas de bash que OpenCode debe DENEGAR,
// espejando domainPermissionDenies de Claude Code. OpenCode evalúa reglas
// en orden, LAST MATCH WINS. Las deny van primero, luego "*": "ask".
var opencodeGitDenyRules = []string{
	"git reset --hard *",
	"git clean *",
	"git stash *",
	"git checkout -- *",
	"git checkout . *",
	"git restore *",
	"git rm *",
	"git worktree remove *",
}

// installOpencodePermission agrega un bloque declarativo `permission` a
// opencode.json con reglas deny para git destructivo, espejando el
// permissions.deny de Claude Code (DOMAINSERV-69). Preserva cualquier regla
// de permission que el usuario ya tenga.
//
// OpenCode no tiene hooks nativos como Claude Code (PreToolUse/PostToolUse),
// por lo que el gate SDD completo (validación de flow activo) requiere un
// plugin JS con tool.execute.before — fuera de alcance de este bloque
// declarativo.
func installOpencodePermission(paths Paths, timestamp string) error {
	if !dirExists(paths.OpencodeDir) || !fileExists(paths.OpencodeMCP) {
		return nil
	}

	m, err := loadOrEmptyJSON(paths.OpencodeMCP)
	if err != nil {
		return err
	}

	perm, _ := m["permission"].(map[string]any)
	if perm == nil {
		perm = map[string]any{}
	}

	bashRules, _ := perm["bash"].(map[string]any)
	if bashRules == nil {
		bashRules = map[string]any{}
	}

	mutated := false
	for _, rule := range opencodeGitDenyRules {
		if bashRules[rule] == nil {
			bashRules[rule] = "deny"
			mutated = true
		}
	}
	if bashRules["*"] == nil {
		bashRules["*"] = "ask"
		mutated = true
	}
	if !mutated {
		return nil
	}

	perm["bash"] = bashRules
	m["permission"] = perm

	if _, err := backupIfExists(paths.OpencodeMCP, timestamp); err != nil {
		return fmt.Errorf("backup opencode.json: %w", err)
	}
	return writeJSON(paths.OpencodeMCP, m)
}
