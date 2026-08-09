package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// DOMAINSERV-269: el deny del commit-gate termina recomendando cómo autorizar UN commit.
// Esa instrucción es la última salida cuando la suite no se puede correr, así que si el
// texto se degrada, el gate pasa de "difícil" a "insatisfacible en la práctica".
//
// Hay un modo de falla MEDIDO (2026-08-09) que el texto tiene que seguir previniendo: si el
// `echo` del bypass se escribe en el MISMO comando que el `git commit` —lo más natural, con
// `&&`— el hook inspecciona el comando ANTES de que exista el archivo, deniega el comando
// entero, y por lo tanto el `echo` NUNCA se ejecuta. Quien lo intenta ve exactamente el
// mismo deny de siempre y no tiene señal de que su autorización tampoco corrió.
//
// Reproducido: `echo razón > $bypass && git commit -m x` en un repo limpio -> deny, y el
// archivo de bypass no existe después.
//
// El texto ya lo advierte desde 24139ed9. Ningún test lo cubría, así que se podía perder en
// cualquier reescritura del mensaje sin que nada se pusiera en rojo.

func bypassDe(home string) string {
	return filepath.Join(home, ".local", "state", "domain", "gate-bypass-"+sesionGate)
}

func escribirBypassDeCommitGate(t *testing.T, home, razon string) string {
	t.Helper()
	ruta := bypassDe(home)
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ruta, []byte(razon+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return ruta
}

func TestCommitGateBypass_ElDenyExplicaQueVaEnUnComandoSeparado(t *testing.T) {
	home := t.TempDir()
	repo := repoGitDePrueba(t)

	salida := correrHookEnRepo(t, "domain-pre-edit.sh", home, repo, payloadIntentoDeCommit(t))
	dec := parsearDecision(t, salida)

	if dec.Decision != "deny" && dec.Decision != "ask" {
		t.Fatalf("sin marker de tests el gate tiene que frenar el commit, obtuve %q", dec.Decision)
	}
	// la ruta, para que sea copiable
	if !strings.Contains(dec.Razon, "gate-bypass-") {
		t.Fatalf("el deny no dice DÓNDE se escribe la autorización:\n%s", dec.Razon)
	}
	// y la advertencia que evita el modo de falla medido
	if !strings.Contains(dec.Razon, "SEPARADO") {
		t.Fatalf("el deny recomienda el bypass SIN advertir que va en un comando separado.\n"+
			"Con && el hook deniega el comando entero y el echo nunca corre, así que quien lo\n"+
			"lea va a ver el mismo deny sin saber que su autorización tampoco se ejecutó.\n"+
			"reason:\n%s", dec.Razon)
	}
}

// El bypass tiene que servir de verdad: si el deny lo recomienda y el gate no lo honra, el
// commit queda bloqueado sin salida y el siguiente paso de cualquiera es desactivar el hook.
func TestCommitGateBypass_ConBypassElCommitPasa(t *testing.T) {
	home := t.TempDir()
	repo := repoGitDePrueba(t)
	escribirBypassDeCommitGate(t, home, "la suite depende de un servicio externo")

	salida := correrHookEnRepo(t, "domain-pre-edit.sh", home, repo, payloadIntentoDeCommit(t))
	dec := parsearDecision(t, salida)

	if dec.Decision == "deny" {
		t.Fatalf("con bypass presente el commit no puede denegarse: %s", dec.Razon)
	}
}

// "UN commit", no una sesión entera con el gate apagado.
func TestCommitGateBypass_SeConsumeEnElPrimerUso(t *testing.T) {
	home := t.TempDir()
	repo := repoGitDePrueba(t)
	ruta := escribirBypassDeCommitGate(t, home, "razón cualquiera")

	correrHookEnRepo(t, "domain-pre-edit.sh", home, repo, payloadIntentoDeCommit(t))

	if _, err := os.Stat(ruta); !os.IsNotExist(err) {
		t.Fatal("el bypass sobrevivió al primer uso: dejaría el gate abierto toda la sesión")
	}

	// y el segundo commit vuelve a estar gateado
	dec := parsearDecision(t, correrHookEnRepo(t, "domain-pre-edit.sh", home, repo, payloadIntentoDeCommit(t)))
	if dec.Decision != "deny" && dec.Decision != "ask" {
		t.Fatalf("tras consumirse el bypass el gate tiene que volver a frenar, obtuve %q", dec.Decision)
	}
}

// El bypass está indexado por session_id: el de otra sesión no puede servir, o alcanzaría
// con dejar uno viejo dado vuelta en el state para desactivar el gate de cualquier sesión
// futura. Medido: quedan archivos gate-bypass-* de sesiones anteriores sin consumir.
func TestCommitGateBypass_ElDeOtraSesionNoSirve(t *testing.T) {
	home := t.TempDir()
	repo := repoGitDePrueba(t)
	dir := filepath.Join(home, ".local", "state", "domain")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	ajeno := filepath.Join(dir, "gate-bypass-otra-sesion-cualquiera")
	if err := os.WriteFile(ajeno, []byte("razón de otra sesión\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	dec := parsearDecision(t, correrHookEnRepo(t, "domain-pre-edit.sh", home, repo, payloadIntentoDeCommit(t)))

	if dec.Decision != "deny" && dec.Decision != "ask" {
		t.Fatalf("un bypass de OTRA sesión no puede destrabar este commit, obtuve %q", dec.Decision)
	}
	if _, err := os.Stat(ajeno); os.IsNotExist(err) {
		t.Fatal("consumió el bypass de otra sesión: lo gasta sin haberlo honrado")
	}
}
