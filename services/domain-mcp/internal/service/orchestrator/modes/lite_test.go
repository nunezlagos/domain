package modes

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/orchestrator/phases"
)

// buildRegistryLite registra los tres handlers reales que el modo Lite
// usa por default: explore, apply, verify.
func buildRegistryLite(t *testing.T) *phases.Registry {
	t.Helper()
	reg := phases.NewRegistry()
	reg.MustRegister(phases.NewSDDExploreHandler())
	reg.MustRegister(phases.NewSDDApplyHandler())
	reg.MustRegister(phases.NewSDDVerifyHandler())
	reg.MustRegister(phases.NewSDDArchiveHandler())
	return reg
}

func TestBuildLitePlan_RunsSubsetInOrder(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	plan, err := BuildLitePlan(context.Background(), buildRegistryLite(t),
		phases.Input{RawText: "fix typo en README"}, now)
	require.NoError(t, err)
	require.Equal(t, "lite", plan.Mode)
	require.Equal(t, now, plan.StartedAt)

	require.Len(t, plan.Steps, 4, "Lite corre explore→apply→verify→archive (DOMAINSERV-89)")
	require.Equal(t, phases.PhaseSlug("sdd-explore"), plan.Steps[0].Slug)
	require.Equal(t, phases.PhaseSlug("sdd-apply"), plan.Steps[1].Slug)
	require.Equal(t, phases.PhaseSlug("sdd-verify"), plan.Steps[2].Slug)
	require.Equal(t, phases.PhaseSlug("sdd-archive"), plan.Steps[3].Slug)
}

func TestBuildLitePlan_SkipsHeavyPhases(t *testing.T) {
	t.Parallel()
	plan, err := BuildLitePlan(context.Background(), buildRegistryLite(t),
		phases.Input{RawText: "refactor chico"}, time.Now())
	require.NoError(t, err)

	heavy := map[phases.PhaseSlug]struct{}{
		"sdd-spec": {}, "sdd-propose": {}, "sdd-design": {}, "sdd-tasks": {},
		"sdd-judge": {}, "sdd-onboard": {},
	}
	for _, st := range plan.Steps {
		_, isHeavy := heavy[st.Slug]
		require.False(t, isHeavy, "Lite no debe incluir la fase pesada %s", st.Slug)
	}
}

func TestBuildLitePlan_HydratesUserPromptEager(t *testing.T) {
	t.Parallel()
	plan, err := BuildLitePlan(context.Background(), buildRegistryLite(t),
		phases.Input{RawText: "fix de 1 línea"}, time.Now())
	require.NoError(t, err)


	for _, st := range plan.Steps {
		require.NotEmpty(t, st.UserPrompt,
			"Lite hidrata UserPrompt eager en cada step (fase %s)", st.Slug)
	}
}

func TestBuildLitePlan_NilRegistry_Fails(t *testing.T) {
	t.Parallel()
	_, err := BuildLitePlan(context.Background(), nil,
		phases.Input{RawText: "x"}, time.Now())
	require.Error(t, err)
}

func TestBuildLitePlan_MissingHandler_Fails(t *testing.T) {
	t.Parallel()

	reg := phases.NewRegistry()
	reg.MustRegister(phases.NewSDDApplyHandler())
	reg.MustRegister(phases.NewSDDVerifyHandler())
	_, err := BuildLitePlan(context.Background(), reg,
		phases.Input{RawText: "x"}, time.Now())
	require.Error(t, err, "sin sdd-explore Lite debe fallar al armar el plan")
}

// TestLitePhases_DefaultSet documenta el set default y sirve de guardia:
// si alguien cambia LitePhases, este test obliga a actualizarlo
// conscientemente (es el punto de tuneo declarado).
func TestLitePhases_DefaultSet(t *testing.T) {
	t.Parallel()
	require.Equal(t, []phases.PhaseSlug{
		phases.PhaseSlug("sdd-explore"),
		phases.PhaseSlug("sdd-apply"),
		phases.PhaseSlug("sdd-verify"),
		phases.PhaseSlug("sdd-archive"),
	}, LitePhases)
}
