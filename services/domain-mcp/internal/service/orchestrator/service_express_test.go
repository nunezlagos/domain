package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/orchestrator/phases"
)

// fixedClock implementa Clock con un wall-clock determinista para tests.
type fixedClock struct{ t time.Time }

func (f fixedClock) Now() time.Time { return f.t }

// buildRegistryWithApplyAndVerify registra los dos handlers reales
// que el modo Express usa. Para tests más profundos otros tests pueden
// usar fakes.
func buildRegistryWithApplyAndVerify(t *testing.T) *phases.Registry {
	t.Helper()
	reg := phases.NewRegistry()
	reg.MustRegister(phases.NewSDDApplyHandler())
	reg.MustRegister(phases.NewSDDVerifyHandler())
	return reg
}

func TestService_Run_ExpressMode_BuildsApplyAndVerifyPlan(t *testing.T) {
	t.Parallel()
	clock := fixedClock{t: time.Date(2026, 6, 10, 14, 0, 0, 0, time.UTC)}
	s := New(nil, nil, buildRegistryWithApplyAndVerify(t), "dev")
	s.Clock = clock

	res, err := s.Run(context.Background(), OrchestrateInput{
		OrganizationID: uuid.New(),
		ProjectID:      uuid.New(),
		UserID:         uuid.New(),
		RawText:        "implementar typo fix en HelloWorld.go",
		Mode:           ModeExpress,
	})
	require.NoError(t, err)
	require.Equal(t, ModeExpress, res.Mode)
	require.NotNil(t, res.Plan)
	require.Equal(t, "express", res.Plan.Mode)
	require.Len(t, res.Plan.Steps, 2, "Express debe expandir sólo apply + verify")
	require.Equal(t, PhaseSlug("sdd-apply"), res.Plan.Steps[0].Slug)
	require.Equal(t, PhaseSlug("sdd-verify"), res.Plan.Steps[1].Slug)
	require.Equal(t, clock.t, res.StartedAt)
	require.NotEmpty(t, res.SnapshotPrompt, "SnapshotPrompt debe ser el prompt del primer step")
	require.Equal(t, res.Plan.Steps[0].UserPrompt, res.SnapshotPrompt)
}

// code_graph retirado (2026-07-07): apply declara code_reference como
// SuggestedSave OPCIONAL (Required=false), ya no lo exige.
func TestService_Run_ExpressMode_DeclaresOptionalCodeReferenceOnApply(t *testing.T) {
	t.Parallel()
	s := New(nil, nil, buildRegistryWithApplyAndVerify(t), "dev")
	res, err := s.Run(context.Background(), OrchestrateInput{
		OrganizationID: uuid.New(),
		ProjectID:      uuid.New(),
		UserID:         uuid.New(),
		RawText:        "anything",
		Mode:           ModeExpress,
	})
	require.NoError(t, err)
	apply := res.Plan.Steps[0]
	require.Len(t, apply.SuggestedSaves, 1)
	require.Equal(t, "code_reference", apply.SuggestedSaves[0].Type)
	require.False(t, apply.SuggestedSaves[0].Required)
}

func TestService_Run_ExpressMode_VerifySuggestedSavesAreOptional(t *testing.T) {
	t.Parallel()
	s := New(nil, nil, buildRegistryWithApplyAndVerify(t), "dev")
	res, err := s.Run(context.Background(), OrchestrateInput{
		OrganizationID: uuid.New(),
		ProjectID:      uuid.New(),
		UserID:         uuid.New(),
		RawText:        "anything",
		Mode:           ModeExpress,
	})
	require.NoError(t, err)
	verify := res.Plan.Steps[1]
	for _, s := range verify.SuggestedSaves {
		require.False(t, s.Required, "verify NO debe declarar required saves")
	}
}

func TestService_Run_ExpressMode_RegistryWithoutHandlers_Fails(t *testing.T) {
	t.Parallel()
	s := New(nil, nil, phases.NewRegistry(), "dev") // registry vacío
	_, err := s.Run(context.Background(), OrchestrateInput{
		OrganizationID: uuid.New(),
		ProjectID:      uuid.New(),
		UserID:         uuid.New(),
		RawText:        "x",
		Mode:           ModeExpress,
	})
	require.Error(t, err, "sin handlers el Express dispatcher debe fallar")
}

func TestService_Run_FullMode_WithoutAllHandlers_Fails(t *testing.T) {
	t.Parallel()

	s := New(nil, nil, buildRegistryWithApplyAndVerify(t), "dev")
	_, err := s.Run(context.Background(), OrchestrateInput{
		OrganizationID: uuid.New(),
		ProjectID:      uuid.New(),
		UserID:         uuid.New(),
		RawText:        "x",
		Mode:           ModeFull,
	})
	require.Error(t, err, "Full sin todos los handlers debe fallar al armar el plan")
}

// code_graph retirado (2026-07-07): apply ya no exige code_reference, así que
// terminar la fase sin guardarlo NO debe disparar ErrRequiredSaveMissing. El
// code_reference queda como SuggestedSave opcional.
func TestService_ApplyWithoutCodeReference_PassesAfterCodeGraphRetirement(t *testing.T) {
	t.Parallel()
	reg := buildRegistryWithApplyAndVerify(t)
	h, err := reg.Lookup(phases.PhaseSlug("sdd-apply"))
	require.NoError(t, err)
	out, err := h.Build(context.Background(), phases.Input{
		RawText: "any task",
	})
	require.NoError(t, err)

	clientResult := phases.ClientResult{
		Output: map[string]any{"summary": "looks good"},
	}
	err = ValidateRequiredSaves(phases.PhaseSlug("sdd-apply"), out, clientResult)
	require.NoError(t, err)
}
