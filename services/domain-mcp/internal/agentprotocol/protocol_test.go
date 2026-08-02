package agentprotocol

import "testing"

// El protocolo debe declarar el sync SDD OBLIGATORIO (REQ-55): sin esta directiva
// el auto-sync de openspec deja de ser determinístico. Guarda contra regresiones.
func TestFull_ContainsMandatorySDDSyncDirective(t *testing.T) {
	must := []string{
		"SYNC AUTOMÁTICO DEL SDD",
		"domain_openspec_export",
		"domain_issue_create_commit",
		"domain_orchestrate_phase_result",
		"domain_verify_complete",
		"domain_issue_set_status(archived)",
		"changes/archive/",
	}
	for _, s := range must {
		if !contains(Full, s) {
			t.Errorf("agent-protocol Full no contiene la directiva de sync SDD: falta %q", s)
		}
	}
}

// DOMAINSERV-158: el server emite planes de fan-out en cada corrida y el protocolo de
// ejecución paralela era criterio del hilo en el momento. Vive en Full —y no en una
// policy aparte— porque quien hace fan-out es el hilo orquestador, que SIEMPRE recibió
// Full como ServerInstructions del handshake; un subagente es hoja y no reparte.
func TestFull_FanOut_DeclaraLoteoAgregacionYRelanzamiento(t *testing.T) {
	must := []string{
		"sin estado compartido",
		"no se resuelven por mayoría",
		"orden de finalización",
		"se relanzan esos 2",
		"degradado:",
		"truncado:",
	}
	for _, s := range must {
		if !contains(Full, s) {
			t.Errorf("agent-protocol Full no declara la doctrina de fan-out: falta %q", s)
		}
	}
}

// Full es ServerInstructions (internal/mcp/server/server.go): se paga en el handshake de
// CADA sesión de CADA cliente. En el rulesBlock, en cambio, no se paga: supera el
// maxInlinePolicyBody=4000 del orchestrator y llega stubbeada como puntero a
// domain_policy_get. Ese es el criterio de reparto de DOMAINSERV-220/158: la doctrina que
// necesita el hilo orquestador entra acá (gratis en el rulesBlock); la que necesita un
// subagente que escribe SQL va como platform policy chica, que sí viaja inline.
func TestFull_Tamano_SuperaElCapInlineYRespetaElTechoDelHandshake(t *testing.T) {
	const capInlineDelOrchestrator = 4000
	// 18.909 medidos + ~600 de margen: alcanza para reescribir una frase, no para meter
	// otra sección (la doctrina de fan-out costó 1.400 bytes). El guard apunta a la
	// sección nueva sin decisión, no al typo
	const techoDelHandshake = 19500

	if len(Full) <= capInlineDelOrchestrator {
		t.Errorf("Full mide %d: dejó de stubbearse en el rulesBlock y ahora consume "+
			"presupuesto compartido con las reglas duras", len(Full))
	}
	if len(Full) > techoDelHandshake {
		t.Errorf("Full mide %d > %d: cada byte se paga en el handshake de toda sesión. "+
			"Lo nuevo va como platform policy chica (<4000, viaja inline en el rulesBlock), "+
			"no acá", len(Full), techoDelHandshake)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
