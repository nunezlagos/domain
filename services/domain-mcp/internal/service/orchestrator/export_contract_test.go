package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/orchestrator/modes"
	"nunezlagos/domain/internal/service/orchestrator/phases"
)

// R5-A: exportPlan debe exponer el contrato (required_tool_calls + output_schema)
// de cada fase en el PhaseStepSummary, para que el cliente lo conozca upfront.

func TestExportPlan_IncluyeRequiredToolCalls(t *testing.T) {
	plan := &modes.PhasePlan{
		Mode: "full",
		Steps: []modes.PhaseStep{
			{
				Slug:              phases.PhaseSlug("sdd-verify"),
				RequiredToolCalls: []string{"domain_verify_start", "domain_verify_complete"},
			},
		},
	}
	out := exportPlan(plan)
	require.NotNil(t, out)
	require.Len(t, out.Steps, 1)
	assert.Equal(t, []string{"domain_verify_start", "domain_verify_complete"}, out.Steps[0].RequiredToolCalls)
}

func TestExportPlan_IncluyeOutputSchema(t *testing.T) {
	schema := map[string]any{
		"type":     "object",
		"required": []any{"issue_slug", "issue_md"},
	}
	plan := &modes.PhasePlan{
		Mode: "full",
		Steps: []modes.PhaseStep{
			{Slug: phases.PhaseSlug("sdd-spec"), OutputSchema: schema},
		},
	}
	out := exportPlan(plan)
	require.Len(t, out.Steps, 1)
	assert.Equal(t, schema, out.Steps[0].OutputSchema)
}

func TestExportPlan_FaseSinContrato_NoRompe(t *testing.T) {
	plan := &modes.PhasePlan{
		Mode:  "full",
		Steps: []modes.PhaseStep{{Slug: phases.PhaseSlug("sdd-archive")}},
	}
	out := exportPlan(plan)
	require.Len(t, out.Steps, 1)
	assert.Empty(t, out.Steps[0].RequiredToolCalls)
	assert.Empty(t, out.Steps[0].OutputSchema)
}
