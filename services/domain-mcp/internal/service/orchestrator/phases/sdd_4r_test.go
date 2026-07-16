package phases

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSDD4RHandler_Slug_DevuelvePhase4R(t *testing.T) {
	require.Equal(t, PhaseSlug("sdd-4r"), NewSDD4RHandler().Slug())
}

func TestSDD4RHandler_Build_ConPriorOutputs_ArmaPlanYReviewTree(t *testing.T) {
	in := Input{
		RawText: "revisar el cambio de la fase sdd-4r",
		PriorOutputs: map[PhaseSlug]map[string]any{
			PhaseSlug("sdd-apply"):  {"files_changed": []any{"a.go", "b.go"}},
			PhaseSlug("sdd-verify"): {"summary": "3 scenarios passed"},
		},
	}
	out, err := NewSDD4RHandler().Build(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, "sdd-4r", out.AgentTemplateSlug)
	require.Equal(t, RetryReemit, out.RetryPolicy)
	for _, lens := range []string{"R1", "R2", "R3", "R4"} {
		require.Contains(t, out.SubagentPlan, lens, "el plan debe incluir la lens %s", lens)
	}
	require.Contains(t, out.SubagentPlan, "file-only")
	require.Contains(t, out.UserPrompt, "a.go")
	require.Contains(t, out.UserPrompt, "3 scenarios passed")
}

func TestSDD4RHandler_Build_RawTextVacio_DevuelveError(t *testing.T) {
	_, err := NewSDD4RHandler().Build(context.Background(), Input{})
	require.Error(t, err)
}

func TestSDD4RHandler_Validate_CuatroLenses_DevuelveNil(t *testing.T) {
	res := ClientResult{Output: map[string]any{"lens_reports": []any{
		map[string]any{"lens": "R1"}, map[string]any{"lens": "R2"},
		map[string]any{"lens": "R3"}, map[string]any{"lens": "R4"},
	}}}
	require.NoError(t, NewSDD4RHandler().Validate(context.Background(), nil, res))
}

func TestSDD4RHandler_Validate_MenosDeCuatroLenses_DevuelveError(t *testing.T) {
	res := ClientResult{Output: map[string]any{"lens_reports": []any{
		map[string]any{"lens": "R1"},
	}}}
	require.Error(t, NewSDD4RHandler().Validate(context.Background(), nil, res))
}

func TestSDD4RHandler_Validate_OutputNulo_DevuelveError(t *testing.T) {
	require.Error(t, NewSDD4RHandler().Validate(context.Background(), nil, ClientResult{}))
}
