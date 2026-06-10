package orchestrator

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRebuildOutputFromStepInputs_PreservesD5Contract(t *testing.T) {
	t.Parallel()
	step := &FlowRunStepRow{
		Inputs: map[string]any{
			"suggested_saves": []any{
				map[string]any{
					"type":     "code_reference",
					"required": true,
					"hint":     "save the file modified",
				},
				map[string]any{
					"type":     "knowledge_doc",
					"required": false,
					"hint":     "",
				},
			},
		},
	}
	out := rebuildOutputFromStepInputs(step)
	require.Len(t, out.SuggestedSaves, 2)
	require.Equal(t, "code_reference", out.SuggestedSaves[0].Type)
	require.True(t, out.SuggestedSaves[0].Required)
	require.Equal(t, "save the file modified", out.SuggestedSaves[0].Hint)
	require.Equal(t, "knowledge_doc", out.SuggestedSaves[1].Type)
	require.False(t, out.SuggestedSaves[1].Required)
}

func TestRebuildOutputFromStepInputs_EmptyInputs(t *testing.T) {
	t.Parallel()
	step := &FlowRunStepRow{Inputs: map[string]any{}}
	out := rebuildOutputFromStepInputs(step)
	require.Empty(t, out.SuggestedSaves)
}

func TestRebuildOutputFromStepInputs_MalformedEntriesSkipped(t *testing.T) {
	t.Parallel()
	step := &FlowRunStepRow{
		Inputs: map[string]any{
			"suggested_saves": []any{
				"not-a-map",
				123,
				map[string]any{"type": "valid", "required": true},
			},
		},
	}
	out := rebuildOutputFromStepInputs(step)
	require.Len(t, out.SuggestedSaves, 1, "los entries malformados se ignoran sin crash")
	require.Equal(t, "valid", out.SuggestedSaves[0].Type)
}

func TestAggregateFlowStatus_AllPending(t *testing.T) {
	t.Parallel()
	id1 := uuid.New()
	steps := []FlowRunStepRow{
		{ID: id1, StepKey: "sdd-apply", Status: "pending"},
		{ID: uuid.New(), StepKey: "sdd-verify", Status: "pending"},
	}
	status, next, key := aggregateFlowStatus(steps)
	require.Equal(t, "running", status)
	require.NotNil(t, next)
	require.Equal(t, id1, *next)
	require.Equal(t, "sdd-apply", key)
}

func TestAggregateFlowStatus_AllCompleted(t *testing.T) {
	t.Parallel()
	steps := []FlowRunStepRow{
		{ID: uuid.New(), Status: "completed"},
		{ID: uuid.New(), Status: "completed"},
	}
	status, next, _ := aggregateFlowStatus(steps)
	require.Equal(t, "completed", status)
	require.Nil(t, next)
}

func TestAggregateFlowStatus_AnyFailed(t *testing.T) {
	t.Parallel()
	steps := []FlowRunStepRow{
		{ID: uuid.New(), Status: "completed"},
		{ID: uuid.New(), Status: "failed"},
		{ID: uuid.New(), Status: "pending"},
	}
	status, next, _ := aggregateFlowStatus(steps)
	require.Equal(t, "failed", status, "cualquier failed → flow failed (sin importar pending posteriores)")
	require.Nil(t, next, "flow failed no expone next step")
}

func TestAggregateFlowStatus_PartialProgress(t *testing.T) {
	t.Parallel()
	id2 := uuid.New()
	steps := []FlowRunStepRow{
		{ID: uuid.New(), StepKey: "sdd-apply", Status: "completed"},
		{ID: id2, StepKey: "sdd-verify", Status: "pending"},
	}
	status, next, key := aggregateFlowStatus(steps)
	require.Equal(t, "running", status)
	require.NotNil(t, next)
	require.Equal(t, id2, *next)
	require.Equal(t, "sdd-verify", key)
}

func TestAggregateFlowStatus_SkippedCountsAsTerminal(t *testing.T) {
	t.Parallel()
	steps := []FlowRunStepRow{
		{ID: uuid.New(), Status: "completed"},
		{ID: uuid.New(), Status: "skipped"},
	}
	status, _, _ := aggregateFlowStatus(steps)
	require.Equal(t, "completed", status,
		"skipped + completed = flow completed (skipped es terminal igual que cancelled)")
}
