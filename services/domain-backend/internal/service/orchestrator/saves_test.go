package orchestrator

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/orchestrator/phases"
)

func TestValidateRequiredSaves_NilOutput_IsNoOp(t *testing.T) {
	t.Parallel()
	require.NoError(t, ValidateRequiredSaves("sdd-apply", nil, phases.ClientResult{}))
}

func TestValidateRequiredSaves_NoRequiredSaves_AlwaysOK(t *testing.T) {
	t.Parallel()
	out := &phases.Output{SuggestedSaves: []phases.SuggestedSave{
		{Type: "knowledge_doc", Required: false, Hint: "optional"},
	}}
	require.NoError(t, ValidateRequiredSaves("sdd-onboard", out, phases.ClientResult{}))
}

func TestValidateRequiredSaves_AllRequiredPresent_OK(t *testing.T) {
	t.Parallel()
	out := &phases.Output{SuggestedSaves: []phases.SuggestedSave{
		{Type: "code_reference", Required: true},
		{Type: "knowledge_doc", Required: false},
	}}
	result := phases.ClientResult{
		MemoryRefsSaved: []phases.MemoryRef{
			{Type: "code_reference", ID: uuid.New()},
		},
	}
	require.NoError(t, ValidateRequiredSaves("sdd-apply", out, result))
}

func TestValidateRequiredSaves_MissingRequired_ReturnsError(t *testing.T) {
	t.Parallel()
	out := &phases.Output{SuggestedSaves: []phases.SuggestedSave{
		{Type: "adr", Required: true, Hint: "guardar ADR"},
		{Type: "code_reference", Required: true, Hint: "guardar code ref"},
	}}
	// Cliente sólo guardó uno de los dos requireds
	result := phases.ClientResult{
		MemoryRefsSaved: []phases.MemoryRef{
			{Type: "adr", ID: uuid.New()},
		},
	}
	err := ValidateRequiredSaves("sdd-design", out, result)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrRequiredSaveMissing)
	var rse *RequiredSaveError
	require.True(t, errors.As(err, &rse))
	require.Equal(t, phases.PhaseSlug("sdd-design"), rse.Phase)
	require.Len(t, rse.Missing, 1)
	require.Equal(t, "code_reference", rse.Missing[0].Type)
	require.Equal(t, "guardar code ref", rse.Missing[0].Hint)
}

// TestValidateRequiredSaves_Sabotage_RemoveRequiredFlag valida que el test
// no es "always green". Sabotaje: si flippeo el Required a false, la
// validación dejaría pasar el resultado y se perdería el contrato D5.
// Este test confirma que el flag Required es lo que dispara la regla.
func TestValidateRequiredSaves_Sabotage_RemoveRequiredFlag(t *testing.T) {
	t.Parallel()
	out := &phases.Output{SuggestedSaves: []phases.SuggestedSave{
		{Type: "sabotage_record", Required: false}, // SABOTAJE: flippeado
	}}
	result := phases.ClientResult{} // sin saves
	// Con Required=false debería pasar → si fallara, el código estaría
	// ignorando el flag y eso sería un bug peor
	require.NoError(t, ValidateRequiredSaves("sdd-judge", out, result))
}
