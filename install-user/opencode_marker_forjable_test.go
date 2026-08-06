package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// DOMAINSERV-233, primer incremento. MEDIDO el 2026-08-06: el commit-gate del plugin de OpenCode
// acepta un marker tests-ok creado con `touch`. Su validación entera era una resta de timestamps
// (opencode-sdd-gate.js:74-80), así que un archivo VACÍO habilitaba el commit.
//
// El hook de bash arregló esto en DOMAINSERV-95 —"sin hash almacenado → NO fresco (fail-closed).
// Un marker legacy (solo-timestamp) o forjado con printf ya no habilita el commit"— y el arreglo
// nunca cruzó al plugin.
//
// El marker lo escribe hooks/domain-post-test.sh:291 con 6 campos separados por tab:
//   timestamp \t tree_hash \t code_hash \t alcance \t runner \t origen
//
// Se prueba el COMPORTAMIENTO ejecutando la función real con node, no grepeando el texto del
// archivo: un guard se verifica corriéndolo.

// freshMarkerDeEsteRepo extrae la función del plugin y la evalúa en node contra un marker dado.
func freshMarkerDeEsteRepo(t *testing.T, contenido string) bool {
	t.Helper()

	src, err := os.ReadFile("templates/opencode-sdd-gate.js")
	if err != nil {
		t.Fatalf("no se pudo leer el plugin: %v", err)
	}
	// DOMAINSERV-247: freshMarker dejó de ser autocontenida — ahora llama a codeHashDelRepo y a
	// raizDelRepo para comparar el hash contra el working tree. Extraer solo freshMarker las
	// dejaba undefined, la excepción caía al catch y TODO devolvía false: un falso rojo que
	// parecía un fallo del gate.
	var fn string
	for _, nombre := range []string{"freshMarker", "codeHashDelRepo", "raizDelRepo"} {
		bloque := regexp.MustCompile(`(?s)function ` + nombre + `\(.*?\n\}`).FindString(string(src))
		if bloque == "" {
			t.Fatalf("no se encontró %s en el plugin", nombre)
		}
		fn += bloque + "\n"
	}

	marker := filepath.Join(t.TempDir(), "tests-ok-sesion")
	if err := os.WriteFile(marker, []byte(contenido), 0o600); err != nil {
		t.Fatal(err)
	}

	script := "const {statSync, readFileSync} = require('fs');\n" +
		"const {execFileSync} = require('child_process');\n" +
		"const {createHash} = require('crypto');\n" + fn +
		"\nprocess.stdout.write(String(freshMarker(process.argv[1], 30)))"
	out, err := exec.Command("node", "-e", script, marker).CombinedOutput()
	if err != nil {
		t.Fatalf("node falló: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out)) == "true"
}

// El caso medido: `touch` produce un archivo vacío y eso habilitaba el commit.
func TestOpenCodeFreshMarker_MarkerVacioForjadoConTouch_NoEsFresco(t *testing.T) {
	if freshMarkerDeEsteRepo(t, "") {
		t.Error("un marker VACÍO pasa el commit-gate de OpenCode: `touch ~/.local/state/domain/tests-ok-<sesion>` " +
			"habilita un commit sin haber corrido un solo test. Es el fail-closed de DOMAINSERV-95, que está " +
			"en el hook de bash y nunca cruzó al plugin")
	}
}

// Un marker legacy de solo-timestamp tampoco alcanza: es lo que el propio hook de bash nombra.
func TestOpenCodeFreshMarker_MarkerSoloTimestamp_NoEsFresco(t *testing.T) {
	if freshMarkerDeEsteRepo(t, "2026-08-06T12:00:00-04:00\n") {
		t.Error("un marker de solo-timestamp pasa: no prueba que ninguna corrida haya evaluado el código")
	}
}

// Campos presentes pero hashes VACÍOS: el forjado con printf que DOMAINSERV-95 menciona.
func TestOpenCodeFreshMarker_HashesVacios_NoEsFresco(t *testing.T) {
	if freshMarkerDeEsteRepo(t, "2026-08-06T12:00:00-04:00\t\t\tinstall-user\tgo\tmain\n") {
		t.Error("un marker con los campos presentes pero los dos hashes vacíos pasa: la forma tabulada " +
			"no es evidencia, el hash sí")
	}
}

// El camino legítimo NO se rompe: un marker con el code_hash REAL del repo sigue siendo fresco.
//
// Este test usaba un hash ficticio ("def456codehash") cuando freshMarker solo exigía que el campo
// existiera. DOMAINSERV-247 lo cambió: ahora se COMPARA contra el working tree, así que un hash
// inventado tiene que ser rechazado — y por eso el test pasa a calcular el hash de verdad. No es
// aflojar la aserción: es que el contrato cambió y el caso "legítimo" ahora significa otra cosa.
func TestOpenCodeFreshMarker_MarkerConElCodeHashRealDelRepo_EsFresco(t *testing.T) {
	repo, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skip("no estamos en un repo git")
	}
	real := codeHashJS(t, strings.TrimSpace(string(repo)))
	if real == "" {
		t.Skip("no se pudo calcular el code_hash del repo")
	}

	marker := "2026-08-06T12:00:00-04:00\tabc123treehash\t" + real + "\tinstall-user\tgo\tmain\n"
	if !freshMarkerDeEsteRepo(t, marker) {
		t.Error("un marker con el code_hash real fue rechazado: el commit-gate quedaría " +
			"insatisfacible en OpenCode y eso empuja al bypass permanente (DOMAINSERV-111/175/195)")
	}
}

// Y el que da sentido al cambio: un code_hash que NO corresponde al estado actual se rechaza.
// Es el defecto 1 del ticket — una edición posterior a la corrida invalida el marker.
func TestOpenCodeFreshMarker_CodeHashQueNoCorresponde_NoEsFresco(t *testing.T) {
	viejo := "2026-08-06T12:00:00-04:00\tabc123treehash\t" +
		"0000000000000000000000000000000000000000000000000000000000000000\tinstall-user\tgo\tmain\n"
	if freshMarkerDeEsteRepo(t, viejo) {
		t.Error("un code_hash que no corresponde al working tree pasó: una edición posterior a la " +
			"corrida de tests dejaría el marker válido, que es el defecto 1 de DOMAINSERV-247")
	}
}

// Precedencia de DOMAINSERV-219: si falta el code_hash, el tree_hash del campo 2 alcanza — es el
// marker que dejó escrito la versión anterior del post-test.
func TestOpenCodeFreshMarker_SoloTreeHash_EsFresco(t *testing.T) {
	if !freshMarkerDeEsteRepo(t, "2026-08-06T12:00:00-04:00\tabc123treehash\t\tinstall-user\tgo\tmain\n") {
		t.Error("un marker con tree_hash pero sin code_hash fue rechazado: los markers escritos antes de " +
			"DOMAINSERV-219 quedarían inservibles sin razón")
	}
}
