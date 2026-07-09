package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/orchestrator/modes"
	"nunezlagos/domain/internal/service/orchestrator/phases"
)

// R4: exportPlan en modo full debe omitir el SystemPrompt de los steps 2..N
// (payload a dieta). El step 0 conserva su SystemPrompt.

func TestExportPlan_Full_SoloStep0TieneSystemPrompt(t *testing.T) {
	plan := &modes.PhasePlan{
		Mode: "full",
		Steps: []modes.PhaseStep{
			{Slug: phases.PhaseSlug("sdd-explore"), SystemPrompt: "SYS-0 largo con rulesBlock", UserPrompt: "USER-0"},
			{Slug: phases.PhaseSlug("sdd-spec"), SystemPrompt: "SYS-1 largo con rulesBlock"},
			{Slug: phases.PhaseSlug("sdd-propose"), SystemPrompt: "SYS-2 largo con rulesBlock"},
		},
	}
	out := exportPlan(plan, true)
	require.Len(t, out.Steps, 3)
	assert.Equal(t, "SYS-0 largo con rulesBlock", out.Steps[0].SystemPrompt, "step 0 conserva SystemPrompt")
	assert.Empty(t, out.Steps[1].SystemPrompt, "step 1 NO debe traer SystemPrompt (lazy)")
	assert.Empty(t, out.Steps[2].SystemPrompt, "step 2 NO debe traer SystemPrompt (lazy)")
	// el UserPrompt del step 0 se conserva; los IDs/slugs de todos también.
	assert.Equal(t, "USER-0", out.Steps[0].UserPrompt)
	assert.Equal(t, "sdd-spec", string(out.Steps[1].Slug))
}

// Retrocompat: en modos NO-full (express/lite), exportPlan mantiene los prompts
// (esos modos son cortos y no sufren el payload obeso).
func TestExportPlan_Express_ConservaTodosLosSystemPrompts(t *testing.T) {
	plan := &modes.PhasePlan{
		Mode: "express",
		Steps: []modes.PhaseStep{
			{Slug: phases.PhaseSlug("sdd-apply"), SystemPrompt: "SYS-A"},
			{Slug: phases.PhaseSlug("sdd-verify"), SystemPrompt: "SYS-B"},
		},
	}
	out := exportPlan(plan, true)
	require.Len(t, out.Steps, 2)
	assert.Equal(t, "SYS-A", out.Steps[0].SystemPrompt)
	assert.Equal(t, "SYS-B", out.Steps[1].SystemPrompt, "express conserva todos los SystemPrompt")
}

// R4: en modo detect (preview) el plan tiene Mode="full" pero NO se persiste,
// así que NO se debe strippear el SystemPrompt (no habría de dónde reconstruir).
// persisted=false lo preserva. (Hallazgo del panel adversarial.)
func TestExportPlan_NoPersistido_ConservaSystemPrompt(t *testing.T) {
	plan := &modes.PhasePlan{
		Mode: "full",
		Steps: []modes.PhaseStep{
			{Slug: phases.PhaseSlug("sdd-explore"), SystemPrompt: "SYS-0"},
			{Slug: phases.PhaseSlug("sdd-spec"), SystemPrompt: "SYS-1"},
		},
	}
	out := exportPlan(plan, false) // detect / no persistido
	require.Len(t, out.Steps, 2)
	assert.Equal(t, "SYS-0", out.Steps[0].SystemPrompt)
	assert.Equal(t, "SYS-1", out.Steps[1].SystemPrompt,
		"sin persistir no se strippea: el preview conserva el SystemPrompt")
}
