package orchestrator

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/orchestrator/modes"
)

// Sin LLM factory, los paths agénticos server-side degradan a un error
// estructurado (ErrLLMFactoryRequired) sin crashear (DOMAINSERV-58).

func TestRunSolo_NilLLM_ReturnsErrLLMFactoryRequired(t *testing.T) {
	s := &Service{}
	err := s.runSolo(context.Background(), OrchestrateInput{}, uuid.Nil, uuid.Nil, uuid.Nil, &modes.PhasePlan{})
	require.ErrorIs(t, err, ErrLLMFactoryRequired)
}

func TestProcessAsyncFlowRun_NilLLM_ReturnsErrLLMFactoryRequired(t *testing.T) {
	s := &Service{}
	err := s.ProcessAsyncFlowRun(context.Background(), uuid.New())
	require.ErrorIs(t, err, ErrLLMFactoryRequired)
}
