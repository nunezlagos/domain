package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// DOMAINSERV-137/139: un agente puede traer su propio guard PreToolUse, y su frontmatter lo
// referencia con ruta relativa. Esa ruta se resuelve contra el cwd, así que en
// ~/.claude/agents/ NO existe: el hook no bloquea nada y el agente queda con Bash SIN
// restricción. Es una falla ABIERTA, y por eso el guard es parte de la instalación.

const frontmatterConGuard = `---
name: con-guard
model: haiku
tools: Bash
hooks:
  PreToolUse:
    - matcher: "Bash"
      hooks:
        - type: command
          command: ".claude/hooks/mi-guard.sh"
---
body
`

func TestGuardsDeclarados_ExtraeLaRutaDelFrontmatter(t *testing.T) {
	guards := guardsDeclarados([]byte(frontmatterConGuard))

	if len(guards) != 1 {
		t.Fatalf("guards = %v, se esperaba 1", guards)
	}
	if guards[0] != "mi-guard.sh" {
		t.Errorf("guards[0] = %q, se esperaba el basename mi-guard.sh", guards[0])
	}
}

func TestGuardsDeclarados_AgenteSinHooks_DevuelveVacio(t *testing.T) {
	if g := guardsDeclarados([]byte("---\nname: sin-guard\nmodel: haiku\n---\nbody\n")); len(g) != 0 {
		t.Errorf("un agente sin hooks no declara guards, dio %v", g)
	}
}

// El guard se copia al directorio de hooks y la referencia del agente instalado queda
// apuntando ahí con ruta ABSOLUTA. Si quedara relativa, el hook seguiría sin resolverse.
func TestInstalarAgentes_ConGuard_LoCopiaYReescribeLaReferencia(t *testing.T) {
	paths := pathsDePrueba(t)
	paths.AgentHooksDir = filepath.Join(t.TempDir(), "domain", "hooks")
	cat := []agentTemplate{{
		slug:   "con-guard",
		claude: []byte(frontmatterConGuard),
		guards: map[string][]byte{"mi-guard.sh": []byte("#!/bin/bash\nexit 0\n")},
	}}

	res, err := instalarAgentes(paths, cat)
	if err != nil {
		t.Fatalf("instalarAgentes: %v", err)
	}

	destinoGuard := filepath.Join(paths.AgentHooksDir, "mi-guard.sh")
	fi, err := os.Stat(destinoGuard)
	if err != nil {
		t.Fatalf("el guard no se instaló: %v", err)
	}
	if fi.Mode().Perm()&0o100 == 0 {
		t.Error("el guard debe quedar ejecutable o el hook no puede correrlo")
	}
	if len(res.guardsInstalados) != 1 {
		t.Errorf("guardsInstalados = %v", res.guardsInstalados)
	}

	b, err := os.ReadFile(filepath.Join(paths.GlobalAgentsDir, "con-guard.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), ".claude/hooks/mi-guard.sh") {
		t.Error("la referencia relativa quedó sin reescribir: no se resolvería desde ~/.claude/agents/")
	}
	if !strings.Contains(string(b), destinoGuard) {
		t.Errorf("la referencia debe apuntar a %s", destinoGuard)
	}
}

// El requisito duro: nunca el hook roto con el tool abierto. Si el guard no está en el
// bundle, el agente NO se instala.
func TestInstalarAgentes_GuardFaltante_NoInstalaElAgente(t *testing.T) {
	paths := pathsDePrueba(t)
	paths.AgentHooksDir = filepath.Join(t.TempDir(), "domain", "hooks")
	cat := []agentTemplate{{slug: "con-guard", claude: []byte(frontmatterConGuard)}}

	res, err := instalarAgentes(paths, cat)
	if err != nil {
		t.Fatalf("un guard faltante no debe abortar la instalación entera: %v", err)
	}

	if _, err := os.Stat(filepath.Join(paths.GlobalAgentsDir, "con-guard.md")); !os.IsNotExist(err) {
		t.Error("el agente se instaló con un guard que no existe: tendría Bash sin restricción")
	}
	if len(res.guardsFaltantes) != 1 || res.guardsFaltantes[0] != "con-guard" {
		t.Errorf("la omisión debe reportarse, guardsFaltantes = %v", res.guardsFaltantes)
	}
	for _, s := range res.instalados {
		if s == "con-guard" {
			t.Error("con-guard no puede figurar como instalado")
		}
	}
}

// El catálogo real: git-archaeology declara un guard, así que el bundle tiene que traerlo.
func TestAgentCatalog_ElGuardDeGitArchaeologyEstaEnElBundle(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}

	ga := buscar(t, cat, "git-archaeology")
	declarados := guardsDeclarados(ga.claude)
	if len(declarados) == 0 {
		t.Fatal("git-archaeology debe declarar su guard: sin él tendría Bash sin restricción")
	}
	for _, nombre := range declarados {
		if len(ga.guards[nombre]) == 0 {
			t.Errorf("el guard %q está declarado pero no viene en el bundle", nombre)
		}
	}
}
