//go:build integration

package flowrunner_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	flowrunner "nunezlagos/domain/internal/runner/flow"
	"nunezlagos/domain/internal/service/flow"
	skillsvc "nunezlagos/domain/internal/service/skill"
)

func mkSkill(t *testing.T, f *fix, slug string) {
	t.Helper()
	_, err := f.skills.Create(context.Background(), skillsvc.CreateInput{
		OrganizationID: f.orgID, Slug: slug, Name: slug,
		SkillType: skillsvc.TypePrompt, Content: "ok",
		InputSchema: map[string]any{"type": "object"},
		ActorID:     f.userID,
	})
	require.NoError(t, err)
}

// Ejecución lineal: 3 steps en orden, outputs de cada uno disponibles en
// el resultado (FlowContext acumula).
func TestFlow_LinearTraversal_ContextAccumulates(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	mkSkill(t, f, "lin-skill")

	steps := []flow.Step{}
	for _, id := range []string{"s1", "s2", "s3"} {
		steps = append(steps, flow.Step{ID: id, Type: flow.StepTypeSkillRun,
			Config: map[string]any{"skill_slug": "lin-skill", "args": map[string]any{}}})
	}
	fl, err := f.flows.Create(ctx, flow.CreateInput{
		OrganizationID: f.orgID, Slug: "linear-3", Name: "Linear",
		Spec: flow.Spec{Version: 1, Steps: steps}, ActorID: f.userID,
	})
	require.NoError(t, err)

	res, err := f.runner.Run(ctx, flowrunner.RunInput{FlowID: fl.ID, TriggeredBy: &f.userID})
	require.NoError(t, err)
	require.Equal(t, flowrunner.StatusCompleted, res.Status)
	for _, id := range []string{"s1", "s2", "s3"} {
		require.Contains(t, res.Outputs, id, "output de %s debe estar en el contexto final", id)
	}

	// Las 3 filas de steps completadas en orden
	rows, err := f.runner.Pool.Query(ctx, `
		SELECT step_key FROM flow_run_steps
		WHERE flow_run_id = $1 AND status = 'completed' ORDER BY created_at ASC`, res.RunID)
	require.NoError(t, err)
	defer rows.Close()
	var order []string
	for rows.Next() {
		var k string
		require.NoError(t, rows.Scan(&k))
		order = append(order, k)
	}
	require.Equal(t, []string{"s1", "s2", "s3"}, order)
}

// Diamante: parallel con 2 branches y step final que corre después.
func TestFlow_ParallelDiamond(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	mkSkill(t, f, "dia-skill")

	fl, err := f.flows.Create(ctx, flow.CreateInput{
		OrganizationID: f.orgID, Slug: "diamond", Name: "Diamond",
		Spec: flow.Spec{Version: 1, Steps: []flow.Step{
			{ID: "fan", Type: flow.StepTypeParallel, Config: map[string]any{
				"branches": []any{
					map[string]any{"id": "b1", "type": "skill_run",
						"config": map[string]any{"skill_slug": "dia-skill", "args": map[string]any{}}},
					map[string]any{"id": "b2", "type": "skill_run",
						"config": map[string]any{"skill_slug": "dia-skill", "args": map[string]any{}}},
				},
			}},
			{ID: "join", Type: flow.StepTypeSkillRun,
				Config: map[string]any{"skill_slug": "dia-skill", "args": map[string]any{}}},
		}},
		ActorID: f.userID,
	})
	require.NoError(t, err)

	res, err := f.runner.Run(ctx, flowrunner.RunInput{FlowID: fl.ID, TriggeredBy: &f.userID})
	require.NoError(t, err)
	require.Equal(t, flowrunner.StatusCompleted, res.Status)
	require.Contains(t, res.Outputs, "fan")
	require.Contains(t, res.Outputs, "join")

	fanOut, ok := res.Outputs["fan"].(map[string]any)
	require.True(t, ok)
	results, ok := fanOut["results"].([]any)
	require.True(t, ok)
	require.Len(t, results, 2, "ambas branches del diamante deben producir resultado")
}

// Cancel en medio de step: un wait largo se cancela vía CancelRun y el run
// termina cancelled/failed sin completar el step.
func TestFlow_CancelMidStep(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	f.runner.Signals = &flow.SignalStore{Pool: f.runner.Pool}

	fl, err := f.flows.Create(ctx, flow.CreateInput{
		OrganizationID: f.orgID, Slug: "cancel-mid", Name: "Cancel Mid",
		Spec: flow.Spec{Version: 1, Steps: []flow.Step{
			{ID: "wait1", Type: flow.StepTypeWaitSignal,
				Config: map[string]any{"signal_name": "never", "timeout_seconds": float64(30)}},
		}},
		ActorID: f.userID,
	})
	require.NoError(t, err)

	resCh := make(chan *flowrunner.RunResult, 1)
	go func() {
		res, _ := f.runner.Run(ctx, flowrunner.RunInput{FlowID: fl.ID, TriggeredBy: &f.userID})
		resCh <- res
	}()

	runID := findRunID(t, f, fl.ID)
	waitRunStatus(t, f, runID, "paused_awaiting_signal", 5*time.Second)
	require.NoError(t, f.runner.CancelRun(runID))

	select {
	case res := <-resCh:
		require.NotNil(t, res)
		require.NotEqual(t, flowrunner.StatusCompleted, res.Status,
			"run cancelado a mitad de step no debe completar")
	case <-time.After(10 * time.Second):
		t.Fatal("cancel no interrumpió el step en curso")
	}
}
