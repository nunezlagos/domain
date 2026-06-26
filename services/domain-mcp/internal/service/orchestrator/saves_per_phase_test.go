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
//  1. Las fases con Required=true (sdd-propose, sdd-design, sdd-tasks,
//     sdd-apply, sdd-judge) declaran el tipo correcto en su SuggestedSaves
//  2. ValidateRequiredSaves bloquea sin la memory_ref correcta
//  3. ValidateRequiredSaves pasa con la memory_ref correcta
//  4. Las fases sin required (explore/spec/verify/archive/onboard)
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

	// Feature B: sdd-design declara 2 SuggestedSaves Required=true —
	// 'adr' (decisiones arquitectónicas) y 'knowledge_doc' (el design.md
	// como documento de primera clase, garantía de registro en BD).
	require.Len(t, out.SuggestedSaves, 2)
	requiredTypes := map[string]bool{}
	for _, s := range out.SuggestedSaves {
		require.True(t, s.Required, "ambos saves de sdd-design son Required")
		requiredTypes[s.Type] = true
	}
	require.True(t, requiredTypes["adr"])
	require.True(t, requiredTypes["knowledge_doc"])

	err = ValidateRequiredSaves(phases.PhaseSlug("sdd-design"), out, phases.ClientResult{})
	require.ErrorIs(t, err, ErrRequiredSaveMissing)

	// Sólo el adr (falta knowledge_doc) → sigue fallando
	err = ValidateRequiredSaves(phases.PhaseSlug("sdd-design"), out, phases.ClientResult{
		MemoryRefsSaved: []phases.MemoryRef{{Type: "adr", ID: uuid.New()}},
	})
	require.ErrorIs(t, err, ErrRequiredSaveMissing)

	// Ambos tipos requeridos → pasa
	err = ValidateRequiredSaves(phases.PhaseSlug("sdd-design"), out, phases.ClientResult{
		MemoryRefsSaved: []phases.MemoryRef{
			{Type: "adr", ID: uuid.New()},
			{Type: "knowledge_doc", ID: uuid.New()},
		},
	})
	require.NoError(t, err)

	err = ValidateRequiredSaves(phases.PhaseSlug("sdd-design"), out, phases.ClientResult{
		MemoryRefsSaved: []phases.MemoryRef{{Type: "code_reference", ID: uuid.New()}},
	})
	require.ErrorIs(t, err, ErrRequiredSaveMissing)
}

// TestD5Contract_DocPhases_RequireKnowledgeDoc — Feature B: las fases que
// generan un DOCUMENTO de primera clase (propose/design/tasks) exigen un
// knowledge_doc Required=true para garantizar su registro en BD.
func TestD5Contract_DocPhases_RequireKnowledgeDoc(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		factory func() phases.Handler
	}{
		{"sdd-propose", phases.NewSDDProposeHandler},
		{"sdd-design", phases.NewSDDDesignHandler},
		{"sdd-tasks", phases.NewSDDTasksHandler},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := tc.factory()
			out, err := h.Build(context.Background(), phases.Input{RawText: "x"})
			require.NoError(t, err)

			var hasKnowledgeDoc bool
			for _, s := range out.SuggestedSaves {
				if s.Type == "knowledge_doc" {
					require.True(t, s.Required,
						"%s: el knowledge_doc debe ser Required=true (Feature B)", tc.name)
					hasKnowledgeDoc = true
				}
			}
			require.True(t, hasKnowledgeDoc,
				"%s declara un SuggestedSave knowledge_doc", tc.name)

			// Sin el knowledge_doc → la fase no avanza
			require.ErrorIs(t,
				ValidateRequiredSaves(phases.PhaseSlug(tc.name), out, phases.ClientResult{}),
				ErrRequiredSaveMissing)
		})
	}
}

func TestD5Contract_SDDApply_RequiresCodeReference(t *testing.T) {
	t.Parallel()
	h := phases.NewSDDApplyHandler()
	out, err := h.Build(context.Background(), phases.Input{RawText: "implement x"})
	require.NoError(t, err)

	require.Len(t, out.SuggestedSaves, 1)
	require.Equal(t, "code_reference", out.SuggestedSaves[0].Type)
	require.True(t, out.SuggestedSaves[0].Required)

	require.ErrorIs(t,
		ValidateRequiredSaves(phases.PhaseSlug("sdd-apply"), out, phases.ClientResult{}),
		ErrRequiredSaveMissing)

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

	require.ErrorIs(t,
		ValidateRequiredSaves(phases.PhaseSlug("sdd-judge"), out, phases.ClientResult{}),
		ErrRequiredSaveMissing)

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

			for _, s := range out.SuggestedSaves {
				require.Falsef(t, s.Required,
					"fase %s no debe declarar SuggestedSave Required=true", tc.name)
			}

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
