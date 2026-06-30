// issue-09.6 — unit tests for durable execution helpers.
package flowrunner

import (
	"testing"

	"github.com/google/uuid"
)

func TestCompressOutput_RoundTrip(t *testing.T) {
	output := map[string]any{"result": "hello world", "count": 42}
	compressed, origSize, err := CompressOutput(output)
	if err != nil {
		t.Fatalf("CompressOutput: %v", err)
	}
	if origSize == 0 {
		t.Fatal("original size should be >0")
	}
	if len(compressed) == 0 {
		t.Fatal("compressed should be non-empty")
	}

	decompressed, err := DecompressOutput(compressed)
	if err != nil {
		t.Fatalf("DecompressOutput: %v", err)
	}
	if string(decompressed) != `{"count":42,"result":"hello world"}` {
		t.Fatalf("unexpected decompressed: %s", string(decompressed))
	}
}

func TestCompressOutput_LargeData(t *testing.T) {

	big := make([]any, 10000)
	for i := range big {
		big[i] = map[string]any{"idx": i, "data": "some data here for compression testing"}
	}
	compressed, origSize, err := CompressOutput(big)
	if err != nil {
		t.Fatalf("CompressOutput: %v", err)
	}
	if origSize < 100000 {
		t.Fatalf("expected large original size, got %d", origSize)
	}
	if len(compressed) >= origSize {
		t.Fatal("compressed should be smaller than original for repetitive data")
	}
}

func TestDecompressOutput_InvalidData(t *testing.T) {
	_, err := DecompressOutput([]byte("not gzip data"))
	if err == nil {
		t.Fatal("expected error for invalid gzip data")
	}
}

func TestStepIDempotencyKey(t *testing.T) {
	runID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	key := StepIDempotencyKey(runID, "step-1")
	expected := "flow_run:550e8400-e29b-41d4-a716-446655440000:step:step-1"
	if key != expected {
		t.Fatalf("got %q, want %q", key, expected)
	}

	key2 := StepIDempotencyKey(runID, "step-1")
	if key != key2 {
		t.Fatal("idempotency key must be deterministic")
	}

	key3 := StepIDempotencyKey(runID, "step-2")
	if key == key3 {
		t.Fatal("different steps must produce different keys")
	}
}

func TestShouldSpillToS3(t *testing.T) {
	if ShouldSpillToS3(1) {
		t.Fatal("1 byte should not spill")
	}
	if ShouldSpillToS3(10*1024*1024 - 1) {
		t.Fatal("1 byte under threshold should not spill")
	}
	if !ShouldSpillToS3(10*1024*1024 + 1) {
		t.Fatal("1 byte over threshold should spill")
	}
}

func TestIsReplaySafe(t *testing.T) {

	if !IsReplaySafe(nil) {
		t.Fatal("nil should be safe")
	}

	trueVal := true
	if !IsReplaySafe(&trueVal) {
		t.Fatal("true should be safe")
	}

	falseVal := false
	if IsReplaySafe(&falseVal) {
		t.Fatal("false should NOT be safe")
	}
}

func TestIsStepReplaySafe_FromMap(t *testing.T) {

	if !IsStepReplaySafe(map[string]any{"id": "x"}) {
		t.Fatal("missing replay_safe should be safe")
	}

	if !IsStepReplaySafe(map[string]any{"replay_safe": true}) {
		t.Fatal("explicit true should be safe")
	}

	if IsStepReplaySafe(map[string]any{"replay_safe": false}) {
		t.Fatal("explicit false should NOT be safe")
	}
}

func TestExtractCompletedIDs(t *testing.T) {

	cursor := map[string]any{}
	ids := extractCompletedIDs(cursor)
	if len(ids) != 0 {
		t.Fatal("expected empty")
	}

	cursor["completed"] = []any{"a", "b", "c"}
	ids = extractCompletedIDs(cursor)
	if !ids["a"] || !ids["b"] || !ids["c"] {
		t.Fatal("missing expected IDs")
	}
	if ids["d"] {
		t.Fatal("unexpected ID")
	}

	cursor["completed"] = []any{"a", 42, "c"}
	ids = extractCompletedIDs(cursor)
	if !ids["a"] || !ids["c"] {
		t.Fatal("string IDs should be extracted")
	}
	if ids["42"] {
		t.Fatal("numeric should be skipped")
	}
}
