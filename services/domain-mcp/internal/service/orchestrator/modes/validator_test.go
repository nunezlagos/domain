package modes

import (
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/orchestrator/phases"
)

func TestValidateDAG_NoSkips_AlwaysValid(t *testing.T) {
	err := ValidateDAG(FullPhases, nil, "")
	require.NoError(t, err, "sin skips siempre válido")
}

func TestValidateDAG_EmptySkipPhases_AlwaysValid(t *testing.T) {
	err := ValidateDAG(FullPhases, []phases.PhaseSlug{}, "")
	require.NoError(t, err, "skip vacío siempre válido")
}

func TestValidateDAG_SkipSuffix_Valid(t *testing.T) {
	tests := []struct {
		name   string
		skip   []phases.PhaseSlug
		expect int // remaining phases
	}{
		{"skip onboard", []phases.PhaseSlug{"sdd-onboard"}, 9},
		{"skip archive+onboard", []phases.PhaseSlug{"sdd-archive", "sdd-onboard"}, 8},
		{"skip judge+archive+onboard", []phases.PhaseSlug{"sdd-judge", "sdd-archive", "sdd-onboard"}, 7},
		{"skip all after explore", []phases.PhaseSlug{
			"sdd-spec", "sdd-propose", "sdd-design", "sdd-tasks",
			"sdd-apply", "sdd-verify", "sdd-judge", "sdd-archive", "sdd-onboard",
		}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDAG(FullPhases, tt.skip, "")
			require.NoError(t, err)
		})
	}
}

func TestValidateDAG_SkipMiddle_KeepsDependents_Invalid(t *testing.T) {
	tests := []struct {
		name string
		skip []phases.PhaseSlug
	}{
		{"skip spec keep propose", []phases.PhaseSlug{"sdd-spec"}},
		{"skip propose keep design", []phases.PhaseSlug{"sdd-propose"}},
		{"skip design keep tasks", []phases.PhaseSlug{"sdd-design"}},
		{"skip tasks keep apply", []phases.PhaseSlug{"sdd-tasks"}},
		{"skip apply keep verify", []phases.PhaseSlug{"sdd-apply"}},
		{"skip verify keep judge", []phases.PhaseSlug{"sdd-verify"}},
		{"skip judge keep archive", []phases.PhaseSlug{"sdd-judge"}},
		{"skip archive keep onboard", []phases.PhaseSlug{"sdd-archive"}},
		{"skip judge+archive keep onboard", []phases.PhaseSlug{"sdd-judge", "sdd-archive"}},
		{"skip explore keep spec", []phases.PhaseSlug{"sdd-explore"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDAG(FullPhases, tt.skip, "")
			require.Error(t, err, "DAG debe ser inválido cuando se salta dependencia")
			require.Contains(t, err.Error(), "requires")
		})
	}
}

func TestValidateDAG_WithStartingPhase_AssumesPriorsDone(t *testing.T) {


	err := ValidateDAG(FullPhases,
		[]phases.PhaseSlug{"sdd-apply"},
		phases.PhaseSlug("sdd-design"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "requires")


	err = ValidateDAG(FullPhases,
		[]phases.PhaseSlug{"sdd-archive", "sdd-onboard"},
		phases.PhaseSlug("sdd-design"))
	require.NoError(t, err)


	err = ValidateDAG(FullPhases,
		[]phases.PhaseSlug{"sdd-judge"},
		phases.PhaseSlug("sdd-verify"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "requires")
}

func TestValidateDAG_UnknownStartingPhase_ReturnsError(t *testing.T) {
	err := ValidateDAG(FullPhases, []phases.PhaseSlug{"sdd-apply"}, phases.PhaseSlug("sdd-nonexistent"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestValidateDAG_SkipAll_ValidBecauseNothingKept(t *testing.T) {
	err := ValidateDAG(FullPhases, []phases.PhaseSlug{
		"sdd-explore", "sdd-spec", "sdd-propose", "sdd-design",
		"sdd-tasks", "sdd-apply", "sdd-verify", "sdd-judge",
		"sdd-archive", "sdd-onboard",
	}, "")
	require.NoError(t, err, "saltar todas las fases es válido (plan vacío)")
}
