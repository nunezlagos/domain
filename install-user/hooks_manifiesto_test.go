package main

import (
	"os"
	"path/filepath"
	"testing"
)

// DOMAINSERV-267, el bucle completo. Los tests de hooks_embebidos_test.go cubren la decisión
// por archivo; este cubre lo que el ticket describe como "el canal de distribución roto":
// que un upgrade REALMENTE actualice, y que la segunda corrida ya sepa distinguir.
//
// Medido antes del fix: hubo que hacer `rm -f` de tres hooks a mano para que se actualizaran,
// y sin ese borrado el install reportaba éxito dejando los viejos.

func TestHooksLifecycle_UpgradeDesdeParqueSinManifiesto_ActualizaYRegistra(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "hooks-manifest.json")

	// el estado real de TODA máquina instalada hoy: hooks viejos, cero manifiesto
	viejo := []byte("#!/usr/bin/env bash\n# hook de una version anterior\n")
	for _, spec := range claudeHooks {
		if err := os.WriteFile(filepath.Join(dir, spec.Script), viejo, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "domain-hooks-lib.sh"), viejo, 0o755); err != nil {
		t.Fatal(err)
	}

	if divergentes := instalarHooksLifecycle(dir, manifest); divergentes != 0 {
		t.Fatalf("%d hook(s) quedaron sin actualizar en el upgrade: es exactamente el canal de "+
			"distribución roto que motivó el ticket", divergentes)
	}

	// todos quedaron con el contenido del binario
	for _, spec := range claudeHooks {
		enDisco, err := os.ReadFile(filepath.Join(dir, spec.Script))
		if err != nil {
			t.Fatal(err)
		}
		embebido, _ := hooksFS.ReadFile("hooks/" + spec.Script)
		if string(enDisco) != string(embebido) {
			t.Errorf("%s no se actualizó al contenido del binario", spec.Script)
		}
	}

	// y el manifiesto quedó escrito: sin eso, la corrida siguiente volvería a no poder
	// distinguir un hook viejo de uno editado, y la gracia se aplicaría para siempre
	m := cargarManifiesto(manifest)
	if len(m) == 0 {
		t.Fatal("no se escribió el manifiesto: la próxima corrida seguiría sin poder distinguir " +
			"'quedó viejo' de 'lo editó el usuario'")
	}
	for _, spec := range claudeHooks {
		embebido, _ := hooksFS.ReadFile("hooks/" + spec.Script)
		if m[spec.Script] != sha256Hex(embebido) {
			t.Errorf("el manifiesto no registró %s con el hash instalado", spec.Script)
		}
	}
	if _, ok := m["domain-hooks-lib.sh"]; !ok {
		t.Error("la lib no quedó en el manifiesto: es la que todos los hooks cargan")
	}
}

// La segunda corrida es la que prueba que la gracia fue por única vez: con el manifiesto ya
// escrito, una edición del usuario se respeta en vez de pisarse.
func TestHooksLifecycle_SegundaCorrida_RespetaLaEdicionDelUsuario(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "hooks-manifest.json")

	// primera corrida sobre un directorio limpio: instala y registra
	if divergentes := instalarHooksLifecycle(dir, manifest); divergentes != 0 {
		t.Fatalf("la instalación limpia reportó %d divergentes", divergentes)
	}

	victima := filepath.Join(dir, claudeHooks[0].Script)
	mio := []byte("#!/usr/bin/env bash\n# lo edité después de instalar\n")
	if err := os.WriteFile(victima, mio, 0o755); err != nil {
		t.Fatal(err)
	}

	if divergentes := instalarHooksLifecycle(dir, manifest); divergentes != 1 {
		t.Fatalf("esperaba 1 hook respetado por tener cambios locales, obtuve %d", divergentes)
	}
	actual, err := os.ReadFile(victima)
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != string(mio) {
		t.Error("la segunda corrida pisó una edición del usuario: la gracia tenía que ser por " +
			"única vez, mientras no había manifiesto con qué distinguir")
	}
}

// El retorno tiene que ser CONSUMIBLE: main.go lo ignoraba, y por eso un hook sin actualizar
// no dejaba rastro en el resumen del install.
func TestHooksLifecycle_ReportaCuantosQuedaronSinActualizar(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "hooks-manifest.json")
	instalarHooksLifecycle(dir, manifest)

	// dos ediciones del usuario tras la instalación
	for _, spec := range claudeHooks[:2] {
		if err := os.WriteFile(filepath.Join(dir, spec.Script), []byte("#!/bin/sh\n# mio\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if divergentes := instalarHooksLifecycle(dir, manifest); divergentes != 2 {
		t.Fatalf("el retorno dice %d y hay 2 hooks con cambios locales: si no cuenta bien, el "+
			"resumen del install miente sobre lo que quedó sin actualizar", divergentes)
	}
}
