package flow

import (
	"encoding/json"
	"testing"
)

func TestDiffSpecs_Identical(t *testing.T) {
	spec := Spec{Version: 1, Steps: []Step{{ID: "s1", Type: "agent_run"}}}
	data, _ := json.Marshal(spec)
	changes := diffSpecs(data, data)
	if len(changes) != 0 {
		t.Fatalf("expected 0 changes, got %d", len(changes))
	}
}

func TestDiffSpecs_AddedStep(t *testing.T) {
	from := Spec{Version: 1, Steps: []Step{{ID: "s1", Type: "agent_run"}}}
	to := Spec{Version: 2, Steps: []Step{{ID: "s1", Type: "agent_run"}, {ID: "s2", Type: "skill_run"}}}
	fromData, _ := json.Marshal(from)
	toData, _ := json.Marshal(to)
	changes := diffSpecs(fromData, toData)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Type != "added" {
		t.Fatalf("expected added, got %s", changes[0].Type)
	}
	if changes[0].Breaking {
		t.Fatal("expected added step to be non-breaking")
	}
	if changes[0].Path != "steps.s2" {
		t.Fatalf("expected path steps.s2, got %s", changes[0].Path)
	}
}

func TestDiffSpecs_RemovedStep(t *testing.T) {
	from := Spec{Version: 1, Steps: []Step{{ID: "s1", Type: "agent_run"}, {ID: "s2", Type: "skill_run"}}}
	to := Spec{Version: 2, Steps: []Step{{ID: "s1", Type: "agent_run"}}}
	fromData, _ := json.Marshal(from)
	toData, _ := json.Marshal(to)
	changes := diffSpecs(fromData, toData)
	found := false
	for _, c := range changes {
		if c.Type == "removed" {
			found = true
			if !c.Breaking {
				t.Fatal("expected removed step to be breaking")
			}
		}
	}
	if !found {
		t.Fatal("expected removed change")
	}
}

func TestDiffSpecs_ChangedType(t *testing.T) {
	from := Spec{Version: 1, Steps: []Step{{ID: "s1", Type: "agent_run"}}}
	to := Spec{Version: 2, Steps: []Step{{ID: "s1", Type: "skill_run"}}}
	fromData, _ := json.Marshal(from)
	toData, _ := json.Marshal(to)
	changes := diffSpecs(fromData, toData)
	foundModified := false
	for _, c := range changes {
		if c.Type == "modified" {
			foundModified = true
			if !c.Breaking {
				t.Fatal("expected type change to be breaking")
			}
		}
	}
	if !foundModified {
		t.Fatal("expected modified change")
	}
}

func TestDiffSpecs_RawFallback(t *testing.T) {
	from := json.RawMessage(`"old_string"`)
	to := json.RawMessage(`"new_string"`)
	changes := diffSpecs(from, to)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change for raw diff, got %d", len(changes))
	}
	if changes[0].Type != "modified" {
		t.Fatalf("expected modified, got %s", changes[0].Type)
	}
}

func TestDetectBreakingChanges_RemovedStep(t *testing.T) {
	diff := &FlowVersionDiff{
		FromVersion: 1, ToVersion: 2,
		Changes: []Change{{Type: "removed", Path: "steps.s1", Breaking: true}},
	}
	breaking := DetectBreakingChanges(diff)
	if len(breaking) != 1 {
		t.Fatalf("expected 1 breaking change, got %d", len(breaking))
	}
	if breaking[0].Severity != "major" {
		t.Fatalf("expected major, got %s", breaking[0].Severity)
	}
}

func TestDetectBreakingChanges_AddedStepIsNotBreaking(t *testing.T) {
	diff := &FlowVersionDiff{
		FromVersion: 1, ToVersion: 2,
		Changes: []Change{{Type: "added", Path: "steps.s2", Breaking: false}},
	}
	breaking := DetectBreakingChanges(diff)
	if len(breaking) != 0 {
		t.Fatalf("expected 0 breaking changes, got %d", len(breaking))
	}
}

func TestDetectBreakingChanges_Mixed(t *testing.T) {
	diff := &FlowVersionDiff{
		FromVersion: 1, ToVersion: 2,
		Changes: []Change{
			{Type: "added", Path: "steps.s3", Breaking: false},
			{Type: "removed", Path: "steps.s1", Breaking: true},
			{Type: "modified", Path: "steps.s2.type", OldValue: "agent_run", NewValue: "skill_run", Breaking: true},
		},
	}
	breaking := DetectBreakingChanges(diff)
	if len(breaking) != 2 {
		t.Fatalf("expected 2 breaking changes, got %d", len(breaking))
	}
}

func TestFlowVersionDiff_JSONRoundTrip(t *testing.T) {
	diff := &FlowVersionDiff{
		FromVersion: 1, ToVersion: 3,
		Changes: []Change{
			{Type: "added", Path: "steps.s2", Breaking: false},
			{Type: "removed", Path: "steps.s1", Breaking: true},
		},
	}
	data, err := json.Marshal(diff)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded FlowVersionDiff
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(decoded.Changes) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(decoded.Changes))
	}
	if decoded.FromVersion != 1 || decoded.ToVersion != 3 {
		t.Fatalf("version mismatch: %d -> %d", decoded.FromVersion, decoded.ToVersion)
	}
}

func TestBreakingChange_JSONRoundTrip(t *testing.T) {
	bc := BreakingChange{Description: "step removed", Severity: "major"}
	data, err := json.Marshal(bc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded BreakingChange
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Description != "step removed" || decoded.Severity != "major" {
		t.Fatal("roundtrip mismatch")
	}
}

func TestVersionStatusConstants(t *testing.T) {
	if VersionDraft != "draft" || VersionPublished != "published" || VersionDeprecated != "deprecated" {
		t.Fatal("status constants mismatch")
	}
}

func TestVersioningErrors(t *testing.T) {
	if ErrVersionAlreadyPublished == nil {
		t.Fatal("ErrVersionAlreadyPublished should not be nil")
	}
	if ErrVersionAlreadyDeprecated == nil {
		t.Fatal("ErrVersionAlreadyDeprecated should not be nil")
	}
	if ErrVersionDraftCannotInvoke == nil {
		t.Fatal("ErrVersionDraftCannotInvoke should not be nil")
	}
	if ErrVersionDeprecatedCannotInvoke == nil {
		t.Fatal("ErrVersionDeprecatedCannotInvoke should not be nil")
	}
}
