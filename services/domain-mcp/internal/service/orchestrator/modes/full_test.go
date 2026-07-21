package modes

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/orchestrator/phases"
)

// stubFullHandler declara un contrato estático (RequiredToolCalls/OutputSchema/
// SubagentPlan) por fase, como los handlers SDD reales.
type stubFullHandler struct{ slug phases.PhaseSlug }

func (s stubFullHandler) Slug() phases.PhaseSlug { return s.slug }

func (s stubFullHandler) Build(context.Context, phases.Input) (*phases.Output, error) {
	return &phases.Output{
		AgentTemplateSlug: string(s.slug),
		UserPrompt:        "up-" + string(s.slug),
		RequiredToolCalls: []string{"tool_" + string(s.slug)},
		OutputSchema:      map[string]any{"type": "object"},
		SubagentPlan:      "plan-" + string(s.slug),
	}, nil
}

func (s stubFullHandler) Validate(context.Context, *phases.Output, phases.ClientResult) error {
	return nil
}

// DOMAINSERV-53: el branch else de BuildFullPlan (steps 2..N) debe copiar el
// contrato de la fase (RequiredToolCalls/OutputSchema/SubagentPlan), no solo el
// step 0. Sin el fix, esas fases cierran en Full sin exigir sus tools.
func TestBuildFullPlan_Steps2ToN_CarryToolContract(t *testing.T) {
	reg := phases.NewRegistry()
	for _, slug := range FullPhases {
		reg.MustRegister(stubFullHandler{slug: slug})
	}

	plan, err := BuildFullPlan(context.Background(), reg, phases.Input{RawText: "x"}, "", nil, time.Now())
	require.NoError(t, err)
	require.Len(t, plan.Steps, len(FullPhases))

	require.NotEmpty(t, plan.Steps[0].RequiredToolCalls, "step 0 debe llevar contrato")
	for i := 1; i < len(plan.Steps); i++ {
		require.NotEmpty(t, plan.Steps[i].RequiredToolCalls, "step %d (%s) sin RequiredToolCalls", i, plan.Steps[i].Slug)
		require.NotNil(t, plan.Steps[i].OutputSchema, "step %d (%s) sin OutputSchema", i, plan.Steps[i].Slug)
		require.NotEmpty(t, plan.Steps[i].SubagentPlan, "step %d (%s) sin SubagentPlan", i, plan.Steps[i].Slug)
	}
}
