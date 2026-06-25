package seeds

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFlowsCatalog_HasExactlySDDPipeline(t *testing.T) {
	t.Parallel()
	c := FlowsCatalog()
	require.Len(t, c, 1, "hoy sólo sdd-pipeline-v1 está en el catálogo")
	require.Equal(t, SDDPipelineFlowSlug, c[0].Slug)
}

func TestFlowsCatalog_SDDPipelineSpec_HasAllTenPhasesInOrder(t *testing.T) {
	t.Parallel()
	spec := buildSDDPipelineSpec()
	require.Equal(t, 1, spec.Version)
	require.Len(t, spec.Steps, 10, "10 fases SDD")
	for i, slug := range SDDPipelinePhaseSlugs {
		require.Equal(t, slug, spec.Steps[i].ID, "orden canónico")
		require.Equal(t, "agent_run", spec.Steps[i].Type)
		require.Equal(t, "fail", spec.Steps[i].OnError)

		require.Equal(t, slug, spec.Steps[i].Config["agent_template_slug"])
		require.Equal(t, slug, spec.Steps[i].Config["phase"])

		require.Equal(t, retryPolicyForPhase(slug), spec.Steps[i].Config["retry_policy"])
	}
}

func TestRetryPolicyForPhase_RFC0006Mapping(t *testing.T) {
	t.Parallel()
	require.Equal(t, "require-cleanup", retryPolicyForPhase("sdd-apply"),
		"apply muta workspace — RFC 0006 ADR-1")
	require.Equal(t, "re-emit", retryPolicyForPhase("sdd-verify"),
		"verify es read-only — RFC 0006 ADR-1")
	require.Equal(t, "", retryPolicyForPhase("sdd-design"),
		"resto: auto-retry default")
	require.Equal(t, "", retryPolicyForPhase("phase-no-existe"),
		"slug desconocido cae al default sin panic")
}

func TestFlowsCatalog_SpecMarshalsToValidJSON(t *testing.T) {
	t.Parallel()
	c := FlowsCatalog()
	raw, err := json.Marshal(c[0].Spec)
	require.NoError(t, err)


	var back FlowSpecJSON
	require.NoError(t, json.Unmarshal(raw, &back))
	require.Equal(t, c[0].Spec, back)
}

// Sabotage sanity: si alguien borrara una phase del slice canónico,
// el spec resultante tendría menos de 10 steps. Esto rompe el orden
// del pipeline → este test atrapa esa regresión.
func TestSDDPipelinePhaseSlugs_NoMissingPhases(t *testing.T) {
	t.Parallel()
	expected := []string{
		"sdd-explore", "sdd-spec", "sdd-propose", "sdd-design",
		"sdd-tasks", "sdd-apply", "sdd-verify", "sdd-judge",
		"sdd-archive", "sdd-onboard",
	}
	require.Equal(t, expected, SDDPipelinePhaseSlugs,
		"ningún reorder/drop del pipeline canónico sin actualizar este test + RFC")
}

func TestFlowsCatalog_SDDPipeline_DeterministicReplayFalse(t *testing.T) {
	t.Parallel()
	c := FlowsCatalog()
	require.False(t, c[0].DeterministicReplay,
		"flows con steps tipo agent_run usan LLMs no-deterministas — replay no aplica")
}

func TestNullStrSeed(t *testing.T) {
	t.Parallel()
	require.Nil(t, nullStrSeed(""))
	require.Equal(t, "x", nullStrSeed("x"))
}
