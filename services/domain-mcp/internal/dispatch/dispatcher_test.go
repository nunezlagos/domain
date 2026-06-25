package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)



type stubFlowCall struct {
	OrgID   uuid.UUID
	FlowID  uuid.UUID
	Inputs  map[string]any
	Trigger string
}

type stubAgentCall struct {
	OrgID     uuid.UUID
	AgentID   uuid.UUID
	UserPromp string
	Vars      map[string]any
}

type stubSkillCall struct {
	OrgID   uuid.UUID
	SkillID uuid.UUID
	Args    map[string]any
}

// makeFlowFunc, makeAgentFunc, makeSkillFunc devuelven RunFuncs
// que registran calls + devuelven lo configurado.
func makeFlowFunc(runID uuid.UUID, err error) (RunFunc, *[]stubFlowCall) {
	var calls []stubFlowCall
	return func(ctx context.Context, req Request) (Result, error) {
		calls = append(calls, stubFlowCall{
			OrgID: req.OrgID, FlowID: req.TargetID,
			Inputs: mapFromJSON(req.Inputs), Trigger: req.Source,
		})
		if err != nil {
			return Result{}, err
		}
		return Result{RunID: runID, Status: "started"}, nil
	}, &calls
}

func makeAgentFunc(runID uuid.UUID, err error) (RunFunc, *[]stubAgentCall) {
	var calls []stubAgentCall
	return func(ctx context.Context, req Request) (Result, error) {
		inputs := mapFromJSON(req.Inputs)
		prompt, _ := inputs["input"].(string)
		calls = append(calls, stubAgentCall{
			OrgID: req.OrgID, AgentID: req.TargetID,
			UserPromp: prompt, Vars: inputs,
		})
		if err != nil {
			return Result{}, err
		}
		return Result{RunID: runID, Status: "started"}, nil
	}, &calls
}

func makeSkillFunc(execID uuid.UUID, err error) (RunFunc, *[]stubSkillCall) {
	var calls []stubSkillCall
	return func(ctx context.Context, req Request) (Result, error) {
		calls = append(calls, stubSkillCall{
			OrgID: req.OrgID, SkillID: req.TargetID, Args: mapFromJSON(req.Inputs),
		})
		if err != nil {
			return Result{}, err
		}
		return Result{RunID: execID, Status: "started"}, nil
	}, &calls
}

func mapFromJSON(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	return m
}

// newTestDispatcher construye un Dispatcher con los stubs conectados.
// SourceValidator: por default todos válidos; cada test puede
// overridearlo.
func newTestDispatcher() (*Dispatcher, *[]stubFlowCall, *[]stubAgentCall, *[]stubSkillCall) {
	flowID := uuid.New()
	agentID := uuid.New()
	skillID := uuid.New()
	runFlow, flowCalls := makeFlowFunc(flowID, nil)
	runAgent, agentCalls := makeAgentFunc(agentID, nil)
	runSkill, skillCalls := makeSkillFunc(skillID, nil)
	d := &Dispatcher{
		RunFlow:         runFlow,
		RunAgent:        runAgent,
		RunSkill:        runSkill,
		SourceValidator: func(string) bool { return true },
	}
	return d, flowCalls, agentCalls, skillCalls
}



func TestDispatch_FlowTarget_CallsFlowRunner(t *testing.T) {
	d, flowCalls, agentCalls, skillCalls := newTestDispatcher()
	flowID := uuid.New()

	inputs, _ := json.Marshal(map[string]any{"key": "value"})
	res, err := d.Dispatch(context.Background(), Request{
		OrgID: uuid.New(), Source: SourceMCP, TargetType: TargetFlow,
		TargetID: flowID, Inputs: inputs,
	})
	require.NoError(t, err)
	require.Equal(t, "started", res.Status)
	require.Len(t, *flowCalls, 1)
	require.Equal(t, flowID, (*flowCalls)[0].FlowID)
	require.Equal(t, "value", (*flowCalls)[0].Inputs["key"])
	require.Equal(t, SourceMCP, (*flowCalls)[0].Trigger)
	require.Empty(t, *agentCalls, "agent runner should NOT be called")
	require.Empty(t, *skillCalls, "skill runner should NOT be called")
}

func TestDispatch_AgentTarget_CallsAgentRunner(t *testing.T) {
	d, flowCalls, agentCalls, _ := newTestDispatcher()

	inputs, _ := json.Marshal(map[string]any{"input": "hello", "x": 1})
	_, err := d.Dispatch(context.Background(), Request{
		OrgID: uuid.New(), Source: SourceCron, TargetType: TargetAgent,
		TargetID: uuid.New(), Inputs: inputs,
	})
	require.NoError(t, err)
	require.Len(t, *agentCalls, 1)
	require.Equal(t, "hello", (*agentCalls)[0].UserPromp)

	require.Equal(t, float64(1), (*agentCalls)[0].Vars["x"])
	require.Empty(t, *flowCalls)
}

func TestDispatch_SkillTarget_CallsSkillRunner(t *testing.T) {
	d, _, agentCalls, skillCalls := newTestDispatcher()

	inputs, _ := json.Marshal(map[string]any{"a": "b"})
	_, err := d.Dispatch(context.Background(), Request{
		OrgID: uuid.New(), Source: SourceWebhook, TargetType: TargetSkill,
		TargetID: uuid.New(), Inputs: inputs,
	})
	require.NoError(t, err)
	require.Len(t, *skillCalls, 1)
	require.Equal(t, "b", (*skillCalls)[0].Args["a"])
	require.Empty(t, *agentCalls)
}

func TestDispatch_UnknownTarget_ReturnsError(t *testing.T) {
	d, _, _, _ := newTestDispatcher()

	_, err := d.Dispatch(context.Background(), Request{
		OrgID: uuid.New(), Source: SourceMCP, TargetType: "unknown_thing",
		TargetID: uuid.New(),
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrUnknownTargetType),
		"debe ser ErrUnknownTargetType, got %T: %v", err, err)
}

func TestDispatch_RunnerErrorBubblesUp(t *testing.T) {
	flowID := uuid.New()
	runFlow, _ := makeFlowFunc(uuid.New(), errors.New("flow runner down"))
	runAgent, _ := makeAgentFunc(uuid.New(), nil)
	runSkill, _ := makeSkillFunc(uuid.New(), nil)
	d := &Dispatcher{
		RunFlow: runFlow, RunAgent: runAgent, RunSkill: runSkill,
		SourceValidator: func(string) bool { return true },
	}
	_, err := d.Dispatch(context.Background(), Request{
		OrgID: uuid.New(), Source: SourceMCP, TargetType: TargetFlow,
		TargetID: flowID,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "flow runner down")
}

func TestDispatch_UnknownSource_DoesNotCrash(t *testing.T) {

	d, flowCalls, _, _ := newTestDispatcher()
	d.SourceValidator = func(s string) bool {
		return s == SourceCron || s == SourceWebhook || s == SourceMCP || s == SourceManual
	}

	_, err := d.Dispatch(context.Background(), Request{
		OrgID: uuid.New(), Source: "experiment_X", TargetType: TargetFlow,
		TargetID: uuid.New(),
	})
	require.NoError(t, err, "unknown source should not fail dispatch")
	require.Len(t, *flowCalls, 1)
}

func TestDispatch_NilRunner_ReturnsConfigurableError(t *testing.T) {
	d := &Dispatcher{
		RunFlow:         nil,
		RunAgent:        nil,
		RunSkill:        nil,
		SourceValidator: func(string) bool { return true },
	}
	_, err := d.Dispatch(context.Background(), Request{
		OrgID: uuid.New(), Source: SourceMCP, TargetType: TargetFlow,
		TargetID: uuid.New(),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "flow runner not configured")
}

func TestDispatch_AuditRecorder_ReceivesStartedAndCompleted(t *testing.T) {
	rec := &stubAuditRecorder{}
	d, _, _, _ := newTestDispatcher()
	d.Audit = rec
	targetID := uuid.New()
	orgID := uuid.New()

	_, err := d.Dispatch(context.Background(), Request{
		OrgID: orgID, Source: SourceMCP, TargetType: TargetFlow,
		TargetID: targetID,
	})
	require.NoError(t, err)
	require.Len(t, rec.events, 2, "debe haber 2 audit events (started + completed)")
	require.Equal(t, "dispatch.started", rec.events[0].Action)
	require.Equal(t, "dispatch.completed", rec.events[1].Action)
	require.Equal(t, targetID, rec.events[0].EntityID)
	require.Equal(t, orgID, rec.events[0].OrgID)
}

func TestDispatch_AuditRecorder_FailureIncludesError(t *testing.T) {
	rec := &stubAuditRecorder{}
	runFlow, _ := makeFlowFunc(uuid.New(), errors.New("boom"))
	d := &Dispatcher{
		RunFlow:         runFlow,
		RunAgent:        func(ctx context.Context, req Request) (Result, error) { return Result{}, nil },
		RunSkill:        func(ctx context.Context, req Request) (Result, error) { return Result{}, nil },
		SourceValidator: func(string) bool { return true },
		Audit:           rec,
	}
	_, err := d.Dispatch(context.Background(), Request{
		OrgID: uuid.New(), Source: SourceCron, TargetType: TargetFlow,
		TargetID: uuid.New(),
	})
	require.Error(t, err)
	require.Len(t, rec.events, 2)

	completed := rec.events[1]
	require.Equal(t, "failed", completed.Metadata["result"])
	require.Equal(t, "boom", completed.Metadata["error"])
}

func TestDispatch_MetricsRecorder_ReceivesObservation(t *testing.T) {
	metrics := &stubMetricsRecorder{}
	d, _, _, _ := newTestDispatcher()
	d.Metrics = metrics

	_, err := d.Dispatch(context.Background(), Request{
		OrgID: uuid.New(), Source: SourceWebhook, TargetType: TargetFlow,
		TargetID: uuid.New(),
	})
	require.NoError(t, err)
	require.Len(t, metrics.calls, 1)
	require.Equal(t, "webhook", metrics.calls[0].Source)
	require.Equal(t, "flow", metrics.calls[0].TargetType)
	require.Equal(t, "success", metrics.calls[0].Result)
	require.Greater(t, metrics.calls[0].DurationSec, 0.0)
}

func TestDispatch_MetricsRecorder_FailureLabel(t *testing.T) {
	metrics := &stubMetricsRecorder{}
	runFlow, _ := makeFlowFunc(uuid.New(), errors.New("nope"))
	d := &Dispatcher{
		RunFlow:         runFlow,
		RunAgent:        func(ctx context.Context, req Request) (Result, error) { return Result{}, nil },
		RunSkill:        func(ctx context.Context, req Request) (Result, error) { return Result{}, nil },
		SourceValidator: func(string) bool { return true },
		Metrics:         metrics,
	}
	_, _ = d.Dispatch(context.Background(), Request{
		OrgID: uuid.New(), Source: SourceCron, TargetType: TargetFlow,
		TargetID: uuid.New(),
	})
	require.Len(t, metrics.calls, 1)
	require.Equal(t, "failed", metrics.calls[0].Result)
}



type stubAuditRecorder struct {
	events []AuditEvent
}

func (s *stubAuditRecorder) Record(_ context.Context, e AuditEvent) error {
	s.events = append(s.events, e)
	return nil
}

type metricsCall struct {
	Source      string
	TargetType  string
	Result      string
	DurationSec float64
}

type stubMetricsRecorder struct {
	calls []metricsCall
}

func (s *stubMetricsRecorder) ObserveDispatch(source, targetType, result string, durSec float64) {
	s.calls = append(s.calls, metricsCall{
		Source: source, TargetType: targetType, Result: result, DurationSec: durSec,
	})
}
