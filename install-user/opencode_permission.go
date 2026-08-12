package main

import (
	"fmt"
)

// opencodeGitDenyRules son las reglas de bash que OpenCode debe DENEGAR,
// espejando domainPermissionDenies de Claude Code. OpenCode evalúa reglas
// en orden, LAST MATCH WINS. Las deny van primero, luego "*": "ask".
//
// DOMAINSERV-278: solo lo irrecuperable. Lo que git puede revertir vive en
// opencodeGitAskRules.
var opencodeGitDenyRules = []string{
	"git reset --hard *",
	"git clean *",
	"git stash drop *",
	"git stash clear *",
	"git worktree remove --force *",
	"git worktree remove -f *",
}

// opencodeGitAskRules espeja domainPermissionAsks: mutaciones recuperables que
// el humano aprueba en el diálogo.
//
// En OpenCode esto NO alcanza por sí solo. El plugin domain-git-guard.js hace
// `throw` y el throw no tiene forma de "preguntar", así que un comando que siga
// en la lista DESTRUCTIVE del plugin queda bloqueado por más que acá diga "ask".
// El par obligatorio de esta lista es la lista DESTRUCTIVE de
// templates/opencode-git-guard.js, que solo conserva lo irrecuperable.
var opencodeGitAskRules = []string{
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
		// idempotencia (DOMAINSERV-84): forzar "deny" aunque la regla exista con otro
		// valor (ej. "ask" de un install previo). Son reglas gestionadas por domain.
		if v, _ := bashRules[rule].(string); v != "deny" {
			bashRules[rule] = "deny"
			mutated = true
		}
	}
	// DOMAINSERV-278: mismo forzado en la otra dirección. Un install previo dejó
	// estas reglas en "deny"; sin sobrescribirlas el upgrade no cambia nada. Acá el
	// pisado alcanza porque OpenCode indexa por regla (un map), no una lista ordenada
	// como el deny/ask de Claude Code — no hay regla stale que arrastrar aparte.
	for _, rule := range opencodeGitAskRules {
		if v, _ := bashRules[rule].(string); v != "ask" {
			bashRules[rule] = "ask"
			mutated = true
		}
	}
	// idempotencia (DOMAINSERV-84): el catch-all "*" lo exige el doctor como "ask"
	// (posición de seguridad: lo no listado pregunta, no auto-corre). Forzarlo aunque
	// exista con otro valor (ej. "allow" de un install/default previo).
	if v, _ := bashRules["*"].(string); v != "ask" {
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
