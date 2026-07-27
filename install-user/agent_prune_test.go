package main

import (
	"os"
	"path/filepath"
	"testing"
)

// DOMAINSERV-137: un agente que salía del catálogo del repo sobrevivía para siempre en
// ~/.claude/agents/, y desinstalar dejaba los guards huérfanos. El manifest de procedencia
// resuelve las dos cosas sin heurísticas: lo que domain escribió está registrado.

func catalogoDe(slugs ...string) []agentTemplate {
	cat := make([]agentTemplate, 0, len(slugs))
	for _, s := range slugs {
		cat = append(cat, agentTemplate{
			slug:   s,
			claude: []byte("---\nname: " + s + "\nmodel: sonnet\n---\n" + s + "\n"),
		})
	}
	return cat
}

// El caso central: el catálogo se reduce y el agente retirado se va del cliente.
func TestInstalarAgentes_AgenteRetiradoDelCatalogo_SeRemueveDelCliente(t *testing.T) {
	paths := pathsDePrueba(t)
	if _, err := instalarAgentes(paths, catalogoDe("se-queda", "se-retira")); err != nil {
		t.Fatalf("primera corrida: %v", err)
	}
	retirado := filepath.Join(paths.GlobalAgentsDir, "se-retira.md")
	if _, err := os.Stat(retirado); err != nil {
		t.Fatalf("se-retira debía estar instalado: %v", err)
	}

	res, err := instalarAgentes(paths, catalogoDe("se-queda"))
	if err != nil {
		t.Fatalf("segunda corrida: %v", err)
	}

	if _, err := os.Stat(retirado); !os.IsNotExist(err) {
		t.Error("un agente fuera del catálogo debe removerse en la instalación siguiente")
	}
	if _, err := os.Stat(filepath.Join(paths.GlobalAgentsDir, "se-queda.md")); err != nil {
		t.Errorf("se-queda no debía tocarse: %v", err)
	}
	if len(res.removidos) != 1 || res.removidos[0] != "se-retira" {
		t.Errorf("la remoción debe reportarse, removidos = %v", res.removidos)
	}
	if m := cargarManifiesto(paths.AgentsManifest); len(m) != 1 {
		t.Errorf("el manifest debe quedar solo con se-queda: %v", m)
	}
}

// Un agente que el usuario puso a mano NUNCA estuvo en el manifest, así que el pruning no
// tiene por qué conocerlo — y no puede tocarlo.
func TestInstalarAgentes_AgenteAjenoAlManifiesto_NoSeRemueve(t *testing.T) {
	paths := pathsDePrueba(t)
	if _, err := instalarAgentes(paths, catalogoDe("del-catalogo")); err != nil {
		t.Fatalf("instalarAgentes: %v", err)
	}
	ajeno := filepath.Join(paths.GlobalAgentsDir, "mi-agente-propio.md")
	if err := os.WriteFile(ajeno, []byte("mío\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := instalarAgentes(paths, catalogoDe("del-catalogo"))
	if err != nil {
		t.Fatalf("segunda corrida: %v", err)
	}

	if _, err := os.Stat(ajeno); err != nil {
		t.Error("un agente que domain nunca escribió no se remueve")
	}
	if len(res.removidos) != 0 {
		t.Errorf("no había nada que remover, removidos = %v", res.removidos)
	}
}

// Misma semántica que en la instalación: si el usuario lo editó, su trabajo no se descarta
// por un cambio de catálogo. Se reporta y se deja.
func TestInstalarAgentes_AgenteRetiradoPeroEditado_NoSeBorraYSeReporta(t *testing.T) {
	paths := pathsDePrueba(t)
	if _, err := instalarAgentes(paths, catalogoDe("se-queda", "se-retira")); err != nil {
		t.Fatalf("primera corrida: %v", err)
	}
	retirado := filepath.Join(paths.GlobalAgentsDir, "se-retira.md")
	editado := "---\nname: se-retira\n---\nlo edité y lo quiero conservar\n"
	if err := os.WriteFile(retirado, []byte(editado), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := instalarAgentes(paths, catalogoDe("se-queda"))
	if err != nil {
		t.Fatalf("segunda corrida: %v", err)
	}

	b, err := os.ReadFile(retirado)
	if err != nil {
		t.Fatalf("el agente editado no debía borrarse: %v", err)
	}
	if string(b) != editado {
		t.Errorf("el contenido editado se alteró: %q", b)
	}
	if len(res.conflictos) != 1 || res.conflictos[0] != "se-retira" {
		t.Errorf("debe reportarse como conflicto, conflictos = %v", res.conflictos)
	}
	if len(res.removidos) != 0 {
		t.Errorf("un agente editado no se cuenta como removido: %v", res.removidos)
	}
}

// Desinstalación simétrica: los guards son parte de lo instalado, así que también salen.
// Un guard que sobrevive apunta a un agente que ya no existe.
func TestDesinstalarAgentes_RemueveLosGuardsDelCatalogo(t *testing.T) {
	paths := pathsDePrueba(t)
	paths.AgentHooksDir = filepath.Join(t.TempDir(), "hooks")
	cat := []agentTemplate{{
		slug:   "con-guard",
		claude: []byte("---\nname: con-guard\nhooks:\n  PreToolUse:\n    - matcher: \"Bash\"\n      hooks:\n        - type: command\n          command: \"guard.sh\"\n---\ncuerpo\n"),
		guards: map[string][]byte{"guard.sh": []byte("#!/bin/sh\nexit 0\n")},
	}}

	if _, err := instalarAgentes(paths, cat); err != nil {
		t.Fatalf("instalarAgentes: %v", err)
	}
	guard := filepath.Join(paths.AgentHooksDir, "guard.sh")
	if _, err := os.Stat(guard); err != nil {
		t.Fatalf("el guard debía instalarse: %v", err)
	}

	desinstalarAgentes(paths, cat)

	if _, err := os.Stat(guard); !os.IsNotExist(err) {
		t.Error("el guard debe removerse con su agente: uno huérfano apunta a un agente que ya no existe")
	}
}

// El doctor tiene que ver el catálogo, no solo hooks y MCP. Un agente ausente lo repara
// re-correr el install, así que es crítico.
func TestCheckAgentCatalog_AgenteAusente_EsCritico(t *testing.T) {
	paths := pathsDePrueba(t)
	cat := catalogoDe("presente", "ausente")
	if _, err := instalarAgentes(paths, cat); err != nil {
		t.Fatalf("instalarAgentes: %v", err)
	}
	if err := os.Remove(filepath.Join(paths.GlobalAgentsDir, "ausente.md")); err != nil {
		t.Fatal(err)
	}

	if got := checkAgentCatalog(paths, cat); got == 0 {
		t.Error("un agente ausente debe sumar a critical: re-correr el install lo repara")
	}
}

// Un conflicto es una decisión del usuario y re-correr el install NO la va a cambiar, así
// que informar es correcto y fallar no.
func TestCheckAgentCatalog_AgenteEditadoPorElUsuario_NoEsCritico(t *testing.T) {
	paths := pathsDePrueba(t)
	cat := catalogoDe("editado")
	if _, err := instalarAgentes(paths, cat); err != nil {
		t.Fatalf("instalarAgentes: %v", err)
	}
	if err := os.WriteFile(filepath.Join(paths.GlobalAgentsDir, "editado.md"),
		[]byte("lo cambié yo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := checkAgentCatalog(paths, cat); got != 0 {
		t.Errorf("una edición deliberada no es una falla de instalación, critical = %d", got)
	}
}

// El catálogo íntegro no reporta nada.
func TestCheckAgentCatalog_CatalogoCompleto_NoSumaCritical(t *testing.T) {
	paths := pathsDePrueba(t)
	cat := catalogoDe("uno", "dos")
	if _, err := instalarAgentes(paths, cat); err != nil {
		t.Fatalf("instalarAgentes: %v", err)
	}

	if got := checkAgentCatalog(paths, cat); got != 0 {
		t.Errorf("con el catálogo completo no hay nada crítico, critical = %d", got)
	}
}
