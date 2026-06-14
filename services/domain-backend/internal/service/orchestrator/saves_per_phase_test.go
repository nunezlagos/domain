package orchestrator

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/orchestrator/phases"
)

// save-003 — Tests unit del contract D5 (RFC 0006) por cada fase.
//
// Verifica que:
//  1. Las fases con Required=true (sdd-design, sdd-apply, sdd-judge)
//     declaran el tipo correcto en su SuggestedSaves
//  2. ValidateRequiredSaves bloquea sin la memory_ref correcta
//  3. ValidateRequiredSaves pasa con la memory_ref correcta
//  4. Las fases sin required (explore/spec/propose/tasks/verify/archive/onboard)
//     no bloquean independientemente de los memory_refs reportados

func TestD5Contract_SDDDesign_RequiresADR(t *testing.T) {
	t.Parallel()
	h := phases.NewSDDDesignHandler()
	out, err := h.Build(context.Background(), phases.Input{
		RawText: "design x",
		PriorOutputs: map[phases.PhaseSlug]map[string]any{
			phases.PhaseSlug("sdd-spec"): {"issue_md": "spec"},
		},
	})
	require.NoError(t, err)

	// El handler declara EXACTAMENTE 1 SuggestedSave de tipo 'adr' Required=true
	require.Len(t, out.SuggestedSaves, 1)
	require.Equal(t, "adr", out.SuggestedSaves[0].Type)
	require.True(t, out.SuggestedSaves[0].Required)

	// Sin memref → falla
	err = ValidateRequiredSaves(phases.PhaseSlug("sdd-design"), out, phases.ClientResult{})
	require.ErrorIs(t, err, ErrRequiredSaveMissing)

	// Con memref del tipo correcto → pasa
	err = ValidateRequiredSaves(phases.PhaseSlug("sdd-design"), out, phases.ClientResult{
		MemoryRefsSaved: []phases.MemoryRef{{Type: "adr", ID: uuid.New()}},
	})
	require.NoError(t, err)

	// Con memref de TIPO INCORRECTO → falla (D5 chequea por type)
	err = ValidateRequiredSaves(phases.PhaseSlug("sdd-design"), out, phases.ClientResult{
		MemoryRefsSaved: []phases.MemoryRef{{Type: "code_reference", ID: uuid.New()}},
	})
	require.ErrorIs(t, err, ErrRequiredSaveMissing)
}

func TestD5Contract_SDDApply_RequiresCodeReference(t *testing.T) {
	t.Parallel()
	h := phases.NewSDDApplyHandler()
	out, err := h.Build(context.Background(), phases.Input{RawText: "implement x"})
	require.NoError(t, err)

	require.Len(t, out.SuggestedSaves, 1)
	require.Equal(t, "code_reference", out.SuggestedSaves[0].Type)
	require.True(t, out.SuggestedSaves[0].Required)

	// Sin memref → falla
	require.ErrorIs(t,
		ValidateRequiredSaves(phases.PhaseSlug("sdd-apply"), out, phases.ClientResult{}),
		ErrRequiredSaveMissing)

	// Con memref correcto → pasa
	require.NoError(t,
		ValidateRequiredSaves(phases.PhaseSlug("sdd-apply"), out, phases.ClientResult{
			MemoryRefsSaved: []phases.MemoryRef{{Type: "code_reference", ID: uuid.New()}},
		}))
}

func TestD5Contract_SDDJudge_RequiresSabotageRecord(t *testing.T) {
	t.Parallel()
	h := phases.NewSDDJudgeHandler()
	out, err := h.Build(context.Background(), phases.Input{RawText: "judge x"})
	require.NoError(t, err)

	require.Len(t, out.SuggestedSaves, 1)
	require.Equal(t, "sabotage_record", out.SuggestedSaves[0].Type)
	require.True(t, out.SuggestedSaves[0].Required)

	// Sin memref → falla
	require.ErrorIs(t,
		ValidateRequiredSaves(phases.PhaseSlug("sdd-judge"), out, phases.ClientResult{}),
		ErrRequiredSaveMissing)

	// Con memref correcto → pasa
	require.NoError(t,
		ValidateRequiredSaves(phases.PhaseSlug("sdd-judge"), out, phases.ClientResult{
			MemoryRefsSaved: []phases.MemoryRef{{Type: "sabotage_record", ID: uuid.New()}},
		}))
}

// Las fases sin required pasan SIEMPRE, incluso sin memory_refs.
func TestD5Contract_PhasesWithoutRequired_AlwaysPass(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		factory func() phases.Handler
		input   phases.Input
	}{
		{"sdd-explore", phases.NewSDDExploreHandler, phases.Input{RawText: "x"}},
		{"sdd-spec", phases.NewSDDSpecHandler, phases.Input{RawText: "x"}},
		{"sdd-propose", phases.NewSDDProposeHandler, phases.Input{RawText: "x"}},
		{"sdd-tasks", phases.NewSDDTasksHandler, phases.Input{RawText: "x"}},
		{"sdd-verify", phases.NewSDDVerifyHandler, phases.Input{RawText: "x"}},
		{"sdd-archive", phases.NewSDDArchiveHandler, phases.Input{RawText: "x"}},
		{"sdd-onboard", phases.NewSDDOnboardHandler, phases.Input{RawText: "x"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := tc.factory()
			out, err := h.Build(context.Background(), tc.input)
			require.NoError(t, err)
			// Cero suggested_saves Required=true
			for _, s := range out.SuggestedSaves {
				require.Falsef(t, s.Required,
					"fase %s no debe declarar SuggestedSave Required=true", tc.name)
			}
			// ValidateRequiredSaves pasa sin memory_refs
			require.NoError(t,
				ValidateRequiredSaves(phases.PhaseSlug(tc.name), out, phases.ClientResult{}))
		})
	}
}

// El RequiredSaveError reporta los tipos faltantes específicos cuando
// múltiples Required no se cumplen (caso teórico — actualmente ningún
// handler declara 2 Required pero el código está preparado para ello).
func TestD5Contract_MultipleRequiredMissing_AllReported(t *testing.T) {
	t.Parallel()
	out := &phases.Output{
		SuggestedSaves: []phases.SuggestedSave{
			{Type: "adr", Required: true, Hint: "save the ADR"},
			{Type: "code_reference", Required: true, Hint: "save the code"},
			{Type: "knowledge_doc", Required: false}, // este NO cuenta
		},
	}
	err := ValidateRequiredSaves("sdd-fake", out, phases.ClientResult{})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrRequiredSaveMissing)

	var rse *RequiredSaveError
	require.ErrorAs(t, err, &rse)
	require.Len(t, rse.Missing, 2, "ambos required faltantes deben estar en el error")
	types := map[string]bool{}
	for _, m := range rse.Missing {
		types[m.Type] = true
	}
	require.True(t, types["adr"])
	require.True(t, types["code_reference"])
	require.False(t, types["knowledge_doc"], "los no-required NO se reportan como missing")
}
