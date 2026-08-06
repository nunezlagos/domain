package flow

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// DOMAINSERV-218 REQ-218.4 y REQ-218.5. Acá vive la decisión que el criterio 3 del ticket pide
// —rechazar allowlists solapadas al EMITIR— y la trampa que la haría inservible: si el chequeo
// no excluyera al agente que pide, su propia renovación lo bloquearía.

func TestSolapamientoConOtros_ScopesDisjuntos_NoHayConflicto(t *testing.T) {
	vigentes := []ScopeVigente{{AgentID: "agente-B", AllowedPaths: []string{"install-user/**"}}}

	err := SolapamientoConOtros("agente-A", []string{"services/domain-mcp/**"}, vigentes)

	require.NoError(t, err)
}

func TestSolapamientoConOtros_OtroAgenteReclamaElMismoTerritorio_Rechaza(t *testing.T) {
	vigentes := []ScopeVigente{{AgentID: "agente-B", AllowedPaths: []string{"services/domain-mcp/**"}}}

	err := SolapamientoConOtros("agente-A", []string{"services/domain-mcp/internal/**"}, vigentes)

	require.ErrorIs(t, err, ErrAllowlistSolapada)
	require.Contains(t, err.Error(), "agente-B", "el error tiene que nombrar al agente con el que se choca")
}

// EL TEST QUE PROTEGE EL DISEÑO. Cada cierre de fase re-emite el token, así que el agente pide
// su propio scope una y otra vez. Si el chequeo no lo excluyera, se bloquearía a sí mismo en la
// segunda fase de cualquier flow real.
func TestSolapamientoConOtros_ElMismoAgenteRenovandoSuScope_NoSeAutoBloquea(t *testing.T) {
	vigentes := []ScopeVigente{{AgentID: "agente-A", AllowedPaths: []string{"services/domain-mcp/**"}}}

	err := SolapamientoConOtros("agente-A", []string{"services/domain-mcp/**"}, vigentes)

	require.NoError(t, err, "el agente chocó con su propia fila: su renovación quedaría bloqueada")
}

func TestSolapamientoConOtros_ElMismoAgenteCambiandoSuScope_TampocoSeAutoBloquea(t *testing.T) {
	vigentes := []ScopeVigente{{AgentID: "agente-A", AllowedPaths: []string{"services/domain-mcp/**"}}}

	err := SolapamientoConOtros("agente-A", []string{"install-user/**"}, vigentes)

	require.NoError(t, err)
}

// El hilo principal es agent_id "" y es un agente más a estos efectos: su scope tampoco puede
// pisar el de un subagente.
func TestSolapamientoConOtros_HiloPrincipalContraSubagente_Rechaza(t *testing.T) {
	vigentes := []ScopeVigente{{AgentID: "agente-A", AllowedPaths: []string{"services/**"}}}

	err := SolapamientoConOtros("", []string{"services/domain-mcp/**"}, vigentes)

	require.ErrorIs(t, err, ErrAllowlistSolapada)
}

// Sin allowlist no hay territorio que reclamar: un grant sin scope no puede solaparse con nada,
// y negarlo dejaría sin token a todo flow que no use batch-mode.
func TestSolapamientoConOtros_SinAllowlist_NoRestringe(t *testing.T) {
	vigentes := []ScopeVigente{{AgentID: "agente-B", AllowedPaths: []string{"services/**"}}}

	require.NoError(t, SolapamientoConOtros("agente-A", nil, vigentes))
	require.NoError(t, SolapamientoConOtros("agente-A", []string{}, vigentes))
}

// Un vigente SIN allowlist tampoco reserva territorio: es un flow normal, no batch-mode.
func TestSolapamientoConOtros_ElVigenteNoTieneAllowlist_NoBloquea(t *testing.T) {
	vigentes := []ScopeVigente{{AgentID: "agente-B", AllowedPaths: nil}}

	require.NoError(t, SolapamientoConOtros("agente-A", []string{"services/**"}, vigentes))
}

func TestSolapamientoConOtros_GlobIndecidible_SeRechazaAntesDeComparar(t *testing.T) {
	err := SolapamientoConOtros("agente-A", []string{"**/*.go"}, nil)

	require.ErrorIs(t, err, ErrAllowlistSinPrefijo)
}
