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
	skillsvc "nunezlagos/domain/internal/service/skill"
)

// TestStepRow_LifecycleAndProgress — escenario 1 issue-09.10: fila running al
// iniciar, Beat actualiza progress + heartbeat, evento publicado vía NOTIFY,
// fila completed al cerrar.
func TestStepRow_LifecycleAndProgress(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()

	fl, err := f.flows.Create(ctx, flow.CreateInput{
		OrganizationID: f.orgID, Slug: "hb-flow", Name: "HB",
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

	// Suscribirse al canal de progreso ANTES del beat
	conn, err := f.runner.Pool.Acquire(ctx)
	require.NoError(t, err)
	defer func() {
		_ = conn.Conn().Close(context.Background())
		conn.Release()
	}()
	_, err = conn.Exec(ctx, "LISTEN "+flow.ProgressChannel)
	require.NoError(t, err)

	rowID := flowrunner.BeginStepRowForTest(ctx, f.runner, runID, "s1")
	require.NotEqual(t, uuid.Nil, rowID)

	hb := &flowrunner.StepHeartbeater{
		Store: &flow.HeartbeatStore{Pool: f.runner.Pool},
		RunID: runID, StepRowID: rowID, StepKey: "s1",
	}
	require.NoError(t, hb.Beat(ctx, 0.3, "downloaded 30%"))

	var progress float64
	var message string
	require.NoError(t, f.runner.Pool.QueryRow(ctx,
		`SELECT progress, progress_message FROM flow_run_steps WHERE id = $1`,
		rowID).Scan(&progress, &message))
	require.InDelta(t, 0.3, progress, 0.001)
	require.Equal(t, "downloaded 30%", message)

	// Evento NOTIFY recibido con payload correcto (hb-004)
	nctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	n, err := conn.Conn().WaitForNotification(nctx)
	require.NoError(t, err, "NOTIFY de progreso no recibido")
	var ev flow.ProgressEvent
	require.NoError(t, json.Unmarshal([]byte(n.Payload), &ev))
	require.Equal(t, "s1", ev.StepKey)
	require.InDelta(t, 0.3, ev.Progress, 0.001)

	require.NoError(t, flowrunner.CompleteStepRowForTest(ctx, f.runner, rowID, map[string]any{"ok": true}, nil))
	var status string
	require.NoError(t, f.runner.Pool.QueryRow(ctx,
		`SELECT status FROM flow_run_steps WHERE id = $1`, rowID).Scan(&status))
	require.Equal(t, "completed", status)
}

// TestFlow_Run_CreatesStepRows — el loop del runner crea filas por step y las
// cierra completed con heartbeat inicial.
func TestFlow_Run_CreatesStepRows(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()

	_, err := f.skills.Create(ctx, skillsvc.CreateInput{
		OrganizationID: f.orgID, Slug: "hb-skill", Name: "HB Skill",
		SkillType: skillsvc.TypePrompt, Content: "ok",
		InputSchema: map[string]any{"type": "object"},
		ActorID:     f.userID,
	})
	require.NoError(t, err)

	fl, err := f.flows.Create(ctx, flow.CreateInput{
		OrganizationID: f.orgID, Slug: "hb-rows", Name: "HB Rows",
		Spec: flow.Spec{Version: 1, Steps: []flow.Step{
			{ID: "s1", Type: flow.StepTypeSkillRun,
				Config: map[string]any{"skill_slug": "hb-skill", "args": map[string]any{}}},
		}},
		ActorID: f.userID,
	})
	require.NoError(t, err)

	res, err := f.runner.Run(ctx, flowrunner.RunInput{FlowID: fl.ID, TriggeredBy: &f.userID})
	require.NoError(t, err)
	require.Equal(t, flowrunner.StatusCompleted, res.Status)

	var status string
	var startedAt, completedAt, lastHB *time.Time
	require.NoError(t, f.runner.Pool.QueryRow(ctx, `
		SELECT status, started_at, completed_at, last_heartbeat_at
		FROM flow_run_steps WHERE flow_run_id = $1 AND step_key = 's1'`,
		res.RunID).Scan(&status, &startedAt, &completedAt, &lastHB))
	require.Equal(t, "completed", status)
	require.NotNil(t, startedAt)
	require.NotNil(t, completedAt)
	require.NotNil(t, lastHB)
}
