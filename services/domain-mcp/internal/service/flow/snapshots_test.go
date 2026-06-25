package flow

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCompareForReplay_Identical(t *testing.T) {
	output := json.RawMessage(`{"result":"ok"}`)
	original := &StepSnapshot{
		StepKey: "step-1",
		Output:  output,
	}
	replay := &StepSnapshot{
		StepKey: "step-1",
		Output:  output,
	}

	match, reason := CompareForReplay(original, replay)
	if !match {
		t.Fatalf("expected match, got: %s", reason)
	}
}

func TestCompareForReplay_DifferentStepKey(t *testing.T) {
	original := &StepSnapshot{StepKey: "step-1", Output: json.RawMessage(`"a"`)}
	replay := &StepSnapshot{StepKey: "step-2", Output: json.RawMessage(`"a"`)}

	match, reason := CompareForReplay(original, replay)
	if match {
		t.Fatal("expected mismatch for different step_key")
	}
	if reason != "step_key differs" {
		t.Fatalf("expected 'step_key differs', got %s", reason)
	}
}

func TestCompareForReplay_DifferentErrorStatus(t *testing.T) {
	errStr := "error"
	original := &StepSnapshot{StepKey: "s", Output: json.RawMessage(`"a"`)}
	replay := &StepSnapshot{StepKey: "s", Output: json.RawMessage(`"a"`), Error: &errStr}

	match, reason := CompareForReplay(original, replay)
	if match {
		t.Fatal("expected mismatch for different error status")
	}
	if reason != "error status differs" {
		t.Fatalf("expected 'error status differs', got %s", reason)
	}
}

func TestCompareForReplay_DifferentErrorMessage(t *testing.T) {
	err1 := "timeout"
	err2 := "crash"
	original := &StepSnapshot{StepKey: "s", Output: json.RawMessage(`"a"`), Error: &err1}
	replay := &StepSnapshot{StepKey: "s", Output: json.RawMessage(`"a"`), Error: &err2}

	match, reason := CompareForReplay(original, replay)
	if match {
		t.Fatal("expected mismatch for different error message")
	}
	if reason != "error message differs: "+err1+" vs "+err2 {
		t.Fatalf("unexpected reason: %s", reason)
	}
}

func TestCompareForReplay_DifferentOutput(t *testing.T) {
	original := &StepSnapshot{StepKey: "s", Output: json.RawMessage(`"a"`)}
	replay := &StepSnapshot{StepKey: "s", Output: json.RawMessage(`"b"`)}

	match, reason := CompareForReplay(original, replay)
	if match {
		t.Fatal("expected mismatch for different output")
	}
	if reason != "output differs" {
		t.Fatalf("expected 'output differs', got %s", reason)
	}
}

func TestCompareForReplay_BothNilError(t *testing.T) {
	original := &StepSnapshot{StepKey: "s", Output: json.RawMessage(`"a"`)}
	replay := &StepSnapshot{StepKey: "s", Output: json.RawMessage(`"a"`)}

	match, reason := CompareForReplay(original, replay)
	if !match {
		t.Fatalf("expected match, got %s", reason)
	}
}

func TestCompareForReplay_BothSameError(t *testing.T) {
	err := "error"
	original := &StepSnapshot{StepKey: "s", Output: json.RawMessage(`"a"`), Error: &err}
	replay := &StepSnapshot{StepKey: "s", Output: json.RawMessage(`"a"`), Error: &err}

	match, reason := CompareForReplay(original, replay)
	if !match {
		t.Fatalf("expected match, got %s", reason)
	}
}

func TestStepSnapshot_JSONRoundTrip(t *testing.T) {
	snap := &StepSnapshot{
		ID:         uuid.New(),
		StepID:     uuid.New(),
		RunID:      uuid.New(),
		StepKey:    "test_step",
		Input:      json.RawMessage(`{"key":"value"}`),
		Output:     json.RawMessage(`{"result":"ok"}`),
		DurationMs: 1234,
		CapturedAt: time.Now().UTC(),
	}
	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded StepSnapshot
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.StepKey != "test_step" {
		t.Fatalf("step_key mismatch")
	}
	if decoded.DurationMs != 1234 {
		t.Fatalf("duration mismatch")
	}
}

func TestSaveSnapshot_WithCompression(t *testing.T) {
	store := &SnapshotStore{}
	snap := &StepSnapshot{
		ID:      uuid.New(),
		StepID:  uuid.New(),
		RunID:   uuid.New(),
		StepKey: "test",
		Input:   json.RawMessage(`{"input":true}`),
		Output:  json.RawMessage(`{"result":"large data repeated large data repeated"}`),
	}

	mockCompress := func(input any) ([]byte, int, error) {
		return []byte("compressed-data"), 50, nil
	}




	err := store.SaveSnapshot(context.Background(), snap, mockCompress)
	if err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}
}

func TestSaveSnapshot_NoCompressFn(t *testing.T) {
	store := &SnapshotStore{}
	snap := &StepSnapshot{
		ID:      uuid.New(),
		StepID:  uuid.New(),
		RunID:   uuid.New(),
		StepKey: "test",
		Input:   json.RawMessage(`{}`),
		Output:  json.RawMessage(`{"result":"ok"}`),
	}

	err := store.SaveSnapshot(context.Background(), snap, nil)
	if err != nil {
		t.Fatalf("SaveSnapshot with nil compressFn: %v", err)
	}
}

func TestSaveSnapshot_NoOutput(t *testing.T) {
	store := &SnapshotStore{}
	snap := &StepSnapshot{
		ID:      uuid.New(),
		StepID:  uuid.New(),
		RunID:   uuid.New(),
		StepKey: "test",
		Input:   json.RawMessage(`{}`),
	}

	err := store.SaveSnapshot(context.Background(), snap, func(any) ([]byte, int, error) {
		return nil, 0, nil
	})
	if err != nil {
		t.Fatalf("SaveSnapshot with nil output: %v", err)
	}
}

func TestSnapshotOutputCompressed_Wrapper(t *testing.T) {
	wrapped := snapshotOutputCompressed{
		Compressed: true,
		Data:       "base64-encoded-data",
	}
	data, err := json.Marshal(wrapped)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded snapshotOutputCompressed
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !decoded.Compressed {
		t.Fatal("expected compressed=true")
	}
	if decoded.Data != "base64-encoded-data" {
		t.Fatalf("data mismatch")
	}
}

func TestPruneSnapshots_BeforeTime(t *testing.T) {
	store := &SnapshotStore{}
	before := time.Now().Add(-DefaultSnapshotRetention)
	n, err := store.PruneSnapshots(context.Background(), before)
	if err != nil {
		t.Fatalf("PruneSnapshots: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 rows, got %d", n)
	}
}

func TestDefaultSnapshotRetention(t *testing.T) {
	if DefaultSnapshotRetention != 30*24*time.Hour {
		t.Fatalf("expected 30d retention, got %v", DefaultSnapshotRetention)
	}
}
