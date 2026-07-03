package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/orchestrator/phases"
)

// allSDDPhases: las 11 fases registradas del pipeline (fuente: los
// MustRegister de cmd/domain/server_services.go y cmd/domain-mcp/main.go).
var allSDDPhases = []string{
	"sdd-explore", "sdd-spec", "sdd-propose", "sdd-design", "sdd-tasks",
	"sdd-apply", "sdd-verify", "sdd-judge", "sdd-review", "sdd-archive",
	"sdd-onboard",
}

// REQ-54 issue-54.6: toda fase tiene entrada EXPLÍCITA en el mapeo de
// preparación de contexto — vacía solo de forma deliberada. Fase nueva sin
// entrada = rojo acá.
func TestPrepContext_AllPhasesMapped(t *testing.T) {
	t.Parallel()
	for _, slug := range allSDDPhases {
		_, ok := prepPhaseContext[slug]
		require.True(t, ok, "fase %s sin entrada en prepPhaseContext (issue-54.6: agregala, vacía si es deliberado)", slug)
	}
}

// REQ-54 issue-54.6: tabla canónica de contratos required_tool_calls por
// fase. Vacío = deliberado con razón. Cambiar un contrato = actualizar esta
// tabla conscientemente (el diff del PR lo muestra).
func TestPhaseContracts_MatchCanonicalTable(t *testing.T) {
	t.Parallel()
	expected := map[string][]string{
		"sdd-explore": {"domain_code_graph"},
		"sdd-spec":    nil, // creativo: el contrato de saves lo cubre D5
		"sdd-propose": {"domain_openspec_export", "domain_openspec_apply"}, // REQ-55.3
		"sdd-design":  {"domain_openspec_export", "domain_openspec_apply"}, // REQ-55.3
		"sdd-tasks":   {"domain_openspec_export", "domain_openspec_apply"}, // REQ-55.3
		"sdd-apply":   nil, // el trabajo es código local; saves via D5
		"sdd-verify":  {"domain_verify_start", "domain_verify_complete"},
		"sdd-judge":   nil, // teeth via shape del Output (issue-54.5)
		"sdd-review":  {"domain_project_policy_list", "domain_verify_start", "domain_verify_update_item", "domain_verify_complete"},
		"sdd-archive": {"domain_openspec_status"},
		"sdd-onboard": {"domain_knowledge_save"},
	}

	reg := phases.NewRegistry()
	reg.MustRegister(phases.NewSDDExploreHandler())
	reg.MustRegister(phases.NewSDDSpecHandler())
	reg.MustRegister(phases.NewSDDProposeHandler())
	reg.MustRegister(phases.NewSDDDesignHandler())
	reg.MustRegister(phases.NewSDDTasksHandler())
	reg.MustRegister(phases.NewSDDApplyHandler())
	reg.MustRegister(phases.NewSDDVerifyHandler())
	reg.MustRegister(phases.NewSDDJudgeHandler())
	reg.MustRegister(phases.NewSDDReviewHandler())
	reg.MustRegister(phases.NewSDDArchiveHandler())
	reg.MustRegister(phases.NewSDDOnboardHandler())

	for _, slug := range allSDDPhases {
		h, err := reg.Lookup(phases.PhaseSlug(slug))
		require.NoError(t, err, "fase %s no registrada", slug)
		out, err := h.Build(context.Background(), phases.Input{
			RawText:      "implementar módulo de prueba para el contrato",
			PhaseSlug:    phases.PhaseSlug(slug),
			PriorOutputs: map[phases.PhaseSlug]map[string]any{},
		})
		require.NoError(t, err, "Build de %s falló", slug)
		require.Equal(t, expected[slug], out.RequiredToolCalls,
			"contrato de %s divergió de la tabla canónica (issue-54.6)", slug)
	}
}
