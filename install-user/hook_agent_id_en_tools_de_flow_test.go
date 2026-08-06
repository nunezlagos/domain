package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// DOMAINSERV-218 incremento 3. Los hooks YA leen agent_id desde f58464e7 y lo usan para el
// nombre del marker, pero NO se lo mandaban al server: el token se emitía y se validaba sin
// eje de agente, así que N subagentes compartían un allowed_paths y el criterio #2 del ticket
// seguía siendo inalcanzable por más que los markers estuvieran separados.
//
// Se verifica sobre el TEXTO del hook y no ejecutándolo porque el camino real exige un MCP
// arriba con credenciales; lo que este test protege es que el campo viaje en el payload, que
// es exactamente lo que faltaba.

func leerHook(t *testing.T, nombre string) string {
	t.Helper()
	b, err := os.ReadFile("hooks/" + nombre)
	if err != nil {
		t.Fatalf("no se pudo leer el hook %s: %v", nombre, err)
	}
	return string(b)
}

// llamadaATool devuelve el argumento JSON con el que el hook invoca a la tool.
func llamadaATool(t *testing.T, hook, tool string) string {
	t.Helper()
	re := regexp.MustCompile(`domain_call_tool ` + tool + `\s*\\?\s*\n?\s*"(\{[^\n]*)`)
	m := re.FindStringSubmatch(hook)
	if m == nil {
		t.Fatalf("no se encontró la invocación de %s en el hook", tool)
	}
	return m[1]
}

func TestHookPostOrchestrate_GrantToken_MandaElAgentId(t *testing.T) {
	hook := leerHook(t, "domain-post-orchestrate.sh")

	args := llamadaATool(t, hook, "domain_flow_grant_token")

	if !strings.Contains(args, `\"agent_id\"`) {
		t.Errorf("el grant no manda agent_id, así que el server no puede atar el token a un agente "+
			"y dos subagentes del mismo flow siguen compartiendo un único allowed_paths.\ninvocación: %s", args)
	}
}

func TestHookPreEdit_ValidateToken_MandaElAgentId(t *testing.T) {
	hook := leerHook(t, "domain-pre-edit.sh")

	args := llamadaATool(t, hook, "domain_flow_validate_token")

	if !strings.Contains(args, `\"agent_id\"`) {
		t.Errorf("la validación no manda agent_id: el server no puede comparar quién valida contra "+
			"el agente del token, así que el deny por agent_mismatch nunca se dispara.\ninvocación: %s", args)
	}
}

// El agent_id del grant se lee del payload y NO se inventa: si el hook mandara una constante,
// todos los subagentes compartirían identidad y el aislamiento sería decorativo.
func TestHookPostOrchestrate_ElAgentIdSaleDeLaVariableDelPayload(t *testing.T) {
	args := llamadaATool(t, leerHook(t, "domain-post-orchestrate.sh"), "domain_flow_grant_token")

	if !strings.Contains(args, "$agent_id") && !strings.Contains(args, "${agent_id") {
		t.Errorf("el agent_id del grant no viene de la variable del payload: %s", args)
	}
}

// En pre-edit el agente NO se manda directo desde $agent_id, y la indirección es el punto:
// si el subagente cayó al fallback del marker de SESIÓN, el token que presenta es el del
// padre y no lleva agente. Mandar el suyo ahí devolvería agent_mismatch y dejaría sin editar
// justamente al subagente que el fallback existe para autorizar — el gate insatisfacible que
// empuja al bypass permanente (DOMAINSERV-111/175/195).
func TestHookPreEdit_ElAgenteSeMandaSoloSiElMarkerEsElPropio(t *testing.T) {
	hook := leerHook(t, "domain-pre-edit.sh")

	args := llamadaATool(t, hook, "domain_flow_validate_token")
	if strings.Contains(args, `\"$agent_id\"`) || strings.Contains(args, `\"${agent_id`) {
		t.Errorf("pre-edit manda $agent_id DIRECTO: un subagente bajo el marker de sesión del padre "+
			"presentaría un token sin agente declarándose agente, y el server lo denegaría por "+
			"agent_mismatch.\ninvocación: %s", args)
	}

	// la variable intermedia solo se llena cuando el marker en uso termina en el agente propio
	if !strings.Contains(hook, "agente_del_token") {
		t.Fatal("no existe la variable intermedia que decide si mandar el agente")
	}
	if !regexp.MustCompile(`case "\$marker" in\s*\n\s*\*"-\$\{agent_id`).MatchString(hook) {
		t.Error("el agente se manda sin comprobar que el marker en uso sea el propio del agente: " +
			"eso rompe el fallback al marker de sesión del incremento 2")
	}
}
