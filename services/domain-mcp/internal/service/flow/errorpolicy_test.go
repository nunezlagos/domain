package flow

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func baseSpec(step Step) Spec {
	return Spec{Version: 1, Steps: []Step{step}}
}

func TestValidate_FallbackStepRequiresDefinition(t *testing.T) {
	err := baseSpec(Step{ID: "s1", Type: StepTypeSkillRun, OnError: "fallback_step"}).Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "requires fallback_step")
}

func TestValidate_IgnoreAndContinueRequiresDefault(t *testing.T) {
	err := baseSpec(Step{ID: "s1", Type: StepTypeSkillRun, OnError: "ignore_and_continue"}).Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "requires default_on_error")

	ok := baseSpec(Step{ID: "s1", Type: StepTypeSkillRun, OnError: "ignore_and_continue",
		DefaultOnError: map[string]any{"fallback_value": "default"}}).Validate()
	require.NoError(t, ok)
}

func TestValidate_MaxRetriesCap(t *testing.T) {
	err := baseSpec(Step{ID: "s1", Type: StepTypeSkillRun,
		Retry: &StepRetryPolicy{MaxRetries: 11}}).Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds cap")

	ok := baseSpec(Step{ID: "s1", Type: StepTypeSkillRun,
		Retry: &StepRetryPolicy{MaxRetries: 10}}).Validate()
	require.NoError(t, ok)
}

func TestValidate_InvalidBackoff(t *testing.T) {
	err := baseSpec(Step{ID: "s1", Type: StepTypeSkillRun,
		Retry: &StepRetryPolicy{MaxRetries: 1, Backoff: "fibonacci"}}).Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "backoff")
}

func TestValidate_FallbackChainDepthLimit(t *testing.T) {

	lvl3 := &Step{ID: "f3", Type: StepTypeSkillRun, Config: map[string]any{}}
	lvl2 := &Step{ID: "f2", Type: StepTypeSkillRun, Config: map[string]any{},
		OnError: "fallback_step", FallbackStep: lvl3}
	lvl1 := &Step{ID: "f1", Type: StepTypeSkillRun, Config: map[string]any{},
		OnError: "fallback_step", FallbackStep: lvl2}
	ok := baseSpec(Step{ID: "s1", Type: StepTypeSkillRun,
		OnError: "fallback_step", FallbackStep: lvl1}).Validate()
	require.NoError(t, ok, "3 niveles de fallback deben ser válidos")


	lvl4 := &Step{ID: "f4", Type: StepTypeSkillRun, Config: map[string]any{}}
	lvl3.OnError = "fallback_step"
	lvl3.FallbackStep = lvl4
	err := baseSpec(Step{ID: "s1", Type: StepTypeSkillRun,
		OnError: "fallback_step", FallbackStep: lvl1}).Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "fallback chain exceeds")
}

func TestValidate_DefaultStepErrorPolicy(t *testing.T) {
	s := baseSpec(Step{ID: "s1", Type: StepTypeSkillRun})
	s.DefaultStepErrorPolicy = "abort_flow"
	require.NoError(t, s.Validate())

	s.DefaultStepErrorPolicy = "explode"
	err := s.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "default_step_error_policy")
}

func TestValidate_AbortFlowAlias(t *testing.T) {
	require.NoError(t, baseSpec(Step{ID: "s1", Type: StepTypeSkillRun,
		OnError: "abort_flow"}).Validate())
}

// Sabotaje: fallback con type inválido debe ser rechazado en save, no en runtime.
func TestSabotage_FallbackInvalidType_RejectedAtSave(t *testing.T) {
	err := baseSpec(Step{ID: "s1", Type: StepTypeSkillRun,
		OnError: "fallback_step",
		FallbackStep: &Step{ID: "fb", Type: "esoteric_type"}}).Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "not valid")
}
