package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/orchestrator/phases"
)

// buildRegistryLite registra los handlers que el modo Lite usa por
// default (explore + apply + verify), sin DB.
func buildRegistryLite(t *testing.T) *phases.Registry {
	t.Helper()
	reg := phases.NewRegistry()
	reg.MustRegister(phases.NewSDDExploreHandler())
	reg.MustRegister(phases.NewSDDApplyHandler())
	reg.MustRegister(phases.NewSDDVerifyHandler())
	return reg
}

func TestService_Run_LiteMode_BuildsSubsetPlan(t *testing.T) {
	t.Parallel()
	clock := fixedClock{t: time.Date(2026, 6, 21, 11, 0, 0, 0, time.UTC)}
	s := New(nil, nil, buildRegistryLite(t), "dev")
	s.Clock = clock

	res, err := s.Run(context.Background(), OrchestrateInput{
		OrganizationID: uuid.New(),
		ProjectID:      uuid.New(),
		UserID:         uuid.New(),
		RawText:        "fix typo trivial en docs",
		Mode:           ModeLite,
	})
	require.NoError(t, err)
	require.Equal(t, ModeLite, res.Mode)
	require.NotNil(t, res.Plan)
	require.Equal(t, "lite", res.Plan.Mode)

	require.Len(t, res.Plan.Steps, 3, "Lite debe expandir explore→apply→verify")
	require.Equal(t, PhaseSlug("sdd-explore"), res.Plan.Steps[0].Slug)
	require.Equal(t, PhaseSlug("sdd-apply"), res.Plan.Steps[1].Slug)
	require.Equal(t, PhaseSlug("sdd-verify"), res.Plan.Steps[2].Slug)
	require.Equal(t, clock.t, res.StartedAt)
	require.NotEmpty(t, res.SnapshotPrompt)
}

func TestService_Run_LiteMode_SkipsHeavyPhases(t *testing.T) {
	t.Parallel()
	s := New(nil, nil, buildRegistryLite(t), "dev")
	res, err := s.Run(context.Background(), OrchestrateInput{
		OrganizationID: uuid.New(),
		ProjectID:      uuid.New(),
		UserID:         uuid.New(),
		RawText:        "refactor chico",
		Mode:           ModeLite,
	})
	require.NoError(t, err)

	heavy := map[PhaseSlug]struct{}{
		"sdd-spec": {}, "sdd-propose": {}, "sdd-design": {}, "sdd-tasks": {},
		"sdd-judge": {}, "sdd-archive": {}, "sdd-onboard": {},
	}
	for _, st := range res.Plan.Steps {
		_, isHeavy := heavy[st.Slug]
		require.False(t, isHeavy, "Lite no debe incluir la fase pesada %s", st.Slug)
	}
}

// TestService_Run_LiteMode_DoesNotNeedFullRegistry confirma que Lite NO
// requiere los 10 handlers (a diferencia de Full): basta con los 3 del
// subset. Es el contraste con TestService_Run_FullMode_WithoutAllHandlers_Fails.
func TestService_Run_LiteMode_DoesNotNeedFullRegistry(t *testing.T) {
	t.Parallel()
	s := New(nil, nil, buildRegistryLite(t), "dev")
	_, err := s.Run(context.Background(), OrchestrateInput{
		OrganizationID: uuid.New(),
		ProjectID:      uuid.New(),
		UserID:         uuid.New(),
		RawText:        "x",
		Mode:           ModeLite,
	})
	require.NoError(t, err, "Lite sólo necesita explore+apply+verify en el registry")
}

func TestMode_Lite_IsValid(t *testing.T) {
	t.Parallel()
	require.True(t, ModeLite.IsValid(), "lite debe ser un modo válido")
	require.True(t, Mode("lite").IsValid())
}

// TestService_Run_DefaultMode_StaysFull garantiza que Lite es opt-in y no
// cambió el default: input sin Mode sigue resolviendo a Full (que falla
// acá por falta de los 10 handlers, probando que NO cayó en Lite).
func TestService_Run_DefaultMode_StaysFull(t *testing.T) {
	t.Parallel()
	s := New(nil, nil, buildRegistryLite(t), "dev")
	res, err := s.Run(context.Background(), OrchestrateInput{
		OrganizationID: uuid.New(),
		ProjectID:      uuid.New(),
		UserID:         uuid.New(),
		RawText:        "x",
		// Mode vacío → debe inferir Full, NO Lite.
	})
	// Full requiere los 10 handlers; con sólo 3 falla al armar el plan.
	// Eso prueba que el default sigue siendo Full y no se desvió a Lite.
	require.Error(t, err)
	require.Nil(t, res)
}
