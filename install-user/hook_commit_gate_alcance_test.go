package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// DOMANSERV-237: el commit-gate acepta CUALQUIER `go test` como prueba de todo el
// repo. No mira qué paquetes recorrió, ni si la corrida vino del cache, así que
// `go test ./unpaquete/` servido del cache habilita un commit de un archivo que
// ningún test evaluó.
//
// El orden importa y es lo que hace al defecto REAL y no teórico: si se edita
// DESPUÉS de la corrida, el code_hash de DOMAINSERV-219 lo caza. El exploit
// necesita el orden `editar → add → test → commit`, que es exactamente el orden
// documentado del flujo de trabajo.
//
// Y DOMAINSERV-245 se cierra por el mismo lado: un subagente comparte el
// session_id del padre (medido), así que su corrida escribe el marker del padre.
// No hay ningún campo del payload que permita distinguirlo, y por eso "rechazar
// los markers de subagentes" no es implementable. Lo que sí se puede exigir es
// que la corrida CUBRA los archivos que se van a commitear: entonces la corrida
// de un subagente vale cuando cubre lo que se commitea, y no vale cuando no.

func payloadCorrida(t *testing.T, comando, stdout string) string {
	t.Helper()
	return payloadJSON(t, map[string]any{
		"session_id":      sesionGate,
		"hook_event_name": "PostToolUse",
		"tool_name":       "Bash",
		"tool_input":      map[string]any{"command": comando},
		"tool_response":   map[string]any{"stdout": stdout, "stderr": "", "interrupted": false},
	})
}

func markerExiste(t *testing.T, home string) bool {
	t.Helper()
	_, err := filepath.Glob(filepath.Join(home, ".local", "state", "domain", "tests-ok-*"))
	if err != nil {
		t.Fatal(err)
	}
	m, _ := filepath.Glob(filepath.Join(home, ".local", "state", "domain", "tests-ok-*"))
	return len(m) > 0
}

// El corazón de DOMAINSERV-237, y el caso que ejercita al VALIDADOR: la corrida
// califica como prueba para el escritor (recursiva y con -count=1) pero su alcance
// es ./templates/..., así que no evaluó main.go.
//
// Que la corrida sea VÁLIDA es lo que hace útil a este test. Una primera versión
// usaba `go test ./templates/` —sin ./... ni -count=1— y pasaba en verde porque el
// ESCRITOR la rechazaba: el validador nunca se ejercitaba. Dos capas de defensa
// enmascarándose una a la otra, con el test declarando cubrir la que no toca.
func TestCommitGate_CorridaValidaDeOtroSubarbol_NoHabilitaElCommit(t *testing.T) {
	dir := repoGitDePrueba(t)
	// se edita ANTES de la corrida: así el code_hash de DOMAINSERV-219 queda
	// satisfecho y el único guard que puede objetar es el de alcance
	escribirArchivo(t, dir, "main.go", "package main\n\nfunc main() { println(999) }\n")
	gitFixture(t, dir, "add", "-A")

	home := t.TempDir()
	correrHookEnRepo(t, "domain-post-test.sh", home, dir,
		payloadCorrida(t, "go test -count=1 ./templates/...", "ok  domain/templates  0.2s"))
	if !markerExiste(t, home) {
		t.Fatal("la corrida no dejó marker: entonces este caso mide al escritor, no al validador")
	}

	out := correrHookEnRepo(t, "domain-pre-edit.sh", home, dir, payloadIntentoDeCommit(t))
	if !strings.Contains(out, `"deny"`) || !strings.Contains(out, "commit-gate") {
		t.Errorf("una corrida de ./templates/... habilitó el commit de main.go, que no evaluó: %s", out)
	}
}

// Y la corrida de un paquete suelto no llega ni a ser prueba: la rechaza el
// escritor. Es el otro lado del par, y separarlos es lo que permite saber CUÁL de
// las dos capas se rompió cuando algo falla.
func TestCommitGate_CorridaDeUnPaqueteSuelto_NoEsPrueba(t *testing.T) {
	dir := repoGitDePrueba(t)
	escribirArchivo(t, dir, "main.go", "package main\n\nfunc main() { println(998) }\n")
	gitFixture(t, dir, "add", "-A")

	home := t.TempDir()
	correrHookEnRepo(t, "domain-post-test.sh", home, dir,
		payloadCorrida(t, "go test -count=1 ./templates/", "ok  domain/templates  (cached)"))
	if markerExiste(t, home) {
		t.Error("una corrida de un paquete suelto dejó marker: el escritor la aceptó como prueba")
	}
}

// La contracara, y la mitad del guard: la suite recursiva con -count=1 SÍ cubre
// el repo y tiene que seguir habilitando. Sin este caso el fix podría cerrar el
// gate de más y volverlo insatisfacible, que es el modo de falla de
// DOMAINSERV-111/175/195 — un gate imposible empuja al bypass permanente.
func TestCommitGate_SuiteRecursivaConCountUno_HabilitaElCommit(t *testing.T) {
	dir := repoGitDePrueba(t)
	escribirArchivo(t, dir, "main.go", "package main\n\nfunc main() { println(2) }\n")
	gitFixture(t, dir, "add", "-A")

	home := t.TempDir()
	correrHookEnRepo(t, "domain-post-test.sh", home, dir,
		payloadCorrida(t, "go test -count=1 ./...", "ok  domain  0.4s"))

	out := correrHookEnRepo(t, "domain-pre-edit.sh", home, dir, payloadIntentoDeCommit(t))
	if strings.Contains(out, `"deny"`) {
		t.Errorf("la suite recursiva con -count=1 cubre el repo y el gate la rechazó: %s", out)
	}
}

// Sin -count=1 la corrida puede venir enteramente del cache, así que no es
// evidencia de nada. Es el hallazgo que destapó el sabotaje de DOMAINSERV-231.
func TestCommitGate_SuiteRecursivaSinCountUno_NoEsPrueba(t *testing.T) {
	dir := repoGitDePrueba(t)
	escribirArchivo(t, dir, "main.go", "package main\n\nfunc main() { println(3) }\n")
	gitFixture(t, dir, "add", "-A")

	home := t.TempDir()
	correrHookEnRepo(t, "domain-post-test.sh", home, dir,
		payloadCorrida(t, "go test ./...", "ok  domain  (cached)"))

	out := correrHookEnRepo(t, "domain-pre-edit.sh", home, dir, payloadIntentoDeCommit(t))
	if !strings.Contains(out, `"deny"`) {
		t.Errorf("una corrida sin -count=1 puede ser 100%% cache y el gate la aceptó como prueba: %s", out)
	}
}

// `-run` acota la suite a un subconjunto: `go test -count=1 -run TestNada ./...`
// sale verde sin ejecutar UN test. La evasión no es solo el cache.
func TestCommitGate_SuiteAcotadaConRun_NoEsPrueba(t *testing.T) {
	dir := repoGitDePrueba(t)
	escribirArchivo(t, dir, "main.go", "package main\n\nfunc main() { println(4) }\n")
	gitFixture(t, dir, "add", "-A")

	home := t.TempDir()
	correrHookEnRepo(t, "domain-post-test.sh", home, dir,
		payloadCorrida(t, "go test -count=1 -run TestNada ./...", "ok  domain  0.1s"))

	out := correrHookEnRepo(t, "domain-pre-edit.sh", home, dir, payloadIntentoDeCommit(t))
	if !strings.Contains(out, `"deny"`) {
		t.Errorf("-run acota la suite a nada y el gate la aceptó como prueba: %s", out)
	}
}

// Sub-decisión que evita volver el gate insatisfacible: una corrida que NO vale
// como prueba no puede BORRAR el marker de una que sí valía. Si borrara, correr
// `go test ./unpaquete` después de la suite completa dejaría el gate cerrado sin
// forma de reabrirlo salvo el bypass.
func TestCommitGate_CorridaSinAlcanceDespuesDeUnaValida_NoDestruyeLaEvidencia(t *testing.T) {
	dir := repoGitDePrueba(t)
	escribirArchivo(t, dir, "main.go", "package main\n\nfunc main() { println(5) }\n")
	gitFixture(t, dir, "add", "-A")

	home := t.TempDir()
	correrHookEnRepo(t, "domain-post-test.sh", home, dir,
		payloadCorrida(t, "go test -count=1 ./...", "ok  domain  0.4s"))
	if !markerExiste(t, home) {
		t.Fatal("la corrida válida no dejó marker: el resto del caso no mide nada")
	}
	// y ahora una corrida que no prueba nada
	correrHookEnRepo(t, "domain-post-test.sh", home, dir,
		payloadCorrida(t, "go test ./templates/", "ok  domain/templates  (cached)"))

	out := correrHookEnRepo(t, "domain-pre-edit.sh", home, dir, payloadIntentoDeCommit(t))
	if strings.Contains(out, `"deny"`) {
		t.Errorf("una corrida sin alcance borró la evidencia de una válida y dejó el gate insatisfacible: %s", out)
	}
}

// El bypass humano de un solo uso NO se toca: existe para cuando la suite no es
// ejecutable, y romperlo deja al humano sin salida.
func TestCommitGate_BypassHumano_SigueHabilitando(t *testing.T) {
	dir := repoGitDePrueba(t)
	escribirArchivo(t, dir, "main.go", "package main\n\nfunc main() { println(6) }\n")
	gitFixture(t, dir, "add", "-A")

	home := t.TempDir()
	escribirArchivo(t, home, filepath.Join(".local", "state", "domain", "gate-bypass-"+sesionGate),
		"la suite depende de un servicio externo\n")

	out := correrHookEnRepo(t, "domain-pre-edit.sh", home, dir, payloadIntentoDeCommit(t))
	if strings.Contains(out, `"deny"`) {
		t.Errorf("el bypass humano dejó de habilitar el commit: %s", out)
	}
}
