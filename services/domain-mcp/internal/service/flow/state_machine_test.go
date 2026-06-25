package flow

import (
	"errors"
	"fmt"
	"testing"
)

func TestFlowStateMachine_New(t *testing.T) {
	m := NewFlowStateMachine()
	if m == nil {
		t.Fatal("NewFlowStateMachine returned nil")
	}
}

func TestFlowStateMachine_ValidateTransition_Valid(t *testing.T) {
	m := NewFlowStateMachine()

	valid := []struct{ from, to FlowStatus }{
		{FlowStatusPending, FlowStatusRunning},
		{FlowStatusPending, FlowStatusCancelled},
		{FlowStatusRunning, FlowStatusCompleted},
		{FlowStatusRunning, FlowStatusFailed},
		{FlowStatusRunning, FlowStatusPaused},
		{FlowStatusRunning, FlowStatusCancelled},
		{FlowStatusRunning, FlowStatusPausedAwaitHuman},
		{FlowStatusRunning, FlowStatusPausedAwaitSignal},
		{FlowStatusRunning, FlowStatusPausedError},
		{FlowStatusPaused, FlowStatusRunning},
		{FlowStatusPaused, FlowStatusCancelled},
		{FlowStatusPausedAwaitHuman, FlowStatusRunning},
		{FlowStatusPausedAwaitHuman, FlowStatusCancelled},
		{FlowStatusPausedAwaitSignal, FlowStatusRunning},
		{FlowStatusPausedAwaitSignal, FlowStatusCancelled},
		{FlowStatusPausedError, FlowStatusRunning},
		{FlowStatusPausedError, FlowStatusCancelled},
	}

	for _, tc := range valid {
		t.Run(fmt.Sprintf("%s→%s", tc.from, tc.to), func(t *testing.T) {
			if err := m.ValidateTransition(tc.from, tc.to); err != nil {
				t.Fatalf("expected VALID transition %s → %s, got error: %v", tc.from, tc.to, err)
			}
		})
	}
}

func TestFlowStateMachine_ValidateTransition_Invalid(t *testing.T) {
	m := NewFlowStateMachine()

	invalid := []struct{ from, to FlowStatus }{
		{FlowStatusPending, FlowStatusCompleted},
		{FlowStatusPending, FlowStatusFailed},
		{FlowStatusPending, FlowStatusPaused},
		{FlowStatusPending, FlowStatusPausedAwaitHuman},
		{FlowStatusPending, FlowStatusPausedAwaitSignal},
		{FlowStatusPending, FlowStatusPausedError},
		{FlowStatusRunning, FlowStatusRunning}, // no self-transition
		{FlowStatusCompleted, FlowStatusRunning},
		{FlowStatusCompleted, FlowStatusFailed},
		{FlowStatusCompleted, FlowStatusPaused},
		{FlowStatusCompleted, FlowStatusCancelled},
		{FlowStatusFailed, FlowStatusRunning},
		{FlowStatusFailed, FlowStatusCompleted},
		{FlowStatusFailed, FlowStatusPaused},
		{FlowStatusCancelled, FlowStatusRunning},
		{FlowStatusCancelled, FlowStatusCompleted},
		{FlowStatusCancelled, FlowStatusPaused},
		{FlowStatusPaused, FlowStatusCompleted},
		{FlowStatusPaused, FlowStatusFailed},
		{FlowStatusPaused, FlowStatusPaused},
		{FlowStatusPausedAwaitHuman, FlowStatusCompleted},
		{FlowStatusPausedAwaitHuman, FlowStatusFailed},
		{FlowStatusPausedError, FlowStatusCompleted},
		{FlowStatusPausedError, FlowStatusFailed},
	}

	for _, tc := range invalid {
		t.Run(fmt.Sprintf("%s→%s", tc.from, tc.to), func(t *testing.T) {
			err := m.ValidateTransition(tc.from, tc.to)
			if err == nil {
				t.Fatalf("expected INVALID transition %s → %s, got nil error", tc.from, tc.to)
			}
			if !errors.Is(err, ErrInvalidTransition) {
				t.Fatalf("expected ErrInvalidTransition, got %v", err)
			}
		})
	}
}

func TestFlowStateMachine_ValidateTransition_UnknownSource(t *testing.T) {
	m := NewFlowStateMachine()
	err := m.ValidateTransition("nonexistent", FlowStatusRunning)
	if err == nil {
		t.Fatal("expected error for unknown source state")
	}
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition, got %v", err)
	}
}

func TestFlowStateMachine_AllowedTransitions(t *testing.T) {
	m := NewFlowStateMachine()

	cases := []struct {
		from FlowStatus
		min  int
	}{
		{FlowStatusPending, 2},
		{FlowStatusRunning, 7},
		{FlowStatusPaused, 2},
		{FlowStatusPausedAwaitHuman, 2},
		{FlowStatusPausedAwaitSignal, 2},
		{FlowStatusPausedError, 2},
		{FlowStatusCompleted, 0},
		{FlowStatusFailed, 0},
		{FlowStatusCancelled, 0},
	}

	for _, tc := range cases {
		t.Run(string(tc.from), func(t *testing.T) {
			allowed := m.AllowedTransitions(tc.from)
			if len(allowed) < tc.min {
				t.Fatalf("%s: expected at least %d allowed, got %d: %v", tc.from, tc.min, len(allowed), allowed)
			}

			for _, to := range allowed {
				if err := m.ValidateTransition(tc.from, to); err != nil {
					t.Fatalf("AllowedTransitions returned %s→%s but ValidateTransition says: %v", tc.from, to, err)
				}
			}
		})
	}
}

func TestFlowStateMachine_IsTerminal(t *testing.T) {
	m := NewFlowStateMachine()

	terminal := []FlowStatus{FlowStatusCompleted, FlowStatusFailed, FlowStatusCancelled}
	nonTerminal := []FlowStatus{FlowStatusPending, FlowStatusRunning, FlowStatusPaused}

	for _, s := range terminal {
		if !m.IsTerminal(s) {
			t.Fatalf("expected %s to be terminal", s)
		}
	}
	for _, s := range nonTerminal {
		if m.IsTerminal(s) {
			t.Fatalf("expected %s to NOT be terminal", s)
		}
	}
}

func TestStepStateMachine_ValidateTransition_Valid(t *testing.T) {
	m := NewFlowStateMachine()

	valid := []struct{ from, to StepStatus }{
		{StepStatusPending, StepStatusRunning},
		{StepStatusPending, StepStatusSkipped},
		{StepStatusPending, StepStatusCancelled},
		{StepStatusRunning, StepStatusCompleted},
		{StepStatusRunning, StepStatusFailed},
		{StepStatusRunning, StepStatusPaused},
		{StepStatusRunning, StepStatusCancelled},
		{StepStatusPaused, StepStatusRunning},
		{StepStatusPaused, StepStatusCancelled},
	}

	for _, tc := range valid {
		t.Run(fmt.Sprintf("%s→%s", tc.from, tc.to), func(t *testing.T) {
			if err := m.ValidateStepTransition(tc.from, tc.to); err != nil {
				t.Fatalf("expected VALID step transition %s → %s, got error: %v", tc.from, tc.to, err)
			}
		})
	}
}

func TestStepStateMachine_ValidateTransition_Invalid(t *testing.T) {
	m := NewFlowStateMachine()

	invalid := []struct{ from, to StepStatus }{
		{StepStatusPending, StepStatusCompleted},
		{StepStatusPending, StepStatusFailed},
		{StepStatusPending, StepStatusPaused},
		{StepStatusRunning, StepStatusSkipped},
		{StepStatusRunning, StepStatusRunning},
		{StepStatusCompleted, StepStatusRunning},
		{StepStatusCompleted, StepStatusFailed},
		{StepStatusFailed, StepStatusRunning},
		{StepStatusFailed, StepStatusCompleted},
		{StepStatusCancelled, StepStatusRunning},
		{StepStatusPaused, StepStatusCompleted},
		{StepStatusPaused, StepStatusFailed},
		{StepStatusSkipped, StepStatusRunning},
	}

	for _, tc := range invalid {
		t.Run(fmt.Sprintf("%s→%s", tc.from, tc.to), func(t *testing.T) {
			err := m.ValidateStepTransition(tc.from, tc.to)
			if err == nil {
				t.Fatalf("expected INVALID step transition %s → %s, got nil error", tc.from, tc.to)
			}
			if !errors.Is(err, ErrInvalidTransition) {
				t.Fatalf("expected ErrInvalidTransition, got %v", err)
			}
		})
	}
}

func TestStepStateMachine_AllowedTransitions(t *testing.T) {
	m := NewFlowStateMachine()

	cases := []struct {
		from StepStatus
		min  int
	}{
		{StepStatusPending, 3},
		{StepStatusRunning, 4},
		{StepStatusPaused, 2},
		{StepStatusCompleted, 0},
		{StepStatusFailed, 0},
		{StepStatusCancelled, 0},
		{StepStatusSkipped, 0},
	}

	for _, tc := range cases {
		t.Run(string(tc.from), func(t *testing.T) {
			allowed := m.AllowedStepTransitions(tc.from)
			if len(allowed) < tc.min {
				t.Fatalf("%s: expected at least %d allowed, got %d: %v", tc.from, tc.min, len(allowed), allowed)
			}
			for _, to := range allowed {
				if err := m.ValidateStepTransition(tc.from, to); err != nil {
					t.Fatalf("AllowedStepTransitions returned %s→%s but ValidateStepTransition says: %v", tc.from, to, err)
				}
			}
		})
	}
}

func TestStepStateMachine_IsStepTerminal(t *testing.T) {
	m := NewFlowStateMachine()

	terminal := []StepStatus{StepStatusCompleted, StepStatusFailed, StepStatusCancelled, StepStatusSkipped}
	nonTerminal := []StepStatus{StepStatusPending, StepStatusRunning, StepStatusPaused}

	for _, s := range terminal {
		if !m.IsStepTerminal(s) {
			t.Fatalf("expected %s to be terminal", s)
		}
	}
	for _, s := range nonTerminal {
		if m.IsStepTerminal(s) {
			t.Fatalf("expected %s to NOT be terminal", s)
		}
	}
}

// Sabotage test: if we remove validation, invalid transitions should pass.
// This test verifies the VALIDATION itself catches them.
func TestFlowStateMachine_Sabotage_RemoveValidation(t *testing.T) {
	m := NewFlowStateMachine()

	err := m.ValidateTransition(FlowStatusCompleted, FlowStatusRunning)
	if err == nil {
		t.Fatal("SABOTAGE: removed validation allows completed→running")
	}
}

func TestStepStateMachine_Sabotage_RemoveValidation(t *testing.T) {
	m := NewFlowStateMachine()

	err := m.ValidateStepTransition(StepStatusCompleted, StepStatusRunning)
	if err == nil {
		t.Fatal("SABOTAGE: removed validation allows step_completed→step_running")
	}
}
