//go:build integration

package flowrunner_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	flowrunner "nunezlagos/domain/internal/runner/flow"
	"nunezlagos/domain/internal/service/flow"
)

// waitRunStatus polls flow_runs.status hasta que matchee o expire el deadline.
func waitRunStatus(t *testing.T, f *fix, runID uuid.UUID, want string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var status string
		err := f.runner.Pool.QueryRow(context.Background(),
			`SELECT status FROM flow_runs WHERE id = $1`, runID).Scan(&status)
		if err == nil && status == want {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("run %s nunca llegó a status %q", runID, want)
}

// findRunID localiza el último run del flow (el Run corre en goroutine).
func findRunID(t *testing.T, f *fix, flowID uuid.UUID) uuid.UUID {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var id uuid.UUID
		err := f.runner.Pool.QueryRow(context.Background(),
			`SELECT id FROM flow_runs WHERE flow_id = $1 ORDER BY created_at DESC LIMIT 1`,
			flowID).Scan(&id)
		if err == nil {
			return id
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("flow_run nunca creado")
	return uuid.Nil
}

// TestFlow_WaitSignal_DeliveredResumes — escenarios 1+2 issue-09.8:
// el run pausa en paused_awaiting_signal y la señal con payload lo resume.
func TestFlow_WaitSignal_DeliveredResumes(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	signals := &flow.SignalStore{Pool: f.runner.Pool}
	f.runner.Signals = signals

	fl, err := f.flows.Create(ctx, flow.CreateInput{
		OrganizationID: f.orgID, Slug: "await-approval", Name: "Await Approval",
		Spec: flow.Spec{Version: 1, Steps: []flow.Step{
			{ID: "wait1", Type: flow.StepTypeWaitSignal,
				Config: map[string]any{"signal_name": "approval_received", "timeout_seconds": float64(15)}},
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


	pending, err := signals.HasPendingExpectation(ctx, runID, "approval_received")
	require.NoError(t, err)
	require.True(t, pending)

	stepKey := "wait1"
	payload, _ := json.Marshal(map[string]any{"approved": true, "by": "alice"})
	_, err = signals.Send(ctx, runID, &stepKey, "approval_received", payload)
	require.NoError(t, err)

	select {
	case res := <-resCh:
		require.NotNil(t, res)
		require.Equal(t, flowrunner.StatusCompleted, res.Status)
		out, ok := res.Outputs["wait1"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "approval_received", out["signal"])
		p, ok := out["payload"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, true, p["approved"])
	case <-time.After(10 * time.Second):
		t.Fatal("run no completó tras la señal")
	}
}

// TestFlow_WaitSignal_Timeout — escenario 4: sin señal, el step falla con
// SignalTimeout y el run termina failed.
func TestFlow_WaitSignal_Timeout(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	f.runner.Signals = &flow.SignalStore{Pool: f.runner.Pool}

	fl, err := f.flows.Create(ctx, flow.CreateInput{
		OrganizationID: f.orgID, Slug: "await-timeout", Name: "Await Timeout",
		Spec: flow.Spec{Version: 1, Steps: []flow.Step{
			{ID: "wait1", Type: flow.StepTypeWaitSignal,
				Config: map[string]any{"signal_name": "never_arrives", "timeout_seconds": float64(1)}},
		}},
		ActorID: f.userID,
	})
	require.NoError(t, err)

	res, _ := f.runner.Run(ctx, flowrunner.RunInput{FlowID: fl.ID, TriggeredBy: &f.userID})
	require.NotNil(t, res)
	require.Equal(t, flowrunner.StatusFailed, res.Status)
	require.Contains(t, res.Error, "signal timeout")
}

// TestSignal_Broadcast_TwoRuns — escenario 5: BroadcastSignal entrega a
// todos los runs esperando la misma señal.
func TestSignal_Broadcast_TwoRuns(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	signals := &flow.SignalStore{Pool: f.runner.Pool}
	f.runner.Signals = signals

	mkFlow := func(slug string) uuid.UUID {
		fl, err := f.flows.Create(ctx, flow.CreateInput{
			OrganizationID: f.orgID, Slug: slug, Name: slug,
			Spec: flow.Spec{Version: 1, Steps: []flow.Step{
				{ID: "wait1", Type: flow.StepTypeWaitSignal,
					Config: map[string]any{"signal_name": "global_pause", "timeout_seconds": float64(15)}},
			}},
			ActorID: f.userID,
		})
		require.NoError(t, err)
		return fl.ID
	}
	flowA, flowB := mkFlow("bcast-a"), mkFlow("bcast-b")

	resCh := make(chan string, 2)
	for _, fid := range []uuid.UUID{flowA, flowB} {
		go func(id uuid.UUID) {
			res, _ := f.runner.Run(ctx, flowrunner.RunInput{FlowID: id, TriggeredBy: &f.userID})
			if res != nil {
				resCh <- res.Status
			} else {
				resCh <- "nil"
			}
		}(fid)
	}

	runA, runB := findRunID(t, f, flowA), findRunID(t, f, flowB)
	waitRunStatus(t, f, runA, "paused_awaiting_signal", 5*time.Second)
	waitRunStatus(t, f, runB, "paused_awaiting_signal", 5*time.Second)

	n, err := signals.BroadcastSignal(ctx, "global_pause", []byte(`{"reason":"maintenance"}`))
	require.NoError(t, err)
	require.Equal(t, 2, n)

	for i := 0; i < 2; i++ {
		select {
		case status := <-resCh:
			require.Equal(t, flowrunner.StatusCompleted, status)
		case <-time.After(10 * time.Second):
			t.Fatal("broadcast no resumió todos los runs")
		}
	}
}

// TestSignal_EarlySignal_DeliveredOnWait — señal enviada antes del LISTEN
// se consume en el intento inmediato de WaitNotify.
func TestSignal_EarlySignal_DeliveredOnWait(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	signals := &flow.SignalStore{Pool: f.runner.Pool}


	fl, err := f.flows.Create(ctx, flow.CreateInput{
		OrganizationID: f.orgID, Slug: "early-sig", Name: "Early",
		Spec: flow.Spec{Version: 1, Steps: []flow.Step{
			{ID: "s1", Type: flow.StepTypeCondition, Config: map[string]any{"expression": "1 == 1"}},
		}},
		ActorID: f.userID,
	})
	require.NoError(t, err)
	var runID uuid.UUID
	require.NoError(t, f.runner.Pool.QueryRow(ctx, `
		INSERT INTO flow_runs (organization_id, flow_id, trigger_type, status, inputs)
		VALUES ($1, $2, 'manual', 'running', '{}') RETURNING id`,
		f.orgID, fl.ID).Scan(&runID))

	stepKey := "wait1"
	_, err = signals.Send(ctx, runID, &stepKey, "early_event", []byte(`{"n":1}`))
	require.NoError(t, err)

	sig, err := signals.WaitNotify(ctx, runID, &stepKey, "early_event", 2*time.Second)
	require.NoError(t, err)
	require.Equal(t, "early_event", sig.Name)
	require.NotNil(t, sig.DeliveredAt)
}

// TestSignal_HasPendingExpectation_Expired — expectativa expirada no cuenta
// (base del 409 del endpoint POST /runs/:id/signals).
func TestSignal_HasPendingExpectation_Expired(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	signals := &flow.SignalStore{Pool: f.runner.Pool}

	fl, err := f.flows.Create(ctx, flow.CreateInput{
		OrganizationID: f.orgID, Slug: "pending-exp", Name: "Pending",
		Spec: flow.Spec{Version: 1, Steps: []flow.Step{
			{ID: "s1", Type: flow.StepTypeCondition, Config: map[string]any{"expression": "1 == 1"}},
		}},
		ActorID: f.userID,
	})
	require.NoError(t, err)
	var runID uuid.UUID
	require.NoError(t, f.runner.Pool.QueryRow(ctx, `
		INSERT INTO flow_runs (organization_id, flow_id, trigger_type, status, inputs)
		VALUES ($1, $2, 'manual', 'running', '{}') RETURNING id`,
		f.orgID, fl.ID).Scan(&runID))

	pending, err := signals.HasPendingExpectation(ctx, runID, "approve")
	require.NoError(t, err)
	require.False(t, pending, "sin ExpectSignal no debe haber pending")

	_, err = signals.ExpectSignal(ctx, runID, "s1", "approve", 50*time.Millisecond)
	require.NoError(t, err)
	pending, err = signals.HasPendingExpectation(ctx, runID, "approve")
	require.NoError(t, err)
	require.True(t, pending)

	time.Sleep(100 * time.Millisecond)
	pending, err = signals.HasPendingExpectation(ctx, runID, "approve")
	require.NoError(t, err)
	require.False(t, pending, "expectativa expirada no debe contar")
}
