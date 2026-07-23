package modes

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/orchestrator/phases"
)

// DOMAINSERV-89: Lite incluye sdd-archive (última fase) para cerrar el ciclo de
// openspec; Express queda afuera (set cerrado por RFC).
func TestLitePhases_IncluyeArchiveAlFinal(t *testing.T) {
	require.NotEmpty(t, LitePhases)
	assert.Equal(t, phases.PhaseSlug("sdd-archive"), LitePhases[len(LitePhases)-1],
		"sdd-archive debe ser la última fase de Lite")
	assert.Contains(t, LitePhases, phases.PhaseSlug("sdd-explore"))
	assert.Contains(t, LitePhases, phases.PhaseSlug("sdd-verify"))
}

func TestExpressPhases_NoIncluyeArchive_RFCLocked(t *testing.T) {
	assert.NotContains(t, ExpressPhases, phases.PhaseSlug("sdd-archive"),
		"Express no archiva: su set está cerrado por RFC (requiere ADR para cambiarlo)")
}
