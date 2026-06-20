package flowrunner

import (
	"context"
	"testing"

	"nunezlagos/domain/internal/service/flow"
)

func TestEstimateTokens(t *testing.T) {
	cases := map[string]int{
		"":      0,
		"abcd":  1,
		"hello": 1,
		"twelve_chars": 3,
	}
	for in, want := range cases {
		if got := estimateTokens(in); got != want {
			t.Fatalf("%q: got %d, want %d", in, got, want)
		}
	}
}

func TestResolveTemplateSimple(t *testing.T) {
	inputs := map[string]any{"name": "Ana", "n": 42}
	cases := map[string]string{
		"hello {{inputs.name}}":            "hello Ana",
		"no templates here":                "no templates here",
		"{{inputs.n}} items, {{inputs.name}}": "42 items, Ana",
	}
	for in, want := range cases {
		if got := resolveTemplateSimple(in, inputs); got != want {
			t.Fatalf("%q: got %q, want %q", in, got, want)
		}
	}
}

func TestAnalyzeStep_AgentRun(t *testing.T) {
	step := &flow.Step{
		ID:   "s1",
		Type: flow.StepTypeAgentRun,
		Config: map[string]any{
			"agent_slug": "my-agent",
			"input":      "Hello {{inputs.name}}",
		},
	}
	inputs := map[string]any{"name": "World"}
	ps := analyzeStep(context.Background(), step, inputs, nil, [16]byte{})
	if ps.StepID != "s1" {
		t.Fatalf("step_id: %s", ps.StepID)
	}
	if ps.EstimatedTokIn == 0 {
		t.Fatal("expected non-zero tokens_in")
	}
	if ps.EstimatedCostUSD == 0 {
		t.Fatal("expected non-zero cost")
	}
}

func TestAnalyzeStep_Condition_StaticVsRuntime(t *testing.T) {
	static := &flow.Step{
		ID: "c1", Type: flow.StepTypeCondition,
		Config: map[string]any{"left": "{{inputs.x}}", "right": "5", "op": "=="},
	}
	ps := analyzeStep(context.Background(), static, nil, nil, [16]byte{})
	if ps.WillExecute != "yes" {
		t.Fatalf("static condition will_execute: %s", ps.WillExecute)
	}

	runtime := &flow.Step{
		ID: "c2", Type: flow.StepTypeCondition,
		Config: map[string]any{"left": "{{outputs.s1.x}}", "right": "5", "op": "=="},
	}
	ps = analyzeStep(context.Background(), runtime, nil, nil, [16]byte{})
	if ps.WillExecute != "depends_on_runtime" {
		t.Fatalf("runtime condition will_execute: %s", ps.WillExecute)
	}
}

func TestAnalyzeStep_SkillRun_WarnsSideEffects(t *testing.T) {
	step := &flow.Step{ID: "s", Type: flow.StepTypeSkillRun}
	ps := analyzeStep(context.Background(), step, nil, nil, [16]byte{})
	if len(ps.Warnings) == 0 {
		t.Fatal("expected side-effect warning")
	}
}
