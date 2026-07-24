package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/orchestrator/phases"
)

// buildRegistryWithApply registra sólo el handler que el modo Micro usa.
func buildRegistryWithApply(t *testing.T) *phases.Registry {
	t.Helper()
	reg := phases.NewRegistry()
	reg.MustRegister(phases.NewSDDApplyHandler())
	return reg
}

// TestService_Run_MicroMode_BuildsApplyOnlyPlan: micro corre SOLO sdd-apply,
// sin sdd-verify. Es el fast path para ediciones triviales sin lógica
// testeable; el commit-gate del cliente exime estos flows de tests.
func TestService_Run_MicroMode_BuildsApplyOnlyPlan(t *testing.T) {
	t.Parallel()
	clock := fixedClock{t: time.Date(2026, 7, 24, 14, 0, 0, 0, time.UTC)}
	s := New(nil, nil, buildRegistryWithApply(t), "dev")
	s.Clock = clock

	res, err := s.Run(context.Background(), OrchestrateInput{
		OrganizationID: uuid.New(),
		ProjectID:      uuid.New(),
		UserID:         uuid.New(),
		RawText:        "cambiar el texto del botón de login en el front",
		Mode:           ModeMicro,
	})
	require.NoError(t, err)
	require.Equal(t, ModeMicro, res.Mode)
	require.NotNil(t, res.Plan)
	require.Equal(t, "micro", res.Plan.Mode)
	require.Len(t, res.Plan.Steps, 1, "Micro debe expandir SÓLO sdd-apply (sin verify)")
	require.Equal(t, PhaseSlug("sdd-apply"), res.Plan.Steps[0].Slug)
	require.Equal(t, clock.t, res.StartedAt)
}

// TestMode_Micro_IsValid: el modo micro es reconocido como válido.
func TestMode_Micro_IsValid(t *testing.T) {
	t.Parallel()
	require.True(t, ModeMicro.IsValid(), "micro debe ser un modo válido")
	require.Equal(t, Mode("micro"), ModeMicro)
}
