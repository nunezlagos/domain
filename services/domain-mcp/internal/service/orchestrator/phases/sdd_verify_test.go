package phases

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSDDVerifyHandler_Slug(t *testing.T) {
	t.Parallel()
	require.Equal(t, PhaseSlug("sdd-verify"), NewSDDVerifyHandler().Slug())
}

func TestSDDVerifyHandler_Build_WithoutApplyOutput_BuildsGenericPrompt(t *testing.T) {
	t.Parallel()



	h := NewSDDVerifyHandler()
	out, err := h.Build(context.Background(), Input{RawText: "fix bug"})
	require.NoError(t, err)
	require.Contains(t, out.UserPrompt, "sdd-apply")
}

func TestSDDVerifyHandler_Build_RejectsEmptyRawText(t *testing.T) {
	t.Parallel()
	h := NewSDDVerifyHandler()
	_, err := h.Build(context.Background(), Input{
		PriorOutputs: map[PhaseSlug]map[string]any{PhaseSlug("sdd-apply"): {}},
	})
	require.Error(t, err)
}

func TestSDDVerifyHandler_Build_IncorporatesApplySummaryAndFiles(t *testing.T) {
	t.Parallel()
	h := NewSDDVerifyHandler()
	in := Input{
		RawText: "fix bug Y",
		PriorOutputs: map[PhaseSlug]map[string]any{
			PhaseSlug("sdd-apply"): {
				"summary":       "agregué retry en el cliente HTTP",
				"files_changed": []any{"internal/http/client.go"},
			},
		},
	}
	out, err := h.Build(context.Background(), in)
	require.NoError(t, err)
	require.Contains(t, out.UserPrompt, "retry en el cliente HTTP")
	require.Contains(t, out.UserPrompt, "internal/http/client.go")
	require.Equal(t, RetryReemit, out.RetryPolicy)

	for _, s := range out.SuggestedSaves {
		require.False(t, s.Required, "verify suggested_saves no deben ser required")
	}
}

func TestSDDVerifyHandler_Validate_BlockerReportedAsBlocked(t *testing.T) {
	t.Parallel()
	h := NewSDDVerifyHandler()
	err := h.Validate(context.Background(), nil, ClientResult{
		Output: map[string]any{
			"blockers":         []any{map[string]any{"question": "..."}},
			"scenarios_failed": []any{},
		},
	})
	require.ErrorIs(t, err, ErrPhaseBlockedByClient)
}

func TestSDDVerifyHandler_Validate_FailedScenariosReportedAsFail(t *testing.T) {
	t.Parallel()
	h := NewSDDVerifyHandler()
	err := h.Validate(context.Background(), nil, ClientResult{
		Output: map[string]any{
			"scenarios_failed": []any{"Escenario 1: ..."},
		},
	})
	require.ErrorIs(t, err, ErrVerificationFailed)
}

func TestSDDVerifyHandler_Validate_HappyPath(t *testing.T) {
	t.Parallel()
	h := NewSDDVerifyHandler()
	err := h.Validate(context.Background(), nil, ClientResult{
		Output: map[string]any{
			"scenarios_failed": []any{},
			"tests_failed":     0,
		},
	})
	require.NoError(t, err)
}
