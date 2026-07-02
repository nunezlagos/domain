package orchestrator

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/orchestrator/modes"
	"nunezlagos/domain/internal/service/orchestrator/phases"
)

// REQ-54 issue-54.5: el plan de subagentes fluye handler → PhaseStep, el
// override del template gana, y las fases sin plan quedan intactas.

func TestSubagentPlan_FlowsFromHandlerToPhaseStep(t *testing.T) {
	t.Parallel()
	reg := phases.NewRegistry()
	reg.MustRegister(phases.NewSDDApplyHandler())
	reg.MustRegister(phases.NewSDDVerifyHandler())

	plan, err := modes.BuildExpressPlan(context.Background(), reg,
		phases.Input{RawText: "implementar contrato de prueba"},
		time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Len(t, plan.Steps, 2)

	byUn := map[phases.PhaseSlug]modes.PhaseStep{}
	for _, st := range plan.Steps {
		byUn[st.Slug] = st
	}
	// verify es piloto: su plan declara validación de escenarios en paralelo.
	require.Contains(t, byUn[phases.PhaseSlug("sdd-verify")].SubagentPlan, "paralelo")
	// apply NO tiene plan (fase monolítica deliberada): no-op.
	require.Empty(t, byUn[phases.PhaseSlug("sdd-apply")].SubagentPlan)
}

func TestSubagentPlan_TemplateOverride_Wins(t *testing.T) {
	t.Parallel()
	tmpl := &AgentTemplate{
		Slug:     "sdd-verify",
		Metadata: map[string]any{"subagent_plan": "PLAN CUSTOM DE BD"},
	}
	require.Equal(t, "PLAN CUSTOM DE BD", tmpl.SubagentPlan())
	// Sin metadata → sin override.
	require.Empty(t, (&AgentTemplate{Slug: "x"}).SubagentPlan())
}

func TestInjectSubagentPlan_PrependsBlock(t *testing.T) {
	t.Parallel()
	out := injectSubagentPlan("PROMPT ORIGINAL", "- juez A\n- juez B")
	require.True(t, strings.HasPrefix(out, "## Plan de subagentes"))
	require.Contains(t, out, "PARALELO")
	require.Less(t, strings.Index(out, "juez A"), strings.Index(out, "PROMPT ORIGINAL"),
		"el plan va ANTES del prompt original")
}

// El orden final de composición en los puntos de inyección es
// prep → plan → prompt original.
func TestInjectionOrder_PrepThenPlanThenPrompt(t *testing.T) {
	t.Parallel()
	composed := injectPreparedContext(injectSubagentPlan("ORIGINAL", "EL-PLAN"), "EL-CONTEXTO")
	iCtx := strings.Index(composed, "EL-CONTEXTO")
	iPlan := strings.Index(composed, "EL-PLAN")
	iOrig := strings.Index(composed, "ORIGINAL")
	require.True(t, iCtx < iPlan && iPlan < iOrig,
		"orden esperado prep(%d) < plan(%d) < original(%d)", iCtx, iPlan, iOrig)
}
