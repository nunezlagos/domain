package main

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

// DOMAINSERV-218: hasta acá había UN marker de flow por SESIÓN, así que N subagentes de un mismo
// flow compartían una sola allowlist. Con eso el criterio #2 del ticket —"un agente que intenta
// editar fuera de SU allowlist es denegado, incluso si el path está dentro de la allowlist de
// otro agente del mismo flow"— era imposible: no había nada que atribuyera una edición a un
// agente.
//
// El ticket lo había descartado porque el marker por agente "dependería de un campo indocumentado
// que se rompería sin aviso". Eso era falso: agent_id está documentado, y la doc de Claude Code
// dice textualmente "use this to distinguish subagent hook calls from main-thread calls". Lo que
// se hereda del padre es el session_id, no el agent_id.

func markerDeFlowDeAgente(home, session, agentID string) string {
	return markerDeFlow(home, session) + "-" + agentID
}

func payloadConAgente(t *testing.T, session, agentID, tool string, respuesta map[string]any) string {
	t.Helper()
	texto, err := json.Marshal(respuesta)
	if err != nil {
		t.Fatal(err)
	}
	campos := map[string]any{
		"session_id":      session,
		"hook_event_name": "PostToolUse",
		"tool_name":       tool,
		"tool_input":      map[string]any{},
		"tool_response":   []any{map[string]any{"type": "text", "text": string(texto)}},
	}
	if agentID != "" {
		campos["agent_id"] = agentID
	}
	p, err := json.Marshal(campos)
	if err != nil {
		t.Fatal(err)
	}
	return string(p)
}

// El caso que el ticket pedía: dos agentes del MISMO flow y de la MISMA sesión terminan con
// markers distintos, o sea con autorizaciones separables.
func TestPostOrchestrate_DosSubagentes_EscribenMarkersDistintos(t *testing.T) {
	home := t.TempDir()
	session := "sesion-compartida"

	correrHook(t, "domain-post-orchestrate.sh", home,
		payloadConAgente(t, session, "agente-A", "mcp__domain-mcp__domain_orchestrate",
			map[string]any{"flow_run_id": flowRunDePrueba}))
	correrHook(t, "domain-post-orchestrate.sh", home,
		payloadConAgente(t, session, "agente-B", "mcp__domain-mcp__domain_orchestrate",
			map[string]any{"flow_run_id": flowRunDePrueba}))

	for _, ag := range []string{"agente-A", "agente-B"} {
		if _, err := os.Stat(markerDeFlowDeAgente(home, session, ag)); err != nil {
			t.Errorf("%s no tiene marker propio: los dos agentes comparten autorización y el "+
				"criterio #2 del ticket es inalcanzable (%v)", ag, err)
		}
	}
}

// El hilo principal NO cambia de nombre: es lo que hace seguro este cambio, porque ese marker es
// el que autoriza las ediciones del agente que implementa esto.
func TestPostOrchestrate_HiloPrincipal_SigueUsandoElMarkerDeSesion(t *testing.T) {
	home := t.TempDir()
	session := "sesion-principal"

	// sin agent_id, que es exactamente lo que manda el cliente en el hilo principal
	correrHook(t, "domain-post-orchestrate.sh", home,
		payloadPostOrchestrate(t, session, "mcp__domain-mcp__domain_orchestrate",
			map[string]any{"flow_run_id": flowRunDePrueba}))

	if _, err := os.Stat(markerDeFlow(home, session)); err != nil {
		t.Fatalf("el hilo principal dejó de escribir flow-<session>: el gate que autoriza sus "+
			"ediciones se rompió (%v)", err)
	}
}

// El bug que este cambio cierra de paso: antes, un flow_cancel de CUALQUIERA borraba el único
// marker que había, así que un subagente que cancelaba su sub-flow le retiraba la autorización
// al padre y a todos sus hermanos.
func TestPostOrchestrate_CancelDeUnSubagente_NoLeBorraElMarkerAlPadre(t *testing.T) {
	home := t.TempDir()
	session := "sesion-con-hijos"

	escribirMarker(t, markerDeFlow(home, session), time.Now())
	escribirMarker(t, markerDeFlowDeAgente(home, session, "agente-A"), time.Now())

	correrHook(t, "domain-post-orchestrate.sh", home,
		payloadConAgente(t, session, "agente-A", "mcp__domain-mcp__domain_flow_cancel",
			map[string]any{"flow_run_id": flowRunDePrueba}))

	if _, err := os.Stat(markerDeFlowDeAgente(home, session, "agente-A")); !os.IsNotExist(err) {
		t.Error("el subagente canceló y su propio marker sobrevivió")
	}
	if _, err := os.Stat(markerDeFlow(home, session)); err != nil {
		t.Error("el cancel de un SUBAGENTE borró el marker del hilo principal: le retiró la " +
			"autorización de edición al padre y a todos sus hermanos")
	}
}

// La contracara, para que el fix no se pase de aislado: el cancel del hilo principal sigue
// borrando el suyo, que es el comportamiento que ya estaba testeado.
func TestPostOrchestrate_CancelDelHiloPrincipal_BorraElSuyo(t *testing.T) {
	home := t.TempDir()
	session := "sesion-sola"
	escribirMarker(t, markerDeFlow(home, session), time.Now())

	correrHook(t, "domain-post-orchestrate.sh", home,
		payloadPostOrchestrate(t, session, "mcp__domain-mcp__domain_flow_cancel",
			map[string]any{"flow_run_id": flowRunDePrueba}))

	if _, err := os.Stat(markerDeFlow(home, session)); !os.IsNotExist(err) {
		t.Error("cancelar el flow tiene que retirar la autorización de edición")
	}
}
