package orchestrator

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/orchestrator/phases"
)

func TestService_Run_RejectsEmptyRawText(t *testing.T) {
	t.Parallel()
	s := New(nil, nil, phases.NewRegistry(), "dev")
	_, err := s.Run(context.Background(), OrchestrateInput{
		OrganizationID: uuid.New(),
		UserID:         uuid.New(),
		RawText:        "   ",
	})
	require.ErrorIs(t, err, ErrEmptyRawText)
}

func TestService_Run_RejectsInvalidMode(t *testing.T) {
	t.Parallel()
	s := New(nil, nil, phases.NewRegistry(), "dev")
	_, err := s.Run(context.Background(), OrchestrateInput{
		OrganizationID: uuid.New(),
		UserID:         uuid.New(),
		RawText:        "refactor X",
		Mode:           Mode("bogus"),
	})
	require.ErrorIs(t, err, ErrInvalidMode)
}

func TestService_Run_AsyncWithExpressMaxLines_Rejected_D6(t *testing.T) {
	t.Parallel()
	s := New(nil, nil, phases.NewRegistry(), "dev")
	_, err := s.Run(context.Background(), OrchestrateInput{
		OrganizationID:  uuid.New(),
		UserID:          uuid.New(),
		RawText:         "any",
		Mode:            ModeAsync,
		ExpressMaxLines: 5,
	})
	require.ErrorIs(t, err, ErrAsyncModeUnsupported)
}

func TestService_Run_UnknownStartingPhase(t *testing.T) {
	t.Parallel()
	s := New(nil, nil, phases.NewRegistry(), "dev")
	_, err := s.Run(context.Background(), OrchestrateInput{
		OrganizationID: uuid.New(),
		UserID:         uuid.New(),
		RawText:        "x",
		StartingPhase:  PhaseSlug("sdd-no-such-phase"),
	})
	require.ErrorIs(t, err, ErrUnknownPhase)
}

func TestService_Run_HappyPath_ReturnsIDs(t *testing.T) {
	t.Parallel()
	s := New(nil, nil, phases.NewRegistry(), "dev")
	res, err := s.Run(context.Background(), OrchestrateInput{
		OrganizationID: uuid.New(),
		UserID:         uuid.New(),
		RawText:        "implement issue-08.10 dispatch",
	})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, res.OrchestratorRunID)
	require.NotEqual(t, uuid.Nil, res.FlowRunID)
	require.Equal(t, ModeFull, res.Mode, "Mode vacío resuelve a Full por default")
}
