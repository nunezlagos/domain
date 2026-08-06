package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// DOMAINSERV-245: un subagente comparte el session_id del padre —medido con un experimento
// directo: se lanzó un subagente cuya única tarea era correr `go test` y el marker que apareció
// llevaba el session_id del hilo principal, no uno propio—. O sea que CUALQUIER subagente puede
// acuñar el marker que autoriza el commit del padre, y no hay ningún campo del payload del hook
// que permita distinguirlo.
//
// Como distinguirlo es imposible, el ticket resuelve la tensión por el otro lado, que es el que
// sí se puede medir: el marker registra QUÉ RECORRIÓ, y el gate exige que lo cubierto incluya
// lo que se va a commitear. Con eso un subagente que corrió la suite del módulo que se está
// tocando SÍ es evidencia válida, y uno que corrió otra cosa no — sin necesidad de saber quién
// lo escribió. Ese mecanismo es el fix compartido con DOMAINSERV-237.
//
// Lo que falta y este test fija: que el deny diga las DOS MITADES. Con solo "no cubrió estos
// archivos", quien lo recibe no sabe si corrió la suite equivocada o no corrió nada, y son
// arreglos distintos. Con subagentes importa más: el alcance registrado es el único dato que
// dice si la corrida ajena sirve para este commit.

func TestCommitGate_DenyPorAlcance_DiceQueSeCorrioYQueFaltaba(t *testing.T) {
	dir := repoGitDePrueba(t)
	escribirArchivo(t, dir, "main.go", "package main\n\nfunc main() { println(1) }\n")
	gitFixture(t, dir, "add", "-A")

	home := t.TempDir()
	// la corrida cubre ./templates/... y el commit toca main.go: el alcance no alcanza
	correrHookEnRepo(t, "domain-post-test.sh", home, dir,
		payloadCorrida(t, "go test -count=1 ./templates/...", "ok  domain/templates  0.2s"))
	if !markerExiste(t, home) {
		t.Fatal("la corrida no dejó marker: sin marker este caso no mide el mensaje del deny")
	}

	out := correrHookEnRepo(t, "domain-pre-edit.sh", home, dir, payloadIntentoDeCommit(t))

	if !strings.Contains(out, `"deny"`) {
		t.Fatalf("una corrida de ./templates/... habilitó el commit de main.go: %s", out)
	}
	// mitad 1: qué faltaba
	if !strings.Contains(out, "main.go") {
		t.Errorf("el deny no nombra el archivo que quedó sin cubrir, así que no dice qué arreglar: %s", out)
	}
	// mitad 2: qué se corrió — es la que agrega este ticket.
	// Se busca sin acentos a propósito: la salida del hook es JSON y los escapa a \uXXXX,
	// así que una aserción con "SÍ" literal falla por el encoding y no por el mensaje.
	if !strings.Contains(out, "Lo que S") || !strings.Contains(out, "se corri") {
		t.Errorf("el deny no dice qué alcance registró el marker: quien lo recibe no puede "+
			"distinguir 'corrí la suite equivocada' de 'no corrí nada', y con un marker escrito "+
			"por un subagente esa distinción es todo lo que hay: %s", out)
	}
	if !strings.Contains(out, "templates") {
		t.Errorf("el deny dice que algo se corrió pero no CUÁL: el alcance concreto es el dato "+
			"que permite decidir si esa corrida servía: %s", out)
	}
}

// La contracara: cuando NO hay marker en absoluto, el deny no puede inventar un alcance. El
// mensaje es el otro, y tiene que seguir siendo distinguible del de arriba — si los dos casos
// dijeran lo mismo, el campo nuevo no aportaría nada.
func TestCommitGate_SinMarker_ElDenyNoInventaUnAlcance(t *testing.T) {
	dir := repoGitDePrueba(t)
	escribirArchivo(t, dir, "main.go", "package main\n\nfunc main() { println(2) }\n")
	gitFixture(t, dir, "add", "-A")

	home := t.TempDir()
	// sin correr domain-post-test.sh: no hay marker
	out := correrHookEnRepo(t, "domain-pre-edit.sh", home, dir, payloadIntentoDeCommit(t))

	if !strings.Contains(out, `"deny"`) {
		t.Fatalf("sin ninguna corrida de tests el commit quedó habilitado: %s", out)
	}
	if strings.Contains(out, "Lo que S") && strings.Contains(out, "se corri") {
		t.Errorf("sin marker el deny afirma un alcance que nadie registró: %s", out)
	}
}

// Y el caso legítimo que no hay que romper: un subagente que corre la suite que SÍ cubre los
// archivos staged habilita el commit. Es el primer criterio de aceptación del ticket, y el que
// impide que el fix se vuelva un gate insatisfacible —el modo de falla de DOMAINSERV-111/175/195,
// donde un gate imposible empuja al bypass permanente—. Desde el hook es indistinguible de una
// corrida del hilo principal, y eso es exactamente el punto: lo que acredita es el ALCANCE.
func TestCommitGate_CorridaQueCubreLoStaged_HabilitaAunqueLaHayaHechoOtro(t *testing.T) {
	dir := repoGitDePrueba(t)
	escribirArchivo(t, dir, "main.go", "package main\n\nfunc main() { println(3) }\n")
	gitFixture(t, dir, "add", "-A")

	home := t.TempDir()
	base := filepath.Base(dir)
	_ = base
	correrHookEnRepo(t, "domain-post-test.sh", home, dir,
		payloadCorrida(t, "go test -count=1 ./...", "ok  domain  0.3s"))

	out := correrHookEnRepo(t, "domain-pre-edit.sh", home, dir, payloadIntentoDeCommit(t))
	if strings.Contains(out, `"deny"`) {
		t.Errorf("la suite recursiva cubría main.go y el gate igual denegó: un gate que rechaza "+
			"lo legítimo empuja al bypass permanente: %s", out)
	}
}
