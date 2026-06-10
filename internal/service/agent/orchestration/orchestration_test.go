package orchestration_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/agent/orchestration"
)

// fakeConductor responde con outputs canned por agent slug.
type fakeConductor struct {
	responses map[string]string
	templates map[string]*orchestration.AgentTemplate
	fail      map[string]bool
}

func (f *fakeConductor) RunAgent(_ context.Context, slug string, _ orchestration.Task) (string, json.RawMessage, error) {
	if f.fail[slug] {
		return "", nil, errors.New("planned failure for " + slug)
	}
	out := f.responses[slug]
	return out, nil, nil
}

func (f *fakeConductor) LoadTemplate(_ context.Context, slug string) (*orchestration.AgentTemplate, error) {
	if t, ok := f.templates[slug]; ok {
		return t, nil
	}
	return nil, nil
}

func TestSequential_PipelinesOutputs(t *testing.T) {
	fc := &fakeConductor{responses: map[string]string{
		"a": "result-a", "b": "result-b", "c": "result-c",
	}}
	seq := &orchestration.Sequential{
		Conductor: fc,
		Steps: []orchestration.SequentialStep{
			{AgentSlug: "a"}, {AgentSlug: "b"}, {AgentSlug: "c"},
		},
	}
	res, err := seq.Run(context.Background(), []byte("start"))
	require.NoError(t, err)
	require.True(t, res.Successful)
	require.Len(t, res.Tasks, 3)
	require.Equal(t, "result-c", res.FinalOutput)
}

func TestSequential_StopOnFailure(t *testing.T) {
	fc := &fakeConductor{
		responses: map[string]string{"a": "ok"},
		fail:      map[string]bool{"b": true},
	}
	seq := &orchestration.Sequential{
		Conductor: fc,
		Steps: []orchestration.SequentialStep{
			{AgentSlug: "a"}, {AgentSlug: "b"}, {AgentSlug: "c"},
		},
	}
	_, err := seq.Run(context.Background(), []byte("start"))
	require.Error(t, err)
}

func TestParallelFanout_AllSuccess(t *testing.T) {
	fc := &fakeConductor{responses: map[string]string{"x": "X", "y": "Y", "z": "Z"}}
	p := &orchestration.ParallelFanout{
		Conductor: fc,
		Tasks: []orchestration.FanoutTask{
			{AgentSlug: "x"}, {AgentSlug: "y"}, {AgentSlug: "z"},
		},
	}
	res, err := p.Run(context.Background())
	require.NoError(t, err)
	require.True(t, res.Successful)
	require.Len(t, res.Tasks, 3)
}

func TestParallelFanout_PartialFailure(t *testing.T) {
	fc := &fakeConductor{
		responses: map[string]string{"x": "X", "y": "Y"},
		fail:      map[string]bool{"y": true},
	}
	p := &orchestration.ParallelFanout{
		Conductor: fc,
		Tasks: []orchestration.FanoutTask{
			{AgentSlug: "x"}, {AgentSlug: "y"},
		},
	}
	res, err := p.Run(context.Background())
	require.NoError(t, err)
	require.False(t, res.Successful)
}

func TestHandoff_DetectAndChain(t *testing.T) {
	fc := &fakeConductor{
		responses: map[string]string{
			"a": `Para responder esto necesitás expertise en X. <handoff to="b" reason="expert in X"/>`,
			"b": "Respuesta final desde B.",
		},
		templates: map[string]*orchestration.AgentTemplate{
			"b": {Slug: "b", HandoffPolicy: orchestration.HandoffAllow},
		},
	}
	h := &orchestration.Handoff{Conductor: fc}
	res, err := h.Run(context.Background(), "a", "pregunta inicial")
	require.NoError(t, err)
	require.Equal(t, "Respuesta final desde B.", res.FinalOutput)
	require.Len(t, res.Tasks, 2)
}

func TestHandoff_BlockedByPolicy(t *testing.T) {
	fc := &fakeConductor{
		responses: map[string]string{
			"a": `<handoff to="forbidden" reason="x"/>`,
		},
		templates: map[string]*orchestration.AgentTemplate{
			"forbidden": {Slug: "forbidden", HandoffPolicy: orchestration.HandoffForbid},
		},
	}
	h := &orchestration.Handoff{Conductor: fc}
	_, err := h.Run(context.Background(), "a", "x")
	require.ErrorIs(t, err, orchestration.ErrHandoffForbidden)
}

func TestBuildHierarchicalContext_ParentChain(t *testing.T) {
	root := uuid.New()
	mid := uuid.New()
	leaf := uuid.New()
	tasks := []orchestration.Task{
		{ID: root, Description: "root"},
		{ID: mid, Parent: &root, Description: "mid"},
		{ID: leaf, Parent: &mid, Description: "leaf"},
	}
	ctx := orchestration.BuildHierarchicalContext(tasks, leaf)
	require.Equal(t, 2, ctx.Depth)
	require.NotNil(t, ctx.ParentTaskID)
	require.Equal(t, mid, *ctx.ParentTaskID)
}

// Sabotaje: si supervisor LLM devuelve JSON inválido, el run debe terminar
// con output = raw text (no crashear, no loop infinito).
func TestSabotage_SupervisorInvalidJSON_TerminatesGracefully(t *testing.T) {
	fc := &fakeConductor{
		responses: map[string]string{"sup": "not json at all"},
	}
	sup := &orchestration.Supervisor{
		Conductor:      fc,
		SupervisorSlug: "sup",
		WorkerSlugs:    []string{"w1"},
		MaxIterations:  3,
	}
	res, err := sup.Run(context.Background(), "task")
	require.NoError(t, err)
	require.True(t, res.Successful)
	require.Equal(t, "not json at all", res.FinalOutput)
}
