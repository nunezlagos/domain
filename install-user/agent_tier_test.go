package main

import (
	"strings"
	"testing"
)

// DOMAINSERV-164: las variantes .opencode.md quedaron en haiku mientras las de Claude Code
// pasaron a sonnet, así que el mismo agente corría en dos tiers según el harness. Estos
// guards existen para que la divergencia no vuelva a pasar en silencio, igual que
// agent_voseo_test.go hace con el voseo sobre estos mismos templates.

// tierEsperadoOpencode es el ID que OpenCode necesita para el tier que la policy
// modelo-por-clase-de-tarea fija para agentes con contrato de retorno. OpenCode exige el ID
// completo con provider, no el alias `sonnet` de Claude Code.
const tierEsperadoOpencode = "anthropic/claude-sonnet-5"

// Una variante en un tier más bajo que su agente conserva el contrato de retorno de tres
// estados en el cuerpo pero lo corre en el tier que se descartó por no poder sostenerlo: el
// retorno queda indistinguible de uno bueno.
func TestCatalogo_VariantesOpencode_NoDiververGenDeTier(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}

	var conVariante int
	for _, a := range cat {
		if len(a.opencode) == 0 {
			continue
		}
		conVariante++
		modelo := campoDelFrontmatter(a.opencode, "model:")
		if modelo != tierEsperadoOpencode {
			t.Errorf("%s.opencode.md declara model %q, se esperaba %q — su variante de Claude Code corre en %q",
				a.slug, modelo, tierEsperadoOpencode, campoDelFrontmatter(a.claude, "model:"))
		}
	}
	if conVariante == 0 {
		t.Fatal("ninguna variante .opencode.md en el catálogo: el guard no está mirando nada")
	}
}

// `temperature` con un valor no-default es un 400 en claude-sonnet-5, así que una variante que
// la declare rompe el agente en cada invocación. No es una preferencia de estilo.
func TestCatalogo_VariantesOpencode_NoDeclaranTemperature(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}

	for _, a := range cat {
		if len(a.opencode) == 0 {
			continue
		}
		if temp := campoDelFrontmatter(a.opencode, "temperature:"); temp != "" {
			t.Errorf("%s.opencode.md declara temperature %q — la API la rechaza con 400 en %s",
				a.slug, temp, tierEsperadoOpencode)
		}
	}
}

// El ID de OpenCode lleva provider y no lleva sufijo de fecha. Un `sonnet` pelado (el alias de
// Claude Code) o un `claude-sonnet-5-20260101` inventado no resuelven del lado de OpenCode.
func TestCatalogo_VariantesOpencode_ModelLlevaProviderYNoLlevaFecha(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}

	for _, a := range cat {
		if len(a.opencode) == 0 {
			continue
		}
		modelo := campoDelFrontmatter(a.opencode, "model:")
		if !strings.Contains(modelo, "/") {
			t.Errorf("%s.opencode.md: model %q sin provider — OpenCode espera provider/model-id", a.slug, modelo)
		}
		if sufijoDeFecha(modelo) {
			t.Errorf("%s.opencode.md: model %q lleva sufijo de fecha; los IDs se usan tal cual", a.slug, modelo)
		}
	}
}

// sufijoDeFecha detecta un `-20260101` al final del ID.
func sufijoDeFecha(modelo string) bool {
	partes := strings.Split(modelo, "-")
	ultima := partes[len(partes)-1]
	if len(ultima) != 8 {
		return false
	}
	for _, r := range ultima {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
