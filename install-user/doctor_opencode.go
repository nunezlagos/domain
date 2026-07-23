package main

import (
	"fmt"
	"path/filepath"
)

// checkOpencodePermission verifica que opencode.json tenga el bloque
// `permission` con las reglas deny de git destructivo y el catch-all
// bash["*"]="ask" (DOMAINSERV-69). Paths es OS-aware (DOMAINSERV-102).
func checkOpencodePermission(paths Paths) int {
	step("Permisos OpenCode (opencode.json)")
	if !fileExists(paths.OpencodeMCP) {
		info("opencode no detectado — chequeo omitido")
		return 0
	}
	m, err := loadOrEmptyJSON(paths.OpencodeMCP)
	if err != nil {
		failL(paths.OpencodeMCP + " ilegible: " + err.Error())
		return 1
	}
	perm, _ := m["permission"].(map[string]any)
	if perm == nil {
		failL(paths.OpencodeMCP + ": falta bloque 'permission'")
		return 1
	}
	bashRules, _ := perm["bash"].(map[string]any)
	if bashRules == nil {
		failL(paths.OpencodeMCP + ": permission falta 'bash'")
		return 1
	}
	var missing []string
	for _, rule := range opencodeGitDenyRules {
		if v, ok := bashRules[rule]; !ok || fmt.Sprint(v) != "deny" {
			missing = append(missing, rule)
		}
	}
	if len(missing) > 0 {
		failL(fmt.Sprintf("faltan reglas deny en permission.bash: %v", missing))
		return 1
	}
	// catch-all: sin bash["*"]="ask" el last-match-wins no gatea el resto.
	if v, ok := bashRules["*"]; !ok || fmt.Sprint(v) != "ask" {
		failL(paths.OpencodeMCP + `: permission.bash falta el catch-all "*":"ask"`)
		return 1
	}
	ok(`reglas git deny + catch-all "*":"ask" presentes en permission.bash`)
	return 0
}

// checkOpencodeInstruction verifica que el instruction global de OpenCode esté
// instalado: el archivo instructions/domain.md existe y opencode.json lo
// referencia en el array "instructions" (DOMAINSERV-102, valida el fix de
// DOMAINSERV-101). Si OpenCode no está presente, omite.
func checkOpencodeInstruction(paths Paths) int {
	step("Instrucción global OpenCode (instructions/domain.md)")
	if !fileExists(paths.OpencodeMCP) {
		info("opencode no detectado — chequeo omitido")
		return 0
	}
	instrPath := filepath.Join(paths.OpencodeDir, "instructions", "domain.md")
	if !fileExists(instrPath) {
		failL(instrPath + ": falta el archivo de instrucción global de OpenCode")
		return 1
	}
	m, err := loadOrEmptyJSON(paths.OpencodeMCP)
	if err != nil {
		failL(paths.OpencodeMCP + " ilegible: " + err.Error())
		return 1
	}
	if !doctorStringSet(m["instructions"])["instructions/domain.md"] {
		failL(paths.OpencodeMCP + `: array "instructions" no referencia "instructions/domain.md"`)
		return 1
	}
	ok("instruction global de OpenCode presente y referenciada")
	return 0
}

// checkOpencodePlugin verifica que el plugin git-guard esté instalado en
// <OpencodeDir>/plugins/domain-git-guard.js (DOMAINSERV-69b). Si OpenCode
// no está presente, omite. Presente sin plugin → falla crítica.
func checkOpencodePlugin(paths Paths) int {
	step("Plugin OpenCode (git-guard)")
	if !dirExists(paths.OpencodeDir) {
		info("opencode no detectado — chequeo omitido")
		return 0
	}
	path := filepath.Join(paths.OpencodeDir, "plugins", "domain-git-guard.js")
	if !fileExists(path) {
		failL(path + ": falta el plugin git-guard")
		return 1
	}
	ok("plugin git-guard presente")
	return 0
}
