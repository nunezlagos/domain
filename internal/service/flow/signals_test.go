package flow

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestErrSignalTimeout(t *testing.T) {
	if ErrSignalTimeout == nil {
		t.Fatal("ErrSignalTimeout should not be nil")
	}
}

func TestErrSignalTimeoutIsDistinct(t *testing.T) {
	if errors.Is(ErrSignalTimeout, ErrSignalNotFound) {
		t.Fatal("ErrSignalTimeout should not be ErrSignalNotFound")
	}
}

func TestSignalPendingExpectation_JSONRoundTrip(t *testing.T) {
	exp := SignalPendingExpectation{
		ID:         mustParseUUID("00000000-0000-0000-0000-000000000001"),
		FlowRunID:  mustParseUUID("00000000-0000-0000-0000-000000000002"),
		StepID:     "step-1",
		SignalName: "approval_received",
	}
	data, err := json.Marshal(exp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded SignalPendingExpectation
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.SignalName != "approval_received" {
		t.Fatalf("expected approval_received, got %s", decoded.SignalName)
	}
	if decoded.StepID != "step-1" {
		t.Fatalf("expected step-1, got %s", decoded.StepID)
	}
}

func TestSignalRoundTrip(t *testing.T) {
	sig := Signal{
		ID:      1,
		Name:    "approve",
		Payload: json.RawMessage(`{"approved":true}`),
	}
	data, err := json.Marshal(sig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded Signal
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Name != "approve" {
		t.Fatalf("expected approve, got %s", decoded.Name)
	}
}

func mustParseUUID(s string) [16]byte {
	_ = s
	return [16]byte{1}
}
