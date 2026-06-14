package cronsched

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/dispatch"
	"nunezlagos/domain/internal/service/cron"
)

// issue-35.1 phase 5: el switch local de target_type fue eliminado.
// Los tests ahora verifican el contrato post-refactor:
//   - runTarget delega al dispatcher (1 sola implementación).
//   - Si Dispatcher == nil → error explícito.

func TestRunTarget_DispatcherNotConfigured_ReturnsError(t *testing.T) {
	s := &Scheduler{}
	c := cron.Cron{TargetType: "flow", TargetID: uuid.New()}
	err := s.runTarget(context.Background(), c)
	require.ErrorContains(t, err, "dispatcher not configured")
}

func TestRunTarget_DelegatesToDispatcher_Flow(t *testing.T) {
	called := false
	var gotReq dispatch.Request
	d := &dispatch.Dispatcher{
		RunFlow: func(_ context.Context, req dispatch.Request) (dispatch.Result, error) {
			called = true
			gotReq = req
			return dispatch.Result{RunID: uuid.New(), Status: "started"}, nil
		},
		RunAgent: func(context.Context, dispatch.Request) (dispatch.Result, error) {
			return dispatch.Result{}, errors.New("should not be called")
		},
		RunSkill: func(context.Context, dispatch.Request) (dispatch.Result, error) {
			return dispatch.Result{}, errors.New("should not be called")
		},
		SourceValidator: func(string) bool { return true },
	}
	s := &Scheduler{Dispatcher: d}
	flowID := uuid.New()
	orgID := uuid.New()
	c := cron.Cron{
		TargetType: "flow", TargetID: flowID, OrganizationID: orgID,
		Inputs: map[string]any{"k": "v"},
	}
	err := s.runTarget(context.Background(), c)
	require.NoError(t, err)
	require.True(t, called)
	require.Equal(t, dispatch.SourceCron, gotReq.Source)
	require.Equal(t, dispatch.TargetFlow, gotReq.TargetType)
	require.Equal(t, flowID, gotReq.TargetID)
	require.Equal(t, orgID, gotReq.OrgID)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(gotReq.Inputs, &parsed))
	require.Equal(t, "v", parsed["k"])
}

func TestRunTarget_UnknownTarget_BubblesDispatcherError(t *testing.T) {
	d := &dispatch.Dispatcher{
		SourceValidator: func(string) bool { return true },
	}
	s := &Scheduler{Dispatcher: d}
	c := cron.Cron{TargetType: "unknown", TargetID: uuid.New()}
	err := s.runTarget(context.Background(), c)
	require.Error(t, err)
	require.True(t, errors.Is(err, dispatch.ErrUnknownTargetType))
}
