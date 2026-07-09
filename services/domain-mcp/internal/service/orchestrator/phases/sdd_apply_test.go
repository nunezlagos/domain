package phases

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSDDApplyHandler_Slug(t *testing.T) {
	t.Parallel()
	h := NewSDDApplyHandler()
	require.Equal(t, PhaseSlug("sdd-apply"), h.Slug())
}

func TestSDDApplyHandler_Build_RejectsEmptyRawText(t *testing.T) {
	t.Parallel()
	h := NewSDDApplyHandler()
	_, err := h.Build(context.Background(), Input{})
	require.Error(t, err)
}

func TestSDDApplyHandler_Build_DeclaresOptionalCodeReference(t *testing.T) {
	t.Parallel()
	h := NewSDDApplyHandler()
	out, err := h.Build(context.Background(), Input{RawText: "refactor X"})
	require.NoError(t, err)
	require.Equal(t, "sdd-apply", out.AgentTemplateSlug)

	require.Empty(t, out.SystemPrompt)
	require.Contains(t, out.UserPrompt, "refactor X")
	require.Len(t, out.SuggestedSaves, 1)
	require.Equal(t, "code_reference", out.SuggestedSaves[0].Type)
	// code_graph retirado (2026-07-07): code_reference ya no se produce, deja de ser required.
	require.False(t, out.SuggestedSaves[0].Required, "code_reference es opcional tras el retiro del code_graph")
	require.Equal(t, RetryCleanup, out.RetryPolicy)
}

func TestSDDApplyHandler_Build_IncludesPriorTasksWhenPresent(t *testing.T) {
	t.Parallel()
	h := NewSDDApplyHandler()
	in := Input{
		RawText: "implement feature foo",
		PriorOutputs: map[PhaseSlug]map[string]any{
			PhaseSlug("sdd-tasks"): {
				"tasks": []any{"escribir test", "implementar service", "refactor"},
			},
		},
	}
	out, err := h.Build(context.Background(), in)
	require.NoError(t, err)
	require.Contains(t, out.UserPrompt, "escribir test")
	require.Contains(t, out.UserPrompt, "implementar service")
}

func TestSDDApplyHandler_Validate_RejectsNilOutput(t *testing.T) {
	t.Parallel()
	h := NewSDDApplyHandler()
	err := h.Validate(context.Background(), nil, ClientResult{})
	require.Error(t, err)
}

func TestSDDApplyHandler_Validate_MultiConcernReportedAsError(t *testing.T) {
	t.Parallel()
	h := NewSDDApplyHandler()
	err := h.Validate(context.Background(), nil, ClientResult{
		Output: map[string]any{"multi_concern": true},
	})
	require.ErrorIs(t, err, ErrMultiConcernDetected)
}

func TestSDDApplyHandler_Validate_BlockedReportedAsError(t *testing.T) {
	t.Parallel()
	h := NewSDDApplyHandler()
	err := h.Validate(context.Background(), nil, ClientResult{
		Output: map[string]any{"blocked": true},
	})
	require.ErrorIs(t, err, ErrPhaseBlockedByClient)
}

func TestSDDApplyHandler_Validate_HappyPath(t *testing.T) {
	t.Parallel()
	h := NewSDDApplyHandler()
	err := h.Validate(context.Background(), nil, ClientResult{
		Output: map[string]any{"summary": "done"},
	})
	require.NoError(t, err)
}

// Sabotaje sab-003: cliente reporta phase_result SIN guardar el
// code_reference required. ValidateRequiredSaves del orquestador
// (centralizado en service/orchestrator/saves.go) lo debe atrapar.
// Acá replicamos el assert pero usando el contrato del handler para
// confirmar que el SuggestedSave que el handler declara es el mismo
// type que el orchestrator espera ver.
func TestSDDApplyHandler_Sabotage_SuggestedSaveTypeIsCodeReference(t *testing.T) {
	t.Parallel()
	h := NewSDDApplyHandler()
	out, err := h.Build(context.Background(), Input{RawText: "x"})
	require.NoError(t, err)

	require.Equal(t, "code_reference", out.SuggestedSaves[0].Type)
}

// Test sanity: asegurar que el handler no devuelve nuevos errores
// silenciosamente cambiando ErrMultiConcernDetected vs blocked
// (importante porque el dispatcher Full/Express los rutea distinto).
func TestSDDApplyHandler_ErrorsAreDistinct(t *testing.T) {
	t.Parallel()
	require.False(t, errors.Is(ErrMultiConcernDetected, ErrPhaseBlockedByClient))
}
