package main

import (
	"os"
	"path/filepath"
	"testing"
)

// DOMAINSERV-278. La reclasificación por recuperabilidad tiene una parte que no se ve en el
// diff de las listas y es la que decide si el cambio sirve de algo: sacar de permissions.deny
// las reglas que pasaron a ask.
//
// El orden de evaluación es deny → ask → allow. Una regla que quedó en deny de un install
// previo GANA sobre su propia versión en ask, así que sin el prune el usuario re-corre el
// install, ve "OK" y sigue sin poder aprobar el comando. Es el mismo falso verde que ya
// mordió en DOMAINSERV-239 con los hooks: el instalador reporta éxito y nada cambió.

// settingsDeInstallPrevio escribe el settings.json tal como lo dejaba la versión anterior:
// las 8 reglas de git en deny, sin bloque ask.
func settingsDeInstallPrevio(t *testing.T, home string) {
	t.Helper()
	path := claudeSettingsPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(path, map[string]any{
		"permissions": map[string]any{
			"deny": []any{
				"Bash(git reset --hard:*)",
				"Bash(git clean:*)",
				"Bash(git stash:*)",
				"Bash(git checkout --:*)",
				"Bash(git checkout .:*)",
				"Bash(git restore:*)",
				"Bash(git rm:*)",
				"Bash(git worktree remove:*)",
				"Bash(rm -rf /:*)",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestInstallClaudePermissions_UpgradeSacaDeDenyLoQuePasoAAsk(t *testing.T) {
	home := t.TempDir()
	settingsDeInstallPrevio(t, home)

	if err := installClaudePermissions(perfilDefault(home), "ts"); err != nil {
		t.Fatalf("install: %v", err)
	}

	perms, _ := readSettings(t, home)["permissions"].(map[string]any)
	deny := toStringSet(perms["deny"])
	ask := toStringSet(perms["ask"])

	for _, rule := range domainPermissionAsks {
		if !ask[rule] {
			t.Errorf("la regla %q no quedó en permissions.ask", rule)
		}
		if deny[rule] {
			t.Errorf("la regla %q sigue en permissions.deny además de estar en ask. deny se evalúa "+
				"PRIMERO, así que el ask es letra muerta y el upgrade no cambió nada observable", rule)
		}
	}
}

// El prune tiene que ser quirúrgico: "Bash(git stash:*)" sale, "Bash(git stash drop:*)" se
// queda. Si se arrastrara por prefijo se abriría en silencio el caso irrecuperable, que es
// exactamente lo que el guard vino a cerrar.
func TestInstallClaudePermissions_UpgradeConservaLosDenyIrrecuperables(t *testing.T) {
	home := t.TempDir()
	settingsDeInstallPrevio(t, home)

	if err := installClaudePermissions(perfilDefault(home), "ts"); err != nil {
		t.Fatalf("install: %v", err)
	}

	deny := toStringSet(readSettings(t, home)["permissions"].(map[string]any)["deny"])
	for _, rule := range domainPermissionDenies {
		if !deny[rule] {
			t.Errorf("falta la regla deny irrecuperable %q: el prune se llevó de más", rule)
		}
	}
}

// Las reglas del usuario no son de domain y no se tocan, ni siquiera las que se parecen a
// las gestionadas. Un prune que barre por su cuenta es una pérdida de configuración ajena.
func TestInstallClaudePermissions_UpgradePreservaDenyDelUsuario(t *testing.T) {
	home := t.TempDir()
	settingsDeInstallPrevio(t, home)

	if err := installClaudePermissions(perfilDefault(home), "ts"); err != nil {
		t.Fatalf("install: %v", err)
	}

	deny := toStringSet(readSettings(t, home)["permissions"].(map[string]any)["deny"])
	if !deny["Bash(rm -rf /:*)"] {
		t.Error("se perdió una regla deny propia del usuario que domain no gestiona")
	}
}

// Idempotencia: re-correr el install sobre un settings ya migrado no vuelve a mutar. Sin
// esto el instalador acumularía backups en cada corrida (DOMAINSERV-84).
func TestInstallClaudePermissions_ReclasificacionEsIdempotente(t *testing.T) {
	home := t.TempDir()
	settingsDeInstallPrevio(t, home)

	if err := installClaudePermissions(perfilDefault(home), "ts1"); err != nil {
		t.Fatalf("install 1: %v", err)
	}
	primera := readSettings(t, home)

	if err := installClaudePermissions(perfilDefault(home), "ts2"); err != nil {
		t.Fatalf("install 2: %v", err)
	}
	segunda := readSettings(t, home)

	primeraPerms, _ := primera["permissions"].(map[string]any)
	segundaPerms, _ := segunda["permissions"].(map[string]any)
	for _, clave := range []string{"deny", "ask"} {
		antes := toStringSet(primeraPerms[clave])
		despues := toStringSet(segundaPerms[clave])
		if len(antes) != len(despues) {
			t.Errorf("permissions.%s cambió de tamaño en la segunda corrida: %d → %d",
				clave, len(antes), len(despues))
		}
		for rule := range antes {
			if !despues[rule] {
				t.Errorf("la segunda corrida perdió %q de permissions.%s", rule, clave)
			}
		}
	}
}

// Una regla no puede estar en las dos listas: sería una contradicción que deny resuelve
// callado, dejando al usuario con un bloqueo que la lista de asks dice que no existe.
func TestPermisosDomain_NingunaReglaEstaEnDenyYEnAsk(t *testing.T) {
	enDeny := map[string]bool{}
	for _, r := range domainPermissionDenies {
		enDeny[r] = true
	}
	for _, r := range domainPermissionAsks {
		if enDeny[r] {
			t.Errorf("la regla %q está en domainPermissionDenies Y en domainPermissionAsks. "+
				"deny gana, así que el ask nunca se vería", r)
		}
	}
}

// Paridad de las dos mitades del cambio: si una lista crece y la otra no, un cliente queda
// con la política vieja sin que nada lo note (DOMAINSERV-69).
func TestPermisosDomain_ParidadClaudeOpencode(t *testing.T) {
	if len(domainPermissionDenies) != len(opencodeGitDenyRules) {
		t.Errorf("deny desalineado: Claude tiene %d reglas y OpenCode %d",
			len(domainPermissionDenies), len(opencodeGitDenyRules))
	}
	if len(domainPermissionAsks) != len(opencodeGitAskRules) {
		t.Errorf("ask desalineado: Claude tiene %d reglas y OpenCode %d",
			len(domainPermissionAsks), len(opencodeGitAskRules))
	}
}
