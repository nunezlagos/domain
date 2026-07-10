package orchestrator

import (
	"strings"
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

// DOMAINSERV-3: el stripping del SystemPrompt de los steps 1..N aplica a TODOS
// los modos persistidos, no solo full. Los modos cortos (express/lite) SÍ sufrían
// el payload obeso (medido: 129.601 chars en lite por el rulesBlock ~34KB duplicado
// en cada step). El step 0 se conserva porque no tiene canal NextStepSystemPrompt.
func TestExportPlan_Express_StrippeaSystemPromptDeStepsPosteriores(t *testing.T) {
	plan := &modes.PhasePlan{
		Mode: "express",
		Steps: []modes.PhaseStep{
			{Slug: phases.PhaseSlug("sdd-apply"), SystemPrompt: "SYS-A"},
			{Slug: phases.PhaseSlug("sdd-verify"), SystemPrompt: "SYS-B"},
		},
	}
	out := exportPlan(plan, true)
	require.Len(t, out.Steps, 2)
	assert.Equal(t, "SYS-A", out.Steps[0].SystemPrompt, "step 0 conserva SystemPrompt")
	assert.Empty(t, out.Steps[1].SystemPrompt, "modo no-full persistido también strippea steps 1..N (DOMAINSERV-3)")
}

// DOMAINSERV-3: reproduce la medición real del bug. En lite, cada step arrastraba
// el mismo rulesBlock (~34KB), inflando el response ~120k+. Tras la fix, el
// rulesBlock duplicado de los steps 1..N desaparece del response inicial.
func TestExportPlan_Lite_NoArrastraRulesBlockDuplicado(t *testing.T) {
	rulesBlock := strings.Repeat("R", 34000)
	plan := &modes.PhasePlan{
		Mode: "lite",
		Steps: []modes.PhaseStep{
			{Slug: phases.PhaseSlug("sdd-explore"), SystemPrompt: "EXPLORE-" + rulesBlock},
			{Slug: phases.PhaseSlug("sdd-apply"), SystemPrompt: "APPLY-" + rulesBlock},
			{Slug: phases.PhaseSlug("sdd-verify"), SystemPrompt: "VERIFY-" + rulesBlock},
		},
	}
	rawTotal := 0
	for _, st := range plan.Steps {
		rawTotal += len(st.SystemPrompt)
	}

	out := exportPlan(plan, true)
	require.Len(t, out.Steps, 3)
	assert.NotEmpty(t, out.Steps[0].SystemPrompt, "step 0 conserva su SystemPrompt inline")
	assert.Empty(t, out.Steps[1].SystemPrompt, "step 1 no arrastra rulesBlock")
	assert.Empty(t, out.Steps[2].SystemPrompt, "step 2 no arrastra rulesBlock")

	exportedTotal := 0
	for _, st := range out.Steps {
		exportedTotal += len(st.SystemPrompt)
	}
	assert.Less(t, exportedTotal, rawTotal-60000,
		"el rulesBlock duplicado de los steps 1..N (~68k) desaparece del response")
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
