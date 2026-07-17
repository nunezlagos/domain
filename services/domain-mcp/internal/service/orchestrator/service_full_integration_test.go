//go:build integration

package orchestrator_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/seeds"
	"nunezlagos/domain/internal/service/orchestrator"
	"nunezlagos/domain/internal/service/orchestrator/phases"
)

// buildFullRegistry registra los 12 handlers para tests del modo Full.
func buildFullRegistry() *phases.Registry {
	reg := phases.NewRegistry()
	reg.MustRegister(phases.NewSDDExploreHandler())
	reg.MustRegister(phases.NewSDDSpecHandler())
	reg.MustRegister(phases.NewSDDProposeHandler())
	reg.MustRegister(phases.NewSDDDesignHandler())
	reg.MustRegister(phases.NewSDDTasksHandler())
	reg.MustRegister(phases.NewSDDApplyHandler())
	reg.MustRegister(phases.NewSDDVerifyHandler())
	reg.MustRegister(phases.NewSDDJudgeHandler())
	reg.MustRegister(phases.NewSDD4RHandler())
	reg.MustRegister(phases.NewSDDReviewHandler())
	reg.MustRegister(phases.NewSDDArchiveHandler())
	reg.MustRegister(phases.NewSDDOnboardHandler())
	return reg
}

func TestService_Run_Full_Persists10StepsWithFirstPromptOnly(t *testing.T) {
	pools, cleanup := setupOrchestratorDB(t)
	defer cleanup()
	ctx := context.Background()

	orgID := newOrgID(t, pools)
	userID := newUserID(t, pools, orgID)
	projectID := newProjectID(t, pools, orgID)
	_, err := seeds.SeedAgentTemplatesForOrg(ctx, pools.App, orgID)
	require.NoError(t, err)
	_, err = seeds.SeedFlowsForOrg(ctx, pools.App, orgID)
	require.NoError(t, err)

	s := orchestrator.New(pools.App, nil, buildFullRegistry(), "dev")
	res, err := s.Run(ctx, orchestrator.OrchestrateInput{
		OrganizationID: orgID,
		ProjectID:      projectID,
		UserID:         userID,
		RawText:        "implementar feature compleja con SDD completo",
		Mode:           orchestrator.ModeFull,
	})
	require.NoError(t, err)
	require.Equal(t, orchestrator.ModeFull, res.Mode)
	require.Len(t, res.Plan.Steps, 12)
	require.Equal(t, "sdd-explore", string(res.Plan.Steps[0].Slug))
	require.Equal(t, "sdd-4r", string(res.Plan.Steps[8].Slug))
	require.Equal(t, "sdd-archive", string(res.Plan.Steps[10].Slug))
	require.Equal(t, "sdd-onboard", string(res.Plan.Steps[11].Slug))

	require.NotEmpty(t, res.Plan.Steps[0].UserPrompt)
	for i := 1; i < 12; i++ {
		require.Empty(t, res.Plan.Steps[i].UserPrompt,
			"step[%d] (%s) debe tener UserPrompt vacío en Full (lazy)", i, res.Plan.Steps[i].Slug)
	}

	require.Equal(t, res.Plan.Steps[0].UserPrompt, res.SnapshotPrompt)

	rows, err := pools.App.Query(ctx,
		`SELECT step_key, status, inputs FROM flow_run_steps WHERE flow_run_id=$1 ORDER BY created_at`,
		res.FlowRunID)
	require.NoError(t, err)
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var k, st string
		var inputsRaw []byte
		require.NoError(t, rows.Scan(&k, &st, &inputsRaw))
		require.Equal(t, "pending", st)
		var inp map[string]any
		require.NoError(t, json.Unmarshal(inputsRaw, &inp))

		sysPrompt, _ := inp["system_prompt"].(string)
		require.NotEmpty(t, sysPrompt, "step %s debe tener system_prompt hidratado desde agent_templates", k)

		require.Equal(t, "implementar feature compleja con SDD completo", inp["raw_text"])
		keys = append(keys, k)
	}
	require.Equal(t, []string{
		"sdd-explore", "sdd-spec", "sdd-propose", "sdd-design", "sdd-tasks",
		"sdd-apply", "sdd-verify", "sdd-judge", "sdd-4r", "sdd-review", "sdd-archive", "sdd-onboard",
	}, keys)
}

func TestService_Run_Full_LazyBuildsNextPromptOnPhaseResult(t *testing.T) {
	pools, cleanup := setupOrchestratorDB(t)
	defer cleanup()
	ctx := context.Background()

	orgID := newOrgID(t, pools)
	userID := newUserID(t, pools, orgID)
	projectID := newProjectID(t, pools, orgID)
	_, err := seeds.SeedAgentTemplatesForOrg(ctx, pools.App, orgID)
	require.NoError(t, err)
	_, err = seeds.SeedFlowsForOrg(ctx, pools.App, orgID)
	require.NoError(t, err)

	s := orchestrator.New(pools.App, nil, buildFullRegistry(), "dev")
	res, err := s.Run(ctx, orchestrator.OrchestrateInput{
		OrganizationID: orgID,
		ProjectID:      projectID,
		UserID:         userID,
		RawText:        "implementar logging estructurado",
		Mode:           orchestrator.ModeFull,
	})
	require.NoError(t, err)

	exploreStepID := res.Plan.Steps[0].ID
	nextRes, err := s.RecordPhaseResult(ctx, orchestrator.PhaseResultInput{
		FlowRunStepID: exploreStepID,
		Output: map[string]any{
			"intent":           "feature",
			"scope":            "multi-file",
			"modules_affected": []any{"internal/logging", "cmd/domain"},
			"summary":          "agregar slog estructurado",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "completed", nextRes.StepStatus)
	require.Equal(t, "running", nextRes.FlowRunStatus)
	require.NotNil(t, nextRes.NextStepID)
	require.Equal(t, "sdd-spec", nextRes.NextStepKey)

	require.NotEmpty(t, nextRes.NextStepPrompt, "lazy build debe rellenar NextStepPrompt")
	require.Contains(t, nextRes.NextStepPrompt, "feature", "prompt de spec debe incluir intent del explore")
	require.Contains(t, nextRes.NextStepPrompt, "multi-file", "debe incluir scope")
	require.Contains(t, nextRes.NextStepPrompt, "internal/logging", "debe incluir módulos afectados")
	require.Contains(t, nextRes.NextStepPrompt, "implementar logging estructurado", "raw_text propagado")

	var inputsRaw []byte
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT inputs FROM flow_run_steps WHERE id=$1`, *nextRes.NextStepID,
	).Scan(&inputsRaw))
	var inputs map[string]any
	require.NoError(t, json.Unmarshal(inputsRaw, &inputs))
	require.Equal(t, nextRes.NextStepPrompt, inputs["user_prompt"])
}

func TestService_Run_Full_SkipPhases_OmitsSelected(t *testing.T) {
	pools, cleanup := setupOrchestratorDB(t)
	defer cleanup()
	ctx := context.Background()

	orgID := newOrgID(t, pools)
	userID := newUserID(t, pools, orgID)
	projectID := newProjectID(t, pools, orgID)
	_, err := seeds.SeedAgentTemplatesForOrg(ctx, pools.App, orgID)
	require.NoError(t, err)
	_, err = seeds.SeedFlowsForOrg(ctx, pools.App, orgID)
	require.NoError(t, err)

	s := orchestrator.New(pools.App, nil, buildFullRegistry(), "dev")
	res, err := s.Run(ctx, orchestrator.OrchestrateInput{
		OrganizationID: orgID,
		ProjectID:      projectID,
		UserID:         userID,
		RawText:        "doc only",
		Mode:           orchestrator.ModeFull,
		SkipPhases: []orchestrator.PhaseSlug{
			orchestrator.PhaseSlug("sdd-archive"),
			orchestrator.PhaseSlug("sdd-onboard"),
		},
	})
	require.NoError(t, err)
	require.Len(t, res.Plan.Steps, 10, "12 - 2 skipped = 10 steps")
	for _, st := range res.Plan.Steps {
		require.NotEqual(t, "sdd-archive", string(st.Slug))
		require.NotEqual(t, "sdd-onboard", string(st.Slug))
	}
}

func TestService_Run_Full_StartingPhase_StartsFromMiddle(t *testing.T) {
	pools, cleanup := setupOrchestratorDB(t)
	defer cleanup()
	ctx := context.Background()

	orgID := newOrgID(t, pools)
	userID := newUserID(t, pools, orgID)
	projectID := newProjectID(t, pools, orgID)
	_, err := seeds.SeedAgentTemplatesForOrg(ctx, pools.App, orgID)
	require.NoError(t, err)
	_, err = seeds.SeedFlowsForOrg(ctx, pools.App, orgID)
	require.NoError(t, err)

	s := orchestrator.New(pools.App, nil, buildFullRegistry(), "dev")
	res, err := s.Run(ctx, orchestrator.OrchestrateInput{
		OrganizationID: orgID,
		ProjectID:      projectID,
		UserID:         userID,
		RawText:        "resume desde apply",
		Mode:           orchestrator.ModeFull,
		StartingPhase:  orchestrator.PhaseSlug("sdd-apply"),
	})
	require.NoError(t, err)

	require.Len(t, res.Plan.Steps, 7)
	require.Equal(t, "sdd-apply", string(res.Plan.Steps[0].Slug))
	require.Equal(t, "sdd-onboard", string(res.Plan.Steps[6].Slug))
}

// Happy path completo: ejecutar las 12 fases en orden con outputs encadenados.
// Verifica que cada fase recibe los outputs de la anterior vía PriorOutputs
// (la firma de Full mode).
func TestService_Run_Full_EndToEnd_11Phases(t *testing.T) {
	pools, cleanup := setupOrchestratorDB(t)
	defer cleanup()
	ctx := context.Background()

	orgID := newOrgID(t, pools)
	userID := newUserID(t, pools, orgID)
	projectID := newProjectID(t, pools, orgID)
	_, err := seeds.SeedAgentTemplatesForOrg(ctx, pools.App, orgID)
	require.NoError(t, err)
	_, err = seeds.SeedFlowsForOrg(ctx, pools.App, orgID)
	require.NoError(t, err)

	s := orchestrator.New(pools.App, nil, buildFullRegistry(), "dev")
	res, err := s.Run(ctx, orchestrator.OrchestrateInput{
		OrganizationID: orgID,
		ProjectID:      projectID,
		UserID:         userID,
		RawText:        "implementar feature foo",
		Mode:           orchestrator.ModeFull,
	})
	require.NoError(t, err)
	require.Len(t, res.Plan.Steps, 12)

	outputs := []map[string]any{
		{"intent": "feature", "scope": "single-file", "summary": "x"},           // explore
		{"issue_slug": "issue-99.1-foo", "issue_md": "# spec"},                  // spec
		{"proposal_md": "scope X", "status": "draft"},                           // propose
		{"design_md": "design X", "adrs": []any{map[string]any{"id": "ADR-1"}}}, // design
		{"tasks": []any{map[string]any{"id": "T01", "description": "do x"}}},    // tasks
		{"summary": "implemented", "files_changed": []any{"a.go"}},              // apply
		{"scenarios_failed": []any{}, "tests_passed": 5},                        // verify
		{"sabotage_records": []any{map[string]any{"invariant": "x"}}},           // judge
		{"lens_reports": []any{ // 4r: 4 lenses limpias con evidencia
			map[string]any{"lens": "R1", "findings": []any{}, "evidence": []any{"scope revisado"}},
			map[string]any{"lens": "R2", "findings": []any{}, "evidence": []any{"scope revisado"}},
			map[string]any{"lens": "R3", "findings": []any{}, "evidence": []any{"scope revisado"}},
			map[string]any{"lens": "R4", "findings": []any{}, "evidence": []any{"scope revisado"}},
		}},
		{"verdict": "compliant", "policies_checked": 3}, // review
		{"archived": true},                             // archive
		{"skipped": true},                              // onboard
	}

	memrefs := []map[string][]phases.MemoryRef{
		{}, // explore
		{}, // spec
		{"saves": {{Type: "knowledge_doc", ID: uuid.New()}}}, // propose (Feature B)
		{"saves": { // design: adr (D5) + knowledge_doc (Feature B)
			{Type: "adr", ID: uuid.New()},
			{Type: "knowledge_doc", ID: uuid.New()},
		}},
		{"saves": {{Type: "knowledge_doc", ID: uuid.New()}}},  // tasks (Feature B)
		{"saves": {{Type: "code_reference", ID: uuid.New()}}}, // apply
		{}, // verify
		{"saves": {{Type: "sabotage_record", ID: uuid.New()}}}, // judge
		{}, // 4r
		{}, // review
		{}, // archive
		{}, // onboard
	}

	for i, step := range res.Plan.Steps {
		var refs []phases.MemoryRef
		if r, ok := memrefs[i]["saves"]; ok {
			refs = r
		}
		out, err := s.RecordPhaseResult(ctx, orchestrator.PhaseResultInput{
			FlowRunStepID:   step.ID,
			Output:          outputs[i],
			MemoryRefsSaved: refs,
		})
		require.NoErrorf(t, err, "fase %d (%s) falló inesperado", i, step.Slug)
		require.Equal(t, "completed", out.StepStatus)
		if i < 11 {
			require.Equal(t, "running", out.FlowRunStatus,
				"flow sigue running tras fase %d", i)
			require.NotNil(t, out.NextStepID)
			require.NotEmpty(t, out.NextStepPrompt,
				"lazy build próximo prompt para fase %d", i+1)
		} else {
			require.Equal(t, "completed", out.FlowRunStatus,
				"flow completed tras última fase")
			require.Nil(t, out.NextStepID)
		}
	}

	st, err := s.GetFlowStatus(ctx, res.FlowRunID)
	require.NoError(t, err)
	require.Equal(t, "completed", st.Status)
	require.Equal(t, "full", st.Mode)
	for _, s := range st.Steps {
		require.Equal(t, "completed", s.Status)
	}
}
