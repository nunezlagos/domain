//go:build integration

package orchestrator_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/seeds"
	"nunezlagos/domain/internal/service/orchestrator"
	"nunezlagos/domain/internal/service/orchestrator/phases"
)

// test-006 — Resume cross-session.
//
// Escenario: usuario arranca un flow Full, completa 2 fases (sdd-explore
// + sdd-spec), la sesión se corta. Después abre una nueva sesión:
//   1. Usa GetFlowStatus para ver dónde quedó
//   2. Recupera el prompt del próximo step pending (sdd-propose)
//   3. Continúa reportando phase_results normalmente
//
// La separación state-vs-exec del RFC 0006 garantiza que el Service NO
// tiene estado in-memory crítico — todo vive en BD (flow_runs, flow_run_steps,
// agent_templates). Crear un Service nuevo (simulando reinicio del proceso)
// debe poder continuar el flow sin pérdida.
func TestService_ResumeCrossSession(t *testing.T) {
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


	session1 := orchestrator.New(pools.App, nil, buildFullRegistry(), "dev")
	res, err := session1.Run(ctx, orchestrator.OrchestrateInput{
		OrganizationID: orgID,
		ProjectID:      projectID,
		UserID:         userID,
		RawText:        "implementar feature de export PDF",
		Mode:           orchestrator.ModeFull,
	})
	require.NoError(t, err)
	require.Len(t, res.Plan.Steps, 10)
	flowRunID := res.FlowRunID


	exploreOut := map[string]any{
		"intent":           "feature",
		"scope":            "multi-file",
		"modules_affected": []any{"internal/export", "internal/api/handler"},
		"summary":          "feature export PDF",
	}
	r, err := session1.RecordPhaseResult(ctx, orchestrator.PhaseResultInput{
		FlowRunStepID: res.Plan.Steps[0].ID,
		Output:        exploreOut,
	})
	require.NoError(t, err)
	require.Equal(t, "completed", r.StepStatus)
	require.Equal(t, "sdd-spec", r.NextStepKey)


	specOut := map[string]any{
		"issue_slug": "issue-99.1-export-pdf",
		"issue_md":   "# Spec del export PDF\n\nGherkin scenarios...",
	}
	specStepID := *r.NextStepID
	r2, err := session1.RecordPhaseResult(ctx, orchestrator.PhaseResultInput{
		FlowRunStepID: specStepID,
		Output:        specOut,
	})
	require.NoError(t, err)
	require.Equal(t, "completed", r2.StepStatus)
	require.Equal(t, "sdd-propose", r2.NextStepKey)
	proposeStepID := *r2.NextStepID


	session2 := orchestrator.New(pools.App, nil, buildFullRegistry(), "dev")


	status, err := session2.GetFlowStatus(ctx, flowRunID)
	require.NoError(t, err)
	require.Equal(t, "running", status.Status)
	require.Equal(t, "full", status.Mode)
	require.Len(t, status.Steps, 10)

	require.Equal(t, "completed", status.Steps[0].Status, "sdd-explore completed")
	require.Equal(t, "completed", status.Steps[1].Status, "sdd-spec completed")

	for i := 2; i < 10; i++ {
		require.Equal(t, "pending", status.Steps[i].Status,
			"step %d (%s) debe estar pending", i, status.Steps[i].StepKey)
	}



	require.Equal(t, "sdd-propose", status.Steps[2].StepKey)
	require.NotEmpty(t, status.Steps[2].UserPromptPreview,
		"propose user_prompt debe estar persistido en BD (lazy build)")


	proposeOut := map[string]any{
		"proposal_md": "Scope: implementar export PDF...",
		"status":      "draft",
	}
	r3, err := session2.RecordPhaseResult(ctx, orchestrator.PhaseResultInput{
		FlowRunStepID: proposeStepID,
		Output:        proposeOut,
	})
	require.NoError(t, err)
	require.Equal(t, "completed", r3.StepStatus)
	require.Equal(t, "sdd-design", r3.NextStepKey,
		"session2 reanuda sin saber del corte; aggregateFlowStatus calcula el next correcto")
	require.NotEmpty(t, r3.NextStepPrompt,
		"el prompt de sdd-design se hizo lazy build con propose.output incluido")


	require.Contains(t, r3.NextStepPrompt, "Proposal",
		"design prompt incluye proposal_md (lazy build con PriorOutputs)")
}

// Confirm condicional resume: si Express quedó blocked esperando confirm,
// una sesión nueva puede invocar ConfirmContinue sin estado in-memory previo.
func TestService_ResumeCrossSession_PendingConfirm(t *testing.T) {
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


	session1 := orchestrator.New(pools.App, nil, buildRegistry(), "dev")
	res, err := session1.Run(ctx, orchestrator.OrchestrateInput{
		OrganizationID: orgID,
		ProjectID:      projectID,
		UserID:         userID,
		RawText:        "refactor mediano",
		Mode:           orchestrator.ModeExpress,
	})
	require.NoError(t, err)

	applyID := res.Plan.Steps[0].ID
	verifyID := res.Plan.Steps[1].ID
	_, err = session1.RecordPhaseResult(ctx, orchestrator.PhaseResultInput{
		FlowRunStepID: applyID,
		Output: map[string]any{
			"files_changed": []any{"a.go"},
			"lines_changed": 30, // > default 10
		},
		MemoryRefsSaved: []phases.MemoryRef{{Type: "code_reference", ID: uuid.New()}},
	})
	require.NoError(t, err)


	var status string
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT status FROM flow_run_steps WHERE id=$1`, verifyID).Scan(&status))
	require.Equal(t, "blocked", status)


	session2 := orchestrator.New(pools.App, nil, buildRegistry(), "dev")


	st, err := session2.GetFlowStatus(ctx, res.FlowRunID)
	require.NoError(t, err)
	require.Equal(t, "blocked", st.Steps[1].Status)


	confirmRes, err := session2.ConfirmContinue(ctx, res.FlowRunID, true)
	require.NoError(t, err)
	require.Equal(t, "pending", confirmRes.StepStatus)
	require.Equal(t, verifyID, confirmRes.StepID)
	require.NotEmpty(t, confirmRes.NextStepPrompt, "user_prompt cacheado en step.inputs sobrevive el corte")
}
