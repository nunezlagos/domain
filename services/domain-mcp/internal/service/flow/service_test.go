package flow

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGenerateSlug_Basic(t *testing.T) {
	slug := generateSlug("Mi Flow de Prueba")
	if slug != "mi-flow-de-prueba" {
		t.Fatalf("expected 'mi-flow-de-prueba', got '%s'", slug)
	}
}

func TestGenerateSlug_SpecialChars(t *testing.T) {
	slug := generateSlug("Hello World! @#$% & More")
	if !strings.Contains(slug, "hello-world") {
		t.Fatalf("expected slug to contain 'hello-world', got '%s'", slug)
	}
}

func TestGenerateSlug_AlreadySlug(t *testing.T) {
	slug := generateSlug("customer-onboarding")
	if slug != "customer-onboarding" {
		t.Fatalf("expected 'customer-onboarding', got '%s'", slug)
	}
}

func TestGenerateSlug_EmptyFallback(t *testing.T) {
	slug := generateSlug("!!!")
	if slug != "flow" {
		t.Fatalf("expected 'flow' fallback, got '%s'", slug)
	}
}

func TestGenerateSlug_Truncation(t *testing.T) {
	long := strings.Repeat("a", 200)
	slug := generateSlug(long)
	if len(slug) > 100 {
		t.Fatalf("slug too long: %d chars", len(slug))
	}
}

func TestSpecValidate_StepIDRequired(t *testing.T) {
	spec := Spec{
		Version: 1,
		Steps: []Step{
			{ID: "", Type: "skill_run"},
		},
	}
	err := spec.Validate()
	if err == nil {
		t.Fatal("expected error for missing step id, got nil")
	}
}

func TestSpecValidate_StepTypeRequired(t *testing.T) {
	spec := Spec{
		Version: 1,
		Steps: []Step{
			{ID: "s1", Type: ""},
		},
	}
	err := spec.Validate()
	if err == nil {
		t.Fatal("expected error for empty step type, got nil")
	}
}

func TestSpecValidate_InvalidStepType(t *testing.T) {
	spec := Spec{
		Version: 1,
		Steps: []Step{
			{ID: "s1", Type: "invalid_type"},
		},
	}
	err := spec.Validate()
	if err == nil {
		t.Fatal("expected error for invalid step type, got nil")
	}
}

func TestSpecValidate_ValidStepTypes(t *testing.T) {
	types := []string{"agent_run", "skill_run", "http_request", "mem_save",
		"condition", "parallel", "wait_signal", "sub_flow"}
	for _, typ := range types {
		spec := Spec{
			Version: 1,
			Steps:   []Step{{ID: "s1", Type: typ}},
		}
		if err := spec.Validate(); err != nil {
			t.Fatalf("expected valid type '%s', got error: %v", typ, err)
		}
	}
}

func TestSpecValidate_DuplicateStepID(t *testing.T) {
	spec := Spec{
		Version: 1,
		Steps: []Step{
			{ID: "s1", Type: "skill_run"},
			{ID: "s1", Type: "mem_save"},
		},
	}
	err := spec.Validate()
	if err == nil {
		t.Fatal("expected error for duplicate step id, got nil")
	}
}

func TestSpecValidate_NoSteps(t *testing.T) {
	spec := Spec{
		Version: 1,
		Steps:   []Step{},
	}
	err := spec.Validate()
	if err == nil {
		t.Fatal("expected error for empty steps, got nil")
	}
}

func TestSpecValidate_VersionRequired(t *testing.T) {
	spec := Spec{
		Version: 0,
		Steps:   []Step{{ID: "s1", Type: "skill_run"}},
	}
	err := spec.Validate()
	if err == nil {
		t.Fatal("expected error for version 0, got nil")
	}
}

func TestStep_JSONRoundTrip(t *testing.T) {
	step := Step{
		ID:        "s1",
		Type:      "skill_run",
		Config:    map[string]any{"skill_slug": "validate-email"},
		DependsOn: []string{"s0"},
		OnError:   "fail",
		Retries:   2,
		TimeoutS:  30,
	}
	data, err := json.Marshal(step)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded Step
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.ID != "s1" {
		t.Fatalf("expected id s1, got %s", decoded.ID)
	}
	if decoded.Type != "skill_run" {
		t.Fatalf("expected type skill_run, got %s", decoded.Type)
	}
	if len(decoded.DependsOn) != 1 || decoded.DependsOn[0] != "s0" {
		t.Fatalf("expected depends_on [s0], got %v", decoded.DependsOn)
	}
}

func TestSpec_JSONRoundTrip(t *testing.T) {
	spec := Spec{
		Version: 1,
		Steps: []Step{
			{ID: "s1", Type: "skill_run", Config: map[string]any{"skill_slug": "validate-email"}},
			{ID: "s2", Type: "agent_run", Config: map[string]any{"agent_slug": "greeter", "input": "hello"}, DependsOn: []string{"s1"}},
			{ID: "s3", Type: "mem_save", Config: map[string]any{"content": "done"}, DependsOn: []string{"s2"}},
		},
	}
	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded Spec
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Version != 1 {
		t.Fatalf("expected version 1, got %d", decoded.Version)
	}
	if len(decoded.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(decoded.Steps))
	}
	if decoded.Steps[1].DependsOn[0] != "s1" {
		t.Fatalf("expected s2 depends_on s1, got %v", decoded.Steps[1].DependsOn)
	}
}

func TestSpec_YAMLRoundTrip(t *testing.T) {
	spec := Spec{
		Version: 1,
		Steps: []Step{
			{ID: "s1", Type: "skill_run", Config: map[string]any{"skill_slug": "validate-email"}},
			{ID: "s2", Type: "mem_save", Config: map[string]any{"content": "done"}, DependsOn: []string{"s1"}},
		},
	}
	yamlBytes, err := yaml.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal yaml: %v", err)
	}
	var decoded Spec
	if err := yaml.Unmarshal(yamlBytes, &decoded); err != nil {
		t.Fatalf("unmarshal yaml: %v", err)
	}
	if decoded.Version != 1 {
		t.Fatalf("expected version 1, got %d", decoded.Version)
	}
	if len(decoded.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(decoded.Steps))
	}
	if decoded.Steps[1].DependsOn[0] != "s1" {
		t.Fatalf("expected s2 depends_on s1, got %v", decoded.Steps[1].DependsOn)
	}
}

func TestSpecValidate_OnErrorUnknownStep(t *testing.T) {
	spec := Spec{
		Version: 1,
		Steps: []Step{
			{ID: "s1", Type: "skill_run", OnError: "nonexistent"},
		},
	}
	err := spec.Validate()
	if err == nil {
		t.Fatal("expected error for on_error unknown step, got nil")
	}
}

func TestSpecValidate_OnErrorValidRefs(t *testing.T) {
	spec := Spec{
		Version: 1,
		Steps: []Step{
			{ID: "s1", Type: "skill_run"},
			{ID: "s2", Type: "mem_save", DependsOn: []string{"s1"}, OnError: "s1"},
		},
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestSpecValidate_OnErrorFailContinue(t *testing.T) {
	// "fail" y "continue" son valores reservados, no step ids
	spec := Spec{
		Version: 1,
		Steps: []Step{
			{ID: "s1", Type: "skill_run", OnError: "fail"},
			{ID: "s2", Type: "skill_run", OnError: "continue"},
		},
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestSpecValidate_DAGComplexValid(t *testing.T) {
	spec := Spec{
		Version: 1,
		Steps: []Step{
			{ID: "validate", Type: "skill_run"},
			{ID: "enrich", Type: "agent_run", DependsOn: []string{"validate"}},
			{ID: "transform", Type: "skill_run", DependsOn: []string{"validate"}},
			{ID: "notify", Type: "mem_save", DependsOn: []string{"enrich", "transform"}},
		},
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("expected no error for complex valid DAG, got: %v", err)
	}
}
