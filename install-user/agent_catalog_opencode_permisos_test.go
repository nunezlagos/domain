// DOMAINSERV-226: la contención de un agente efímero vive en su allowlist, y cada cliente la
// expresa distinto. Claude Code la hace cumplir con `tools`; OpenCode con `permission`, cuyas
// claves se matchean como patrones contra el NOMBRE de la tool, MCP incluidas.
//
// Las variantes read-only del catálogo declaraban SOLO `edit/write/bash: deny`. Eso acota los
// built-ins y deja TODO el server domain-mcp permitido: ticket-triage podía llamar
// domain_ticket_create y domain-memory podía llamar domain_mem_save, aunque sus contrapartes de
// Claude Code enumeran 3 y 4 tools de lectura. El agujero estaba declarado en el CHANGELOG de
// DOMAINSERV-206 —"merece ticket propio"— y nadie lo abrió.
//
// El guard es BIDIRECCIONAL a propósito: deriva el conjunto esperado de la contraparte de Claude
// Code, así que si mañana una allowlist cambia en un cliente y no en el otro, falla. Una
// divergencia silenciosa entre clientes es exactamente el modo de falla que esto viene a cerrar.
//
// LO QUE ESTE GUARD NO PRUEBA: que OpenCode APLIQUE el bloque. Verifica que el template lo
// DECLARE. Es la misma brecha entre decisión y efecto que DOMAINSERV-222 encontró en el guard de
// borrado, y acá no se puede cerrar desde el repo: exige un OpenCode corriendo.
package main

import (
	"strings"
	"testing"
)

// permisoOpencode traduce un nombre de tool de Claude Code a su clave de `permission` en
// OpenCode. Devuelve ok=false para las que no tienen equivalente y por lo tanto no se esperan
// en la variante.
func permisoOpencode(tool string) (string, bool) {
	if resto, ok := strings.CutPrefix(tool, "mcp__domain-mcp__"); ok {
		return "domain-mcp_" + resto, true
	}
	switch tool {
	case "Read", "Grep", "Glob", "Bash", "Write", "Edit":
		return strings.ToLower(tool), true
	case "ToolSearch":
		// carga schemas de tools ya permitidas; no es una tool de OpenCode
		return "", false
	}
	return "", false
}

func TestAgentCatalog_VariantesOpencode_AcotanLasToolsMcpYNoSoloLosBuiltins(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}

	for _, a := range cat {
		if len(a.opencode) == 0 {
			continue
		}
		perm := lineaDe(frontmatter(t, a.opencode), "permission:")
		if perm == "" {
			t.Errorf("%s: la variante de OpenCode no declara bloque permission", a.slug)
			continue
		}

		// sin el default-deny, toda tool no enumerada queda PERMITIDA, incluido domain-mcp entero
		if !strings.Contains(perm, `"*": deny`) {
			t.Errorf(`%s: la variante no arranca en default-deny ("*": deny), así que las tools MCP quedan permitidas y la allowlist de Claude Code se evapora`, a.slug)
		}

		esperadas := map[string]bool{}
		for _, tool := range toolsDeclaradas(lineaDe(frontmatter(t, a.claude), "tools:")) {
			if clave, ok := permisoOpencode(tool); ok {
				esperadas[clave] = true
			}
		}
		if len(esperadas) == 0 {
			t.Errorf("%s: no se derivó ninguna tool esperada de la contraparte de Claude Code; el guard quedaría verde por vacío", a.slug)
			continue
		}

		minima := sortedKeys(esperadas)
		for _, permitida := range clavesConAccion(perm, "allow") {
			if !esperadas[permitida] {
				t.Errorf("%s: permission permite %q, que su contraparte de Claude Code NO declara: divergencia entre clientes (esperado: %v)", a.slug, permitida, minima)
			}
			delete(esperadas, permitida)
		}
		for _, faltante := range sortedKeys(esperadas) {
			t.Errorf("%s: falta %q permitida en la variante de OpenCode: el agente no puede completar su procedimiento ahí", a.slug, faltante)
		}
	}
}

// Con default-deny las escrituras ya quedan fuera por efecto, pero nada documenta la intención
// ni frena a quien afloje el `"*"` más adelante. El mismo criterio que ya se le exigía a
// knowledge-ingest, aplicado a todo el catálogo.
func TestAgentCatalog_VariantesOpencode_LasEscriturasEstanDenegadasDeFormaExplicita(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}

	for _, a := range cat {
		if len(a.opencode) == 0 {
			continue
		}
		perm := lineaDe(frontmatter(t, a.opencode), "permission:")
		for _, prohibida := range []string{"write", "edit", "bash"} {
			if !strings.Contains(perm, prohibida+": deny") {
				t.Errorf("%s: %q no figura denegada de forma explícita: la prohibición queda implícita en el default", a.slug, prohibida)
			}
		}
	}
}

// Los agentes de solo lectura NO deben permitir NINGUNA tool MCP de escritura. La lista es de
// verbos y no de nombres completos: un `domain_ticket_create` nuevo con otro nombre igual cae.
func TestAgentCatalog_VariantesOpencode_NingunReadOnlyPermiteUnaToolDeEscritura(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}

	// knowledge-ingest es el único agente del catálogo con escritura, por decisión de
	// DOMAINSERV-206, y tiene su propio guard que enumera exactamente lo que puede
	conEscritura := map[string]bool{"knowledge-ingest": true}

	verbosDeEscritura := []string{
		"_save", "_create", "_update", "_delete", "_set", "_register", "_import",
		"_change_status", "_add", "_link", "_edit", "_commit", "_submit",
	}

	for _, a := range cat {
		if len(a.opencode) == 0 || conEscritura[a.slug] {
			continue
		}
		perm := lineaDe(frontmatter(t, a.opencode), "permission:")
		for _, permitida := range clavesConAccion(perm, "allow") {
			if !strings.HasPrefix(permitida, "domain-mcp_") {
				continue
			}
			for _, verbo := range verbosDeEscritura {
				if strings.Contains(permitida, verbo) {
					t.Errorf("%s es read-only y permite %q, que escribe (%q): la escritura la decide el hilo principal", a.slug, permitida, verbo)
				}
			}
		}
	}
}
