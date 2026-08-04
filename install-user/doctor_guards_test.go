package main

import (
	"os"
	"path/filepath"
	"testing"
)

// El caso que da nombre a DOMAINSERV-232: git-archaeology-guard.sh tenía un agujero de
// escritura arbitraria por --output, se arregló en el repo, y el hook INSTALADO siguió con el
// hueco. El doctor decía verde porque solo miraba el .md del agente, nunca el .sh del guard.
// Un guard divergente es peor que uno ausente: el agente cree estar acotado y no lo está.
func catalogoConGuard(guard []byte) []agentTemplate {
	return []agentTemplate{{
		slug:   "con-guard",
		claude: []byte("---\nname: con-guard\nhooks:\n  PreToolUse:\n    - matcher: \"Bash\"\n      hooks:\n        - type: command\n          command: \"guard.sh\"\n---\ncuerpo\n"),
		guards: map[string][]byte{"guard.sh": guard},
	}}
}

func pathsConHooks(t *testing.T) Paths {
	t.Helper()
	paths := pathsDePrueba(t)
	paths.AgentHooksDir = filepath.Join(t.TempDir(), "hooks")
	return paths
}

func TestCheckAgentCatalog_GuardDivergenteEnDisco_EsCritico(t *testing.T) {
	paths := pathsConHooks(t)
	cat := catalogoConGuard([]byte("#!/bin/sh\n# version arreglada\nexit 0\n"))
	if _, err := instalarAgentes(paths, cat); err != nil {
		t.Fatalf("instalarAgentes: %v", err)
	}

	// el hueco: el disco quedó con la versión vieja y vulnerable
	vieja := []byte("#!/bin/sh\n# version con el agujero de --output\nexit 0\n")
	if err := os.WriteFile(filepath.Join(paths.AgentHooksDir, "guard.sh"), vieja, 0o755); err != nil {
		t.Fatal(err)
	}

	if got := checkAgentCatalog(paths, cat); got == 0 {
		t.Error("un guard con contenido distinto al del bundle debe sumar a critical: re-correr el install lo repara, y mientras tanto el agente corre sin la restricción que promete")
	}
}

func TestCheckAgentCatalog_GuardAusente_EsCritico(t *testing.T) {
	paths := pathsConHooks(t)
	cat := catalogoConGuard([]byte("#!/bin/sh\nexit 0\n"))
	if _, err := instalarAgentes(paths, cat); err != nil {
		t.Fatalf("instalarAgentes: %v", err)
	}
	if err := os.Remove(filepath.Join(paths.AgentHooksDir, "guard.sh")); err != nil {
		t.Fatal(err)
	}

	if got := checkAgentCatalog(paths, cat); got == 0 {
		t.Error("un guard ausente deja el tool sin acotar: debe ser crítico")
	}
}

func TestCheckAgentCatalog_GuardIntegro_NoSumaCritical(t *testing.T) {
	paths := pathsConHooks(t)
	cat := catalogoConGuard([]byte("#!/bin/sh\nexit 0\n"))
	if _, err := instalarAgentes(paths, cat); err != nil {
		t.Fatalf("instalarAgentes: %v", err)
	}

	if got := checkAgentCatalog(paths, cat); got != 0 {
		t.Errorf("con el guard instalado y al día no hay nada crítico, critical = %d", got)
	}
}

// El guard se ejecuta, así que tiene que quedar ejecutable. Se asevera el bit del owner y NO
// la igualdad con 0755: escribirEjecutable garantiza 0755 exacto solo en la rama idempotente
// (agent_install.go:162); la rama de escritura usa os.WriteFile, cuyo permiso efectivo es
// 0755 & ~umask, así que con umask 077 un assert de igualdad daría rojo en una máquina ajena.
func TestInstalarAgentes_ElGuardQuedaEjecutablePorElOwner(t *testing.T) {
	paths := pathsConHooks(t)
	cat := catalogoConGuard([]byte("#!/bin/sh\nexit 0\n"))
	if _, err := instalarAgentes(paths, cat); err != nil {
		t.Fatalf("instalarAgentes: %v", err)
	}

	fi, err := os.Stat(filepath.Join(paths.AgentHooksDir, "guard.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm()&0o100 == 0 {
		t.Errorf("un guard no ejecutable no corre, y un hook que no corre no es un guard: modo = %v", fi.Mode().Perm())
	}
}
