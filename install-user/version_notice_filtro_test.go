package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// El hook nuevo convive con binarios VIEJOS: al desplegar esto, ningún cliente instalado tiene
// todavía --version-check. Y `flag.Usage = printHelp`, con printHelp usando fmt.Println, o sea
// STDOUT: un binario viejo responde al flag desconocido imprimiendo las ~30 líneas del help por
// la misma salida de la que el hook lee el aviso. Sin filtro, ese help entero termina inyectado
// en el bloque de arranque de CADA sesión de todos los clientes existentes.
//
// El fix no puede ser solo arreglar printHelp: los binarios viejos ya están instalados y no se
// los puede cambiar retroactivamente. Tiene que estar del lado del hook.

func TestPrefijoDelAviso_LosDosMensajesLoComparten(t *testing.T) {
	informativo := AvisoDeActualizacion("1.1.0", "1.2.0", "")
	bajoMinimo := AvisoDeActualizacion("1.0.0", "1.4.0", "1.2.0")

	for nombre, aviso := range map[string]string{"informativo": informativo, "bajo el mínimo": bajoMinimo} {
		if !strings.Contains(aviso, marcadorDelAviso) {
			t.Errorf("el aviso %s no lleva el marcador %q, así que el hook lo filtraría y no llegaría nunca: %q",
				nombre, marcadorDelAviso, aviso)
		}
	}
}

// El help NO puede llevar el marcador: si lo llevara, el filtro del hook lo dejaría pasar y
// estaríamos en el mismo problema que este test existe para cerrar.
func TestPrefijoDelAviso_ElHelpNoLoLleva(t *testing.T) {
	bin := compilarConVersion(t, "1.1.0")

	out, _ := exec.Command(bin, "--flag-que-no-existe").CombinedOutput()
	for _, linea := range strings.Split(string(out), "\n") {
		if strings.Contains(linea, marcadorDelAviso) {
			t.Fatalf("una línea del help lleva el marcador del aviso y burlaría el filtro del hook: %q", linea)
		}
	}
}

// El guard del hook: sin el filtro, la salida de un binario viejo se pega entera al bloque.
func TestHookSessionStart_FiltraLaSalidaPorElMarcadorDelAviso(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("hooks", "domain-session-start.sh"))
	if err != nil {
		t.Fatal(err)
	}
	hook := string(raw)
	if !strings.Contains(hook, "grep") || !strings.Contains(hook, marcadorDelAviso) {
		t.Fatal("el hook no filtra la salida de --version-check por el marcador del aviso: un binario viejo le inyectaría su help entero al bloque de arranque")
	}
}

// Prueba de punta a punta del modo de falla real: un stand-in que se comporta como un binario
// viejo —help por stdout, exit 2— no debe aportar ni una línea al aviso.
func TestVersionNotice_BinarioViejoQueImprimeElHelp_NoAportaNingunaLinea(t *testing.T) {
	dir := t.TempDir()
	viejo := filepath.Join(dir, "domain-install")
	script := "#!/bin/sh\n" +
		"echo 'domain-install — instalador cross-platform del cliente MCP domain.'\n" +
		"echo 'Uso:'\n" +
		"echo '  domain-install --url http://1.2.3.4'\n" +
		"exit 2\n"
	if err := os.WriteFile(viejo, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	// misma tubería que usa el hook: capturar stdout, descartar stderr, filtrar por el marcador
	cmd := exec.Command("sh", "-c",
		`"`+viejo+`" --version-check 1.2.0 "" 2>/dev/null | grep -m1 -- '`+marcadorDelAviso+`' || true`)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("la tubería del hook no puede fallar: %v", err)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Fatalf("el help de un binario viejo se coló al aviso: %q", out)
	}
}
