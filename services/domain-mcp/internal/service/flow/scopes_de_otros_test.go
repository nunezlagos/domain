package flow

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// DOMAINSERV-218, incremento 5. MEDIDO instrumentando el hook (2026-08-06): un subagente sin
// marker propio cae al marker del PADRE por el fallback del incremento 2, y el token del padre
// no tiene allowed_paths — así que pre-edit toma la rama "sin allowlist → sin restricción" y
// sale por exit 0. El fallback funciona como fue diseñado, y por eso mismo le entrega al
// subagente la autorización amplia del hilo principal.
//
// Ahí muere el aislamiento: no importa cuántos scopes se declaren, cualquier subagente puede
// editar todo heredando el token del padre.
//
// La corrección NO puede ser "denegar a todo subagente sin token propio": eso volvería el gate
// insatisfacible en cada flow normal y empuja al bypass permanente (DOMAINSERV-111/175/195). Se
// acota a los flows donde el aislamiento está EN JUEGO: si hay scopes vigentes de otros agentes,
// heredar la autorización amplia contradice la partición que alguien declaró a propósito.

func TestHayScopesDeOtros_FlowSinScopes_EsFalse(t *testing.T) {
	require.False(t, HayScopesDeOtros("agente-A", nil))
	require.False(t, HayScopesDeOtros("agente-A", []ScopeVigente{}))
}

func TestHayScopesDeOtros_SoloElPropio_EsFalse(t *testing.T) {
	vigentes := []ScopeVigente{{AgentID: "agente-A", AllowedPaths: []string{"docs/**"}}}

	require.False(t, HayScopesDeOtros("agente-A", vigentes),
		"el scope propio no puede restringirse a sí mismo: sería auto-bloqueo")
}

func TestHayScopesDeOtros_OtroAgenteConScope_EsTrue(t *testing.T) {
	vigentes := []ScopeVigente{{AgentID: "agente-B", AllowedPaths: []string{"docs/**"}}}

	require.True(t, HayScopesDeOtros("agente-A", vigentes),
		"hay una partición declarada por otro agente: heredar la autorización amplia del padre la contradice")
}

// Una fila sin allowlist no declara partición alguna, así que no restringe a nadie.
func TestHayScopesDeOtros_OtroAgenteSinAllowlist_EsFalse(t *testing.T) {
	vigentes := []ScopeVigente{{AgentID: "agente-B", AllowedPaths: nil}}

	require.False(t, HayScopesDeOtros("agente-A", vigentes))
}

// El hilo principal es agent_id "": si un subagente declaró scope, el principal tampoco debería
// asumir que su token amplio sigue siendo la última palabra.
func TestHayScopesDeOtros_ConsultandoDesdeElHiloPrincipal_VeLosScopesDeLosSubagentes(t *testing.T) {
	vigentes := []ScopeVigente{{AgentID: "agente-A", AllowedPaths: []string{"docs/**"}}}

	require.True(t, HayScopesDeOtros("", vigentes))
}
