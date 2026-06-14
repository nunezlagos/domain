package cronsched

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/cron"
)

func TestDispatchSync_FlowNil_ReturnsError(t *testing.T) {
	s := &Scheduler{}
	c := cron.Cron{TargetType: "flow", TargetID: uuid.New()}
	err := s.dispatchSync(context.Background(), c)
	require.ErrorContains(t, err, "flow runner not configured")
}

func TestDispatchSync_AgentNil_ReturnsError(t *testing.T) {
	s := &Scheduler{}
	c := cron.Cron{TargetType: "agent", TargetID: uuid.New()}
	err := s.dispatchSync(context.Background(), c)
	require.ErrorContains(t, err, "agent runner not configured")
}

func TestDispatchSync_SkillNil_ReturnsError(t *testing.T) {
	s := &Scheduler{}
	c := cron.Cron{TargetType: "skill", TargetID: uuid.New()}
	err := s.dispatchSync(context.Background(), c)
	require.ErrorContains(t, err, "skill runner not configured")
}

func TestDispatchSync_UnknownTarget_ReturnsError(t *testing.T) {
	s := &Scheduler{}
	c := cron.Cron{TargetType: "unknown", TargetID: uuid.New()}
	err := s.dispatchSync(context.Background(), c)
	require.ErrorContains(t, err, "unknown target_type")
}
