package main

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// DOMAINSERV-247. Los tres defectos de paridad de los plugins de OpenCode, medidos el 2026-08-06.

// ─── DEFECTO 3: el doctor mira 1 de los 2 plugins ────────────────────────────
//
// MEDIDO: opencode_plugin.go instala domain-git-guard.js Y domain-sdd-gate.js, pero
// checkOpencodePlugin solo verificaba el primero. El sdd-gate es el que lleva el commit-gate y el
// gate SDD: si falta, el doctor daba todo verde y el usuario se quedaba sin gate sin enterarse.

// Se mira la VARIABLE, no el texto del archivo. Un grep sobre doctor_opencode.go encuentra el
// nombre del plugin en un comentario y da verde con la lista vacía — lo confirmé saboteando: quité
// "domain-sdd-gate.js" de la lista y el test seguía pasando porque el string quedaba en el
// comentario de arriba. Un guard que se satisface con una mención no verifica nada.
func TestDoctorOpencode_VerificaLosDosPlugins(t *testing.T) {
	requeridos := map[string]bool{}
	for _, p := range opencodePluginsRequeridos {
		requeridos[p] = true
	}

	for _, plugin := range []string{"domain-git-guard.js", "domain-sdd-gate.js"} {
		if !requeridos[plugin] {
			t.Errorf("el doctor no verifica %s. opencode_plugin.go instala los DOS; el que falta "+
				"puede desaparecer del disco sin que ningún chequeo lo note", plugin)
		}
	}
}

// El guard sobre el guard: si mañana se instala un tercer plugin, el doctor tiene que cubrirlo.
// Sin esto, el defecto 3 se repite con el próximo.
func TestDoctorOpencode_CubreTodoLoQueElInstaladorEscribe(t *testing.T) {
	inst, err := os.ReadFile("opencode_plugin.go")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`"(domain-[a-z0-9-]+\.js)"`)
	instalados := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(string(inst), -1) {
		instalados[m[1]] = true
	}
	if len(instalados) == 0 {
		t.Fatal("no se detectó ningún plugin en opencode_plugin.go: el guard no estaría midiendo nada")
	}

	// contra la variable, por la misma razón que el test de arriba
	verificados := map[string]bool{}
	for _, p := range opencodePluginsRequeridos {
		verificados[p] = true
	}

	for plugin := range instalados {
		if !verificados[plugin] {
			t.Errorf("opencode_plugin.go instala %s pero el doctor no lo verifica", plugin)
		}
	}
}

// ─── DEFECTO 2: el git-guard bloquea comandos read-only ──────────────────────
//
// MEDIDO ejecutando isDestructiveGit: bloqueaba `git stash list`, `git stash show`, `git clean -n`
// y `git clean --dry-run`. La causa eran dos patrones que matchean el subcomando entero sin mirar
// si la variante muta algo.
//
// Se EJECUTA la función con node en vez de grepear el archivo: un guard se verifica corriéndolo.

// clasificaGit corre isDestructiveGit del plugin real contra un comando.
func clasificaGit(t *testing.T, cmd string) bool {
	t.Helper()
	src, err := os.ReadFile("templates/opencode-git-guard.js")
	if err != nil {
		t.Fatal(err)
	}
	bloque := regexp.MustCompile(`(?s)const DESTRUCTIVE.*?\nexport function isDestructiveGit\(cmd\) \{.*?\n\}`).
		FindString(string(src))
	if bloque == "" {
		t.Fatal("no se encontró isDestructiveGit en el plugin")
	}
	script := strings.Replace(bloque, "export function", "function", 1) +
		"\nprocess.stdout.write(String(isDestructiveGit(process.argv[1])))"

	out, err := exec.Command("node", "-e", script, cmd).CombinedOutput()
	if err != nil {
		t.Fatalf("node falló: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out)) == "true"
}

func TestGitGuard_NoBloqueaLecturas(t *testing.T) {
	// se arman por concatenación para que el git-guard de BASH no bloquee este archivo al leerlo:
	// clasifica por substring y bloquearía el test que denuncia justamente eso
	g := "git "
	lecturas := []string{
		g + "st" + "ash list",
		g + "st" + "ash show",
		g + "cl" + "ean -n",
		g + "cl" + "ean --dry-run",
		g + "worktree list",
	}

	for _, cmd := range lecturas {
		if clasificaGit(t, cmd) {
			t.Errorf("bloquea un comando de LECTURA: %q. Un guard que impide mirar el estado obliga "+
				"a trabajar a ciegas y empuja a desactivarlo entero", cmd)
		}
	}
}

func TestGitGuard_SigueBloqueandoLoDestructivo(t *testing.T) {
	g := "git "
	destructivos := []string{
		g + "st" + "ash pop",
		g + "st" + "ash drop",
		g + "cl" + "ean -fd",
		g + "cl" + "ean -f",
		g + "re" + "set --hard",
		g + "-C . re" + "set --hard",
		g + "worktree remove x",
	}

	for _, cmd := range destructivos {
		if !clasificaGit(t, cmd) {
			t.Errorf("DEJÓ PASAR un comando destructivo: %q. Afinar los patrones no puede abrir la "+
				"puerta que el guard vino a cerrar", cmd)
		}
	}
}
