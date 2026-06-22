//go:build integration

package orchestrator_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/seeds"
	"nunezlagos/domain/internal/service/flow"
	"nunezlagos/domain/internal/service/orchestrator"
)

// TestService_Run_Async_ReturnsImmediately verifica que ModeAsync construye
// el plan, persiste flow_run + steps, y devuelve inmediatamente sin ejecutar.
// La ejecución queda pendiente para que ProcessAsyncFlowRun la procese.
func TestService_Run_Async_ReturnsImmediately(t *testing.T) {
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
		RawText:        "implement async mode",
		Mode:           orchestrator.ModeAsync,
	})
	require.NoError(t, err)
	require.Equal(t, orchestrator.ModeAsync, res.Mode)
	require.NotEqual(t, uuid.Nil, res.FlowRunID)
	require.NotNil(t, res.Plan, "Async debe devolver el plan")
	require.Len(t, res.Plan.Steps, 10, "Async plan = 10 fases sin skips")

	// Verificar que los steps quedaron pending (no ejecutados)
	rows, err := pools.App.Query(ctx,
		`SELECT step_key, status FROM flow_run_steps
		 WHERE flow_run_id=$1 ORDER BY created_at`, res.FlowRunID)
	require.NoError(t, err)
	defer rows.Close()
	count := 0
	for rows.Next() {
		var k, st string
		require.NoError(t, rows.Scan(&k, &st))
		require.Equal(t, "pending", st,
			"step %s debe estar pending tras Run async (sin Process)", k)
		count++
	}
	require.Equal(t, 10, count, "10 fases SDD persistidas")

	var flowStatus string
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT status FROM flow_runs WHERE id=$1`, res.FlowRunID,
	).Scan(&flowStatus))
	require.Equal(t, "pending", flowStatus, "flow_run pending hasta ProcessAsyncFlowRun")
}

// TestService_ProcessAsyncFlowRun_ExecutesAllSteps verifica que
// ProcessAsyncFlowRun ejecuta las 10 fases, emite signals, y marca
// flow_run + steps como completed.
func TestService_ProcessAsyncFlowRun_ExecutesAllSteps(t *testing.T) {
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

	factory := llm.NewFactory()
	factory.Register("anthropic", &fakeProvider{byPhase: cannedSoloResponses()})

	s := orchestrator.New(pools.App, nil, buildFullRegistry(), "dev")
	s.LLM = factory
	s.SignalStore = &flow.SignalStore{Pool: pools.App}

	res, err := s.Run(ctx, orchestrator.OrchestrateInput{
		OrganizationID: orgID,
		ProjectID:      projectID,
		UserID:         userID,
		RawText:        "implement async mode e2e",
		Mode:           orchestrator.ModeAsync,
	})
	require.NoError(t, err)

	err = s.ProcessAsyncFlowRun(ctx, res.FlowRunID)
	require.NoError(t, err)

	// Verificar que todos los steps quedaron completed
	rows, err := pools.App.Query(ctx,
		`SELECT step_key, status FROM flow_run_steps
		 WHERE flow_run_id=$1 ORDER BY created_at`, res.FlowRunID)
	require.NoError(t, err)
	defer rows.Close()
	count := 0
	for rows.Next() {
		var k, st string
		require.NoError(t, rows.Scan(&k, &st))
		require.Equal(t, "completed", st, "step %s debe estar completed", k)
		count++
	}
	require.Equal(t, 10, count, "10 fases ejecutadas por ProcessAsyncFlowRun")

	// flow_run terminal
	var flowStatus string
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT status FROM flow_runs WHERE id=$1`, res.FlowRunID,
	).Scan(&flowStatus))
	require.Equal(t, "completed", flowStatus)

	// Verificar signals emitidos
	signals, err := s.SignalStore.List(ctx, res.FlowRunID, true)
	require.NoError(t, err)
	// 10 step_completed + 1 flow_completed = 11 signals
	require.GreaterOrEqual(t, len(signals), 11, "debe haber al menos 11 signals (10 step + 1 flow)")

	stepCompletedCount := 0
	flowCompletedCount := 0
	for _, sig := range signals {
		switch sig.Name {
		case orchestrator.SignalNameStepCompleted:
			stepCompletedCount++
		case orchestrator.SignalNameFlowCompleted:
			flowCompletedCount++
		}
	}
	require.Equal(t, 10, stepCompletedCount, "10 step_completed signals")
	require.Equal(t, 1, flowCompletedCount, "1 flow_completed signal")
}

// TestService_ProcessAsyncFlowRun_WithoutLLMFactory_ReturnsError verifica
// que ProcessAsyncFlowRun falla si no hay LLM factory configurado.
func TestService_ProcessAsyncFlowRun_WithoutLLMFactory_ReturnsError(t *testing.T) {
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
	// LLM intentionally not set

	res, err := s.Run(ctx, orchestrator.OrchestrateInput{
		OrganizationID: orgID,
		ProjectID:      projectID,
		UserID:         userID,
		RawText:        "x",
		Mode:           orchestrator.ModeAsync,
	})
	require.NoError(t, err)

	err = s.ProcessAsyncFlowRun(ctx, res.FlowRunID)
	require.ErrorIs(t, err, orchestrator.ErrLLMFactoryRequired)
}

// TestService_ProcessAsyncFlowRun_NonAsyncFlow_ReturnsError verifica que
// ProcessAsyncFlowRun rechaza flow_runs que no están en modo async.
func TestService_ProcessAsyncFlowRun_NonAsyncFlow_ReturnsError(t *testing.T) {
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
	s.LLM = llm.NewFactory()

	// Persistir flow_run en modo express (no async) via Run()
	res, err := s.Run(ctx, orchestrator.OrchestrateInput{
		OrganizationID: orgID,
		ProjectID:      projectID,
		UserID:         userID,
		RawText:        "fix typo",
		Mode:           orchestrator.ModeExpress,
	})
	require.NoError(t, err)

	err = s.ProcessAsyncFlowRun(ctx, res.FlowRunID)
	require.ErrorIs(t, err, orchestrator.ErrAsyncFlowNotAsync)
}

// TestService_ProcessAsyncFlowRun_InvalidJSON_MarksStepFailed verifica que
// si el LLM devuelve JSON inválido, el step se marca failed, se emite
// failure signal, y el flow se marca failed.
func TestService_ProcessAsyncFlowRun_InvalidJSON_MarksStepFailed(t *testing.T) {
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

	canned := cannedSoloResponses()
	canned["sdd-explore"] = "invalid json no braces"

	factory := llm.NewFactory()
	factory.Register("anthropic", &fakeProvider{byPhase: canned})

	s := orchestrator.New(pools.App, nil, buildFullRegistry(), "dev")
	s.LLM = factory
	s.SignalStore = &flow.SignalStore{Pool: pools.App}

	res, err := s.Run(ctx, orchestrator.OrchestrateInput{
		OrganizationID: orgID, UserID: userID,
		ProjectID:      projectID,
		RawText: "x", Mode: orchestrator.ModeAsync,
	})
	require.NoError(t, err)

	err = s.ProcessAsyncFlowRun(ctx, res.FlowRunID)
	require.Error(t, err, "debe abortar con JSON inválido")

	// Verificar signal de failure
	signals, err := s.SignalStore.List(ctx, res.FlowRunID, true)
	require.NoError(t, err)
	hasFailedSignal := false
	for _, sig := range signals {
		if sig.Name == orchestrator.SignalNameStepFailed {
			hasFailedSignal = true
			break
		}
	}
	require.True(t, hasFailedSignal, "debe haber un step_failed signal")

	var flowStatus string
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT status FROM flow_runs WHERE id=$1`, res.FlowRunID,
	).Scan(&flowStatus))
	require.Equal(t, "failed", flowStatus)
}

// TestService_ProcessAsyncFlowRun_WithoutSignalStore_WorksDegraded verifica
// que ProcessAsyncFlowRun funciona sin SignalStore configurado (no-op).
func TestService_ProcessAsyncFlowRun_WithoutSignalStore_WorksDegraded(t *testing.T) {
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

	factory := llm.NewFactory()
	factory.Register("anthropic", &fakeProvider{byPhase: cannedSoloResponses()})

	s := orchestrator.New(pools.App, nil, buildFullRegistry(), "dev")
	s.LLM = factory
	// SignalStore NOT set → degraded mode

	res, err := s.Run(ctx, orchestrator.OrchestrateInput{
		OrganizationID: orgID, UserID: userID,
		ProjectID:      projectID,
		RawText: "x", Mode: orchestrator.ModeAsync,
	})
	require.NoError(t, err)

	err = s.ProcessAsyncFlowRun(ctx, res.FlowRunID)
	require.NoError(t, err, "debe funcionar sin SignalStore (degraded)")

	var flowStatus string
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT status FROM flow_runs WHERE id=$1`, res.FlowRunID,
	).Scan(&flowStatus))
	require.Equal(t, "completed", flowStatus)
}

// TestService_ProcessAsyncFlowRun_ResumesFromSavedPriors verifica que
// ProcessAsyncFlowRun reconstruye priors desde steps ya completados,
// permitiendo reanudación cross-session.
func TestService_ProcessAsyncFlowRun_ResumesFromSavedPriors(t *testing.T) {
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

	factory := llm.NewFactory()
	factory.Register("anthropic", &fakeProvider{byPhase: cannedSoloResponses()})

	s := orchestrator.New(pools.App, nil, buildFullRegistry(), "dev")
	s.LLM = factory

	res, err := s.Run(ctx, orchestrator.OrchestrateInput{
		OrganizationID: orgID, UserID: userID,
		ProjectID:      projectID,
		RawText: "x", Mode: orchestrator.ModeAsync,
	})
	require.NoError(t, err)

	// Marcar el primer step como completed manualmente para simular
	// que ya se procesó antes de una reanudación.
	var firstStepID uuid.UUID
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT id FROM flow_run_steps
		 WHERE flow_run_id=$1 ORDER BY created_at LIMIT 1`, res.FlowRunID,
	).Scan(&firstStepID))
	exploreOutput := map[string]any{
		"intent": "feature", "scope": "single-file",
		"modules_affected": []any{"internal/x"},
		"summary":          "implementar x",
	}
	outJSON, _ := json.Marshal(exploreOutput)
	_, err = pools.App.Exec(ctx,
		`UPDATE flow_run_steps
		 SET status='completed', outputs=$2, completed_at=NOW()
		 WHERE id=$1`, firstStepID, outJSON)
	require.NoError(t, err)

	// Process debería reanudar desde el step 2 (sdd-spec)
	err = s.ProcessAsyncFlowRun(ctx, res.FlowRunID)
	require.NoError(t, err)

	var flowStatus string
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT status FROM flow_runs WHERE id=$1`, res.FlowRunID,
	).Scan(&flowStatus))
	require.Equal(t, "completed", flowStatus)

	// Todos los steps completed
	rows, err := pools.App.Query(ctx,
		`SELECT status FROM flow_run_steps WHERE flow_run_id=$1`, res.FlowRunID)
	require.NoError(t, err)
	defer rows.Close()
	allCompleted := true
	for rows.Next() {
		var st string
		require.NoError(t, rows.Scan(&st))
		if st != "completed" {
			allCompleted = false
		}
	}
	require.True(t, allCompleted, "todos los steps deben estar completed tras reanudación")
}

// TestService_Async_WithStartingPhase_StartsFromPhase verifica que
// StartingPhase funciona en modo Async (delega a BuildFullPlan).
func TestService_Async_WithStartingPhase_StartsFromPhase(t *testing.T) {
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
		RawText:        "x",
		Mode:           orchestrator.ModeAsync,
		StartingPhase:  orchestrator.PhaseSlug("sdd-design"),
	})
	require.NoError(t, err)
	require.Len(t, res.Plan.Steps, 7, "debe arrancar desde sdd-design (10-3 anteriores)")
	require.Equal(t, "sdd-design", string(res.Plan.Steps[0].Slug))
}

// TestService_Async_WithSkipPhases_OmittedPhases verifica que SkipPhases
// funciona en modo Async (delega a BuildFullPlan).
func TestService_Async_WithSkipPhases_OmittedPhases(t *testing.T) {
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
		RawText:        "x",
		Mode:           orchestrator.ModeAsync,
		SkipPhases:     []orchestrator.PhaseSlug{"sdd-archive", "sdd-onboard"},
	})
	require.NoError(t, err)
	require.Len(t, res.Plan.Steps, 8, "10 - 2 skip = 8 (archive + onboard son sufijo válido)")
	// Verificar que el último step es sdd-judge (archive y onboard saltados)
	last := res.Plan.Steps[len(res.Plan.Steps)-1]
	require.Equal(t, "sdd-judge", string(last.Slug))
}

// TestService_Async_RequiresRepo verifica que ModeAsync sin Repo falla.
func TestService_Async_RequiresRepo(t *testing.T) {
	s := orchestrator.New(nil, nil, buildFullRegistry(), "dev")
	_, err := s.Run(context.Background(), orchestrator.OrchestrateInput{
		OrganizationID: uuid.New(),
		ProjectID:      uuid.New(),
		UserID:         uuid.New(),
		RawText:        "x",
		Mode:           orchestrator.ModeAsync,
	})
	require.ErrorContains(t, err, "Repo required for Async mode")
}

// TestService_Run_AsyncWithExpressMaxLines_Rejected_D6 es redundante
// vs service_test.go pero lo mantenemos para cubrir el case explícito.
func init() {
	// Asegurar que el time.Local no afecta asserts de duración
	time.Local = time.UTC
}
