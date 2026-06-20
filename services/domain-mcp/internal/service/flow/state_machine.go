package flow

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

// FlowStateMachine implements deterministic state transitions for flow runs.
// Each FlowRun has a global status and per-step status.
// The machine validates transitions and rejects illegal ones.

// ErrInvalidTransition is returned when a state transition is not allowed.
var ErrInvalidTransition = errors.New("invalid state transition")

// FlowStatus represents the lifecycle state of a flow run.
type FlowStatus string

const (
	FlowStatusPending         FlowStatus = "pending"
	FlowStatusRunning         FlowStatus = "running"
	FlowStatusCompleted       FlowStatus = "completed"
	FlowStatusFailed          FlowStatus = "failed"
	FlowStatusPaused          FlowStatus = "paused"
	FlowStatusCancelled       FlowStatus = "cancelled"
	FlowStatusPausedAwaitHuman  FlowStatus = "paused_awaiting_human"
	FlowStatusPausedAwaitSignal FlowStatus = "paused_awaiting_signal"
	FlowStatusPausedError    FlowStatus = "paused_error"
)

// StepStatus represents the lifecycle state of an individual step within a run.
type StepStatus string

const (
	StepStatusPending   StepStatus = "step_pending"
	StepStatusRunning   StepStatus = "step_running"
	StepStatusCompleted StepStatus = "step_completed"
	StepStatusFailed    StepStatus = "step_failed"
	StepStatusPaused    StepStatus = "step_paused"
	StepStatusCancelled StepStatus = "step_cancelled"
	StepStatusSkipped   StepStatus = "step_skipped"
)

// FlowStateMachine holds the legal transition tables.
type FlowStateMachine struct {
	transitions     map[FlowStatus]map[FlowStatus]bool
	stepTransitions map[StepStatus]map[StepStatus]bool
}

// flowTransitions defines all legal flow status transitions.
// Key = from state, value = set of allowed to states.
var flowTransitions = map[FlowStatus]map[FlowStatus]bool{
	FlowStatusPending: {
		FlowStatusRunning:   true,
		FlowStatusCancelled: true,
	},
	FlowStatusRunning: {
		FlowStatusCompleted:        true,
		FlowStatusFailed:           true,
		FlowStatusPaused:           true,
		FlowStatusCancelled:        true,
		FlowStatusPausedAwaitHuman:  true,
		FlowStatusPausedAwaitSignal: true,
		FlowStatusPausedError:      true,
	},
	FlowStatusPaused: {
		FlowStatusRunning:   true,
		FlowStatusCancelled: true,
	},
	FlowStatusPausedAwaitHuman: {
		FlowStatusRunning:   true,
		FlowStatusCancelled: true,
	},
	FlowStatusPausedAwaitSignal: {
		FlowStatusRunning:   true,
		FlowStatusCancelled: true,
	},
	FlowStatusPausedError: {
		FlowStatusRunning:   true,
		FlowStatusCancelled: true,
	},
	FlowStatusCompleted: {},
	FlowStatusFailed:    {},
	FlowStatusCancelled: {},
}

// stepTransitions defines all legal step status transitions.
var stepTransitions = map[StepStatus]map[StepStatus]bool{
	StepStatusPending: {
		StepStatusRunning:   true,
		StepStatusSkipped:   true,
		StepStatusCancelled: true,
	},
	StepStatusRunning: {
		StepStatusCompleted: true,
		StepStatusFailed:    true,
		StepStatusPaused:    true,
		StepStatusCancelled: true,
	},
	StepStatusPaused: {
		StepStatusRunning:   true,
		StepStatusCancelled: true,
	},
	StepStatusCompleted: {},
	StepStatusFailed:    {},
	StepStatusCancelled: {},
	StepStatusSkipped:   {},
}

// NewFlowStateMachine creates a state machine with default legal transitions.
func NewFlowStateMachine() *FlowStateMachine {
	// Copy maps to prevent mutation of globals
	fm := make(map[FlowStatus]map[FlowStatus]bool, len(flowTransitions))
	for from, tos := range flowTransitions {
		tm := make(map[FlowStatus]bool, len(tos))
		for to := range tos {
			tm[to] = true
		}
		fm[from] = tm
	}
	sm := make(map[StepStatus]map[StepStatus]bool, len(stepTransitions))
	for from, tos := range stepTransitions {
		tm := make(map[StepStatus]bool, len(tos))
		for to := range tos {
			tm[to] = true
		}
		sm[from] = tm
	}
	return &FlowStateMachine{transitions: fm, stepTransitions: sm}
}

// ValidateTransition returns nil if the flow transition from→to is legal.
func (m *FlowStateMachine) ValidateTransition(from, to FlowStatus) error {
	allowed, ok := m.transitions[from]
	if !ok {
		return fmt.Errorf("%w: unknown source state %q", ErrInvalidTransition, from)
	}
	if !allowed[to] {
		return fmt.Errorf("%w: %s → %s", ErrInvalidTransition, from, to)
	}
	return nil
}

// AllowedTransitions returns all legal target states from the given status.
func (m *FlowStateMachine) AllowedTransitions(from FlowStatus) []FlowStatus {
	var out []FlowStatus
	if allowed, ok := m.transitions[from]; ok {
		for to := range allowed {
			out = append(out, to)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// IsTerminal returns true if the status is a terminal state (no outgoing transitions).
func (m *FlowStateMachine) IsTerminal(status FlowStatus) bool {
	allowed, ok := m.transitions[status]
	return ok && len(allowed) == 0
}

// ValidateStepTransition returns nil if the step transition from→to is legal.
func (m *FlowStateMachine) ValidateStepTransition(from, to StepStatus) error {
	allowed, ok := m.stepTransitions[from]
	if !ok {
		return fmt.Errorf("%w: unknown source step state %q", ErrInvalidTransition, from)
	}
	if !allowed[to] {
		return fmt.Errorf("%w: step %s → %s", ErrInvalidTransition, from, to)
	}
	return nil
}

// AllowedStepTransitions returns all legal target step states from the given status.
func (m *FlowStateMachine) AllowedStepTransitions(from StepStatus) []StepStatus {
	var out []StepStatus
	if allowed, ok := m.stepTransitions[from]; ok {
		for to := range allowed {
			out = append(out, to)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// IsStepTerminal returns true if the step status is terminal.
func (m *FlowStateMachine) IsStepTerminal(status StepStatus) bool {
	allowed, ok := m.stepTransitions[status]
	return ok && len(allowed) == 0
}

// StepRunState captures the execution state of a single step within a run.
type StepRunState struct {
	Status      StepStatus `json:"status"`
	Result      any        `json:"result,omitempty"`
	Error       string     `json:"error,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// FlowRunModel is the domain model for a persisted flow run.
type FlowRunModel struct {
	ID             uuid.UUID              `json:"id"`
	FlowID         uuid.UUID              `json:"flow_id"`
	Status         FlowStatus             `json:"status"`
	Steps          map[string]StepRunState `json:"steps"`
	Error          string                 `json:"error,omitempty"`
	StartedAt      *time.Time             `json:"started_at,omitempty"`
	FinishedAt     *time.Time             `json:"finished_at,omitempty"`
	TriggeredBy    *uuid.UUID             `json:"triggered_by,omitempty"`
	TriggerType    string                 `json:"trigger_type,omitempty"`
}
