package flow

import (
	"errors"
	"testing"
)

func TestValidateDAG_Acyclic(t *testing.T) {
	steps := []Step{
		{ID: "s1", Type: "skill_run"},
		{ID: "s2", Type: "skill_run", DependsOn: []string{"s1"}},
		{ID: "s3", Type: "skill_run", DependsOn: []string{"s2"}},
	}
	if err := ValidateDAG(steps); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateDAG_AcyclicComplex(t *testing.T) {
	steps := []Step{
		{ID: "s1", Type: "skill_run"},
		{ID: "s2", Type: "skill_run", DependsOn: []string{"s1"}},
		{ID: "s3", Type: "skill_run", DependsOn: []string{"s1"}},
		{ID: "s4", Type: "condition", DependsOn: []string{"s2", "s3"}},
		{ID: "s5", Type: "skill_run", DependsOn: []string{"s4"}},
		{ID: "s6", Type: "skill_run", DependsOn: []string{"s4"}},
		{ID: "s7", Type: "parallel", DependsOn: []string{"s5"}},
		{ID: "s8", Type: "mem_save", DependsOn: []string{"s6", "s7"}},
	}
	if err := ValidateDAG(steps); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateDAG_NoDeps(t *testing.T) {
	steps := []Step{
		{ID: "s1", Type: "skill_run"},
		{ID: "s2", Type: "skill_run"},
		{ID: "s3", Type: "skill_run"},
	}
	if err := ValidateDAG(steps); err != nil {
		t.Fatalf("expected no error for independent steps, got: %v", err)
	}
}

func TestValidateDAG_CycleSimple(t *testing.T) {
	steps := []Step{
		{ID: "a", Type: "skill_run", DependsOn: []string{"b"}},
		{ID: "b", Type: "skill_run", DependsOn: []string{"c"}},
		{ID: "c", Type: "skill_run", DependsOn: []string{"a"}},
	}
	err := ValidateDAG(steps)
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	if !errors.Is(err, ErrCycleDetected) {
		t.Fatalf("expected ErrCycleDetected, got: %v", err)
	}
}

func TestValidateDAG_CycleSelfRef(t *testing.T) {
	steps := []Step{
		{ID: "a", Type: "skill_run", DependsOn: []string{"a"}},
	}
	err := ValidateDAG(steps)
	if err == nil {
		t.Fatal("expected cycle error for self-reference, got nil")
	}
	if !errors.Is(err, ErrCycleDetected) {
		t.Fatalf("expected ErrCycleDetected, got: %v", err)
	}
}

func TestValidateDAG_DependsOnUnknown(t *testing.T) {
	steps := []Step{
		{ID: "s1", Type: "skill_run", DependsOn: []string{"nonexistent"}},
	}
	err := ValidateDAG(steps)
	if err == nil {
		t.Fatal("expected error for unknown depends_on, got nil")
	}
	if errors.Is(err, ErrCycleDetected) {
		t.Fatalf("expected ErrSpecInvalid for unknown ref, got cycle error")
	}
}

func TestValidateDAG_DisconnectedComponents(t *testing.T) {
	steps := []Step{
		{ID: "a1", Type: "skill_run"},
		{ID: "a2", Type: "skill_run", DependsOn: []string{"a1"}},
		{ID: "b1", Type: "skill_run"},
		{ID: "b2", Type: "skill_run", DependsOn: []string{"b1"}},
	}
	if err := ValidateDAG(steps); err != nil {
		t.Fatalf("expected no error for disconnected DAGs, got: %v", err)
	}
}

func TestTopologicalSort_Simple(t *testing.T) {
	steps := []Step{
		{ID: "s1", Type: "skill_run"},
		{ID: "s2", Type: "skill_run", DependsOn: []string{"s1"}},
		{ID: "s3", Type: "skill_run", DependsOn: []string{"s2"}},
	}
	sorted, err := TopologicalSort(steps)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(sorted) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(sorted))
	}
	if sorted[0].ID != "s1" || sorted[1].ID != "s2" || sorted[2].ID != "s3" {
		t.Fatalf("expected order s1,s2,s3, got: %v", stepIDs(sorted))
	}
}

func TestTopologicalSort_FanOut(t *testing.T) {
	steps := []Step{
		{ID: "root", Type: "skill_run"},
		{ID: "left", Type: "skill_run", DependsOn: []string{"root"}},
		{ID: "right", Type: "skill_run", DependsOn: []string{"root"}},
		{ID: "join", Type: "mem_save", DependsOn: []string{"left", "right"}},
	}
	sorted, err := TopologicalSort(steps)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(sorted) != 4 {
		t.Fatalf("expected 4 steps, got %d", len(sorted))
	}
	pos := map[string]int{}
	for i, s := range sorted {
		pos[s.ID] = i
	}
	if pos["root"] >= pos["left"] || pos["root"] >= pos["right"] {
		t.Fatal("root must come before left and right")
	}
	if pos["left"] >= pos["join"] || pos["right"] >= pos["join"] {
		t.Fatal("left and right must come before join")
	}
}

func TestTopologicalSort_CycleError(t *testing.T) {
	steps := []Step{
		{ID: "a", DependsOn: []string{"b"}},
		{ID: "b", DependsOn: []string{"a"}},
	}
	_, err := TopologicalSort(steps)
	if err == nil {
		t.Fatal("expected error for cycle, got nil")
	}
}

func TestSpecValidate_IntegratesDAG(t *testing.T) {

	spec := Spec{
		Version: 1,
		Steps: []Step{
			{ID: "a", Type: "skill_run", DependsOn: []string{"b"}},
			{ID: "b", Type: "skill_run", DependsOn: []string{"a"}},
		},
	}
	err := spec.Validate()
	if err == nil {
		t.Fatal("expected validation error for cycle, got nil")
	}
}

func TestSpecValidate_DAGOK(t *testing.T) {
	spec := Spec{
		Version: 1,
		Steps: []Step{
			{ID: "s1", Type: "skill_run"},
			{ID: "s2", Type: "skill_run", DependsOn: []string{"s1"}},
		},
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func stepIDs(steps []Step) []string {
	ids := make([]string, len(steps))
	for i, s := range steps {
		ids[i] = s.ID
	}
	return ids
}
