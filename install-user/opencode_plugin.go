package main

import (
	"os"
	"path/filepath"
)

// installOpencodePlugin escribe el plugin git-guard de OpenCode en
// ~/.config/opencode/plugins/domain-git-guard.js. El plugin deniega git
// destructivo a nivel tool.execute.before, normalizando el argv para cerrar la
// evasión `git -C . reset --hard` que las reglas declarativas de
// permission.bash (DOMAINSERV-69a) no pueden detectar (DOMAINSERV-69b).
//
// No-op si OpenCode no está presente (no existe el dir de config).
func installOpencodePlugin(paths Paths, timestamp string) error {
	if !dirExists(paths.OpencodeDir) {
		return nil
	}
	pluginDir := filepath.Join(paths.OpencodeDir, "plugins")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return err
	}
	dst := filepath.Join(pluginDir, "domain-git-guard.js")
	if _, err := backupIfExists(dst, timestamp); err != nil {
		return err
	}
	if err := os.WriteFile(dst, opencodeGitGuardJS, 0o644); err != nil {
		return err
	}
	// DOMAINSERV-100: gate SDD + commit-gate (paridad con domain-pre-edit.sh).
	gate := filepath.Join(pluginDir, "domain-sdd-gate.js")
	if _, err := backupIfExists(gate, timestamp); err != nil {
		return err
	}
	return os.WriteFile(gate, opencodeSddGateJS, 0o644)
}
