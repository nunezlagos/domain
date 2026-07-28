package main

import (
	"strings"
	"testing"
)

// DOMAINSERV-182 dejó project_slug OBLIGATORIO en domain_knowledge_search y
// domain_knowledge_get para cerrar un leak cross-tenant, pero el template de
// domain-memory seguía instruyendo `domain_knowledge_search(query)` a secas. El agente
// hacía la llamada tal como se la enseñaban y recibía "query y project_slug son
// requeridos": el prompt lo mandaba a un error.
//
// Es el modo de falla de un cambio de contrato de INPUT. La policy
// mcp-response-shape-contract obliga a verificar consumidores al cambiar el shape del
// RESPONSE, y un template que documenta una firma es consumidor del shape del REQUEST:
// nadie lo estaba mirando.

// toolsQueExigenProjectSlug son las tools cuyo project_slug NO es opcional. Si un
// template las nombra con una lista de argumentos, esa lista tiene que incluirlo.
var toolsQueExigenProjectSlug = []string{
	"domain_knowledge_search",
	"domain_knowledge_get",
}

// firmaDocumentada extrae los argumentos de la PRIMERA aparición de `tool(...)` en el
// body. Devuelve ok=false cuando el template solo nombra la tool sin paréntesis, que es
// legítimo: mencionarla en una tabla de paths no es documentar su firma.
func firmaDocumentada(body, tool string) (args string, ok bool) {
	i := strings.Index(body, tool+"(")
	if i < 0 {
		return "", false
	}
	resto := body[i+len(tool)+1:]
	fin := strings.Index(resto, ")")
	if fin < 0 {
		return "", false
	}
	return resto[:fin], true
}

// Un template que documenta la firma vieja le enseña al agente una llamada que la API
// rechaza. El fallo no es silencioso —la tool devuelve error— pero el agente igual
// queda inutilizable para ese paso de su procedimiento.
func TestAgentTemplates_FirmaDocumentada_IncluyeProjectSlugDondeEsObligatorio(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}

	var verificadas int
	for _, a := range cat {
		for _, variante := range []struct {
			nombre string
			tpl    []byte
		}{
			{"claude", a.claude},
			{"opencode", a.opencode},
		} {
			if len(variante.tpl) == 0 {
				continue
			}
			for _, tool := range toolsQueExigenProjectSlug {
				args, ok := firmaDocumentada(string(variante.tpl), tool)
				if !ok {
					continue
				}
				verificadas++
				if !strings.Contains(args, "project_slug") {
					t.Errorf("%s.%s documenta %s(%s) sin project_slug, que es obligatorio: el agente va a recibir un error",
						a.slug, variante.nombre, tool, args)
				}
			}
		}
	}
	if verificadas == 0 {
		t.Fatal("ningún template documenta la firma de estas tools: el guard no está mirando nada")
	}
}

// El project_slug de estas tools NO es opcional, así que documentarlo con el sufijo `?`
// —la convención que el propio template usa para el de domain_mem_search— le dice al
// agente que puede omitirlo. Puede, y falla.
func TestAgentTemplates_ProjectSlugObligatorio_NoSeDocumentaComoOpcional(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}

	for _, a := range cat {
		for _, tpl := range [][]byte{a.claude, a.opencode} {
			if len(tpl) == 0 {
				continue
			}
			for _, tool := range toolsQueExigenProjectSlug {
				args, ok := firmaDocumentada(string(tpl), tool)
				if !ok {
					continue
				}
				if strings.Contains(args, "project_slug?") {
					t.Errorf("%s documenta %s con project_slug? (opcional) y es obligatorio", a.slug, tool)
				}
			}
		}
	}
}
