//go:build integration

package flowrunner_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	flowrunner "nunezlagos/domain/internal/runner/flow"
	"nunezlagos/domain/internal/service/flow"
)

// withDLQ habilita la DLQ en el runner del fixture.
func withDLQ(f *fix) *flow.DLQStore {
	store := &flow.DLQStore{Pool: f.runner.Pool}
	f.runner.DLQ = store
	return store
}

// failingStep es un skill_run con slug inexistente: falla en todos los intentos.
func failingStep(id string, extra func(*flow.Step)) flow.Step {
	st := flow.Step{ID: id, Type: flow.StepTypeSkillRun,
		Config: map[string]any{"skill_slug": "no-existe", "args": map[string]any{}}}
	if extra != nil {
		extra(&st)
	}
	return st
}

// Escenario 7: fallo permanente (retries agotados, sin política) → DLQ.
func TestErrorPolicy_PermanentFailure_GoesToDLQ(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	dlq := withDLQ(f)

	fl, err := f.flows.Create(ctx, flow.CreateInput{
		OrganizationID: f.orgID, Slug: "dlq-flow", Name: "DLQ Flow",
		Spec: flow.Spec{Version: 1, Steps: []flow.Step{
			failingStep("s1", func(st *flow.Step) {
				st.Retry = &flow.StepRetryPolicy{MaxRetries: 2, Backoff: "fixed", FixedDelayMs: 10}
			}),
		}},
		ActorID: f.userID,
	})
	require.NoError(t, err)

	res, err := f.runner.Run(ctx, flowrunner.RunInput{FlowID: fl.ID, TriggeredBy: &f.userID})
	require.NoError(t, err)
	require.Equal(t, flowrunner.StatusFailed, res.Status)
	require.Contains(t, res.Error, "retry_count=2")

	entries, err := dlq.List(ctx, f.orgID, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "dlq-flow", entries[0].FlowSlug)
	require.Equal(t, "s1", entries[0].StepKey)
	require.Equal(t, 2, entries[0].RetryCount)
	require.Len(t, entries[0].Errors, 3, "un mensaje por intento (1 + 2 retries)")

	// Resolve saca la entry de la lista
	require.NoError(t, dlq.Resolve(ctx, f.orgID, entries[0].ID))
	entries, err = dlq.List(ctx, f.orgID, 10)
	require.NoError(t, err)
	require.Empty(t, entries)
	require.ErrorIs(t, dlq.Resolve(ctx, f.orgID, uuid.New()), flow.ErrDLQNotFound)
}

// Escenario 4: ignore_and_continue reemplaza el resultado con default_on_error.
func TestErrorPolicy_IgnoreAndContinue_UsesDefault(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	mkSkill(t, f, "ok-skill")

	fl, err := f.flows.Create(ctx, flow.CreateInput{
		OrganizationID: f.orgID, Slug: "ignore-flow", Name: "Ignore",
		Spec: flow.Spec{Version: 1, Steps: []flow.Step{
			failingStep("s1", func(st *flow.Step) {
				st.OnError = "ignore_and_continue"
				st.DefaultOnError = map[string]any{"fallback_value": "default"}
			}),
			{ID: "s2", Type: flow.StepTypeSkillRun,
				Config: map[string]any{"skill_slug": "ok-skill", "args": map[string]any{}}},
		}},
		ActorID: f.userID,
	})
	require.NoError(t, err)

	res, err := f.runner.Run(ctx, flowrunner.RunInput{FlowID: fl.ID, TriggeredBy: &f.userID})
	require.NoError(t, err)
	require.Equal(t, flowrunner.StatusCompleted, res.Status, "el flow NO se detiene")
	s1, ok := res.Outputs["s1"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "default", s1["fallback_value"], "resultado reemplazado por default_on_error")
	require.Contains(t, res.Outputs, "s2", "el siguiente step corre")
}

// Escenario 6: fallback_step se ejecuta y el flow continúa con su resultado.
func TestErrorPolicy_FallbackStep_Succeeds(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	mkSkill(t, f, "handle-error")

	fl, err := f.flows.Create(ctx, flow.CreateInput{
		OrganizationID: f.orgID, Slug: "fb-flow", Name: "FB",
		Spec: flow.Spec{Version: 1, Steps: []flow.Step{
			failingStep("s2", func(st *flow.Step) {
				st.OnError = "fallback_step"
				st.FallbackStep = &flow.Step{ID: "s2_fallback", Type: flow.StepTypeSkillRun,
					Config: map[string]any{"skill_slug": "handle-error", "args": map[string]any{}}}
			}),
		}},
		ActorID: f.userID,
	})
	require.NoError(t, err)

	res, err := f.runner.Run(ctx, flowrunner.RunInput{FlowID: fl.ID, TriggeredBy: &f.userID})
	require.NoError(t, err)
	require.Equal(t, flowrunner.StatusCompleted, res.Status)
	s2, ok := res.Outputs["s2"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, s2["fallback_used"])
	require.NotNil(t, s2["result"])
}

// Fallback que también falla sin política → abort + DLQ.
func TestErrorPolicy_FallbackFails_Aborts(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	dlq := withDLQ(f)

	fl, err := f.flows.Create(ctx, flow.CreateInput{
		OrganizationID: f.orgID, Slug: "fb-fail", Name: "FB Fail",
		Spec: flow.Spec{Version: 1, Steps: []flow.Step{
			failingStep("s1", func(st *flow.Step) {
				st.OnError = "fallback_step"
				fb := failingStep("s1_fb", nil)
				st.FallbackStep = &fb
			}),
		}},
		ActorID: f.userID,
	})
	require.NoError(t, err)

	res, err := f.runner.Run(ctx, flowrunner.RunInput{FlowID: fl.ID, TriggeredBy: &f.userID})
	require.NoError(t, err)
	require.Equal(t, flowrunner.StatusFailed, res.Status)
	require.Contains(t, res.Error, "fallback")

	entries, err := dlq.List(ctx, f.orgID, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

// Escenario 8: default_step_error_policy del flow aplica cuando el step no
// declara on_error; la política del step tiene prioridad.
func TestErrorPolicy_FlowDefaultApplied(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	mkSkill(t, f, "next-skill")

	spec := flow.Spec{
		Version:                1,
		DefaultStepErrorPolicy: "continue",
		Steps: []flow.Step{
			failingStep("s1", nil), // sin on_error → hereda continue del flow
			{ID: "s2", Type: flow.StepTypeSkillRun,
				Config: map[string]any{"skill_slug": "next-skill", "args": map[string]any{}}},
		},
	}
	fl, err := f.flows.Create(ctx, flow.CreateInput{
		OrganizationID: f.orgID, Slug: "default-pol", Name: "Default",
		Spec: spec, ActorID: f.userID,
	})
	require.NoError(t, err)

	res, err := f.runner.Run(ctx, flowrunner.RunInput{FlowID: fl.ID, TriggeredBy: &f.userID})
	require.NoError(t, err)
	require.Equal(t, flowrunner.StatusCompleted, res.Status,
		"default continue del flow debe aplicar al step sin on_error")
	require.Contains(t, res.Outputs, "s2")
}
