package observability

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type countingWorkflowStore struct {
	stubWorkflowStore
	mu      sync.Mutex
	upserts []WorkflowRow
}

func (s *countingWorkflowStore) UpsertWorkflow(_ context.Context, w WorkflowRow) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.upserts = append(s.upserts, w)
	return nil
}

func (s *countingWorkflowStore) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.upserts)
}

func TestLogToolInvocation_SinWorkflow_NoEmiteUUIDEnCeros(t *testing.T) {
	logger, buf := captureLogger()
	LogToolInvocation(context.Background(), logger, nil, nil,
		ToolCall{Tool: "domain_session_bootstrap", Status: "ok", DurationMS: 12})
	require.NotContains(t, buf.String(), uuid.Nil.String())
}

func TestLogToolInvocation_SinWorkflow_NoPersisteUUIDEnCeros(t *testing.T) {
	store := &stubStore{}
	inv := NewInvocationLogger(store, nil, 1, 8)
	LogToolInvocation(context.Background(), nil, inv, nil,
		ToolCall{Tool: "domain_mem_context", Status: "ok", DurationMS: 7})
	inv.Close()
	require.Len(t, store.calls, 1)
	require.Empty(t, store.calls[0].WorkflowID)
}

func TestLogToolInvocation_ConWorkflow_EmiteElIDEnElLog(t *testing.T) {
	id := NewWorkflowID()
	require.NotEqual(t, uuid.Nil, id)
	logger, buf := captureLogger()
	LogToolInvocation(WithWorkflowID(context.Background(), id), logger, nil, nil,
		ToolCall{Tool: "domain_ticket_list", Status: "ok", DurationMS: 3})
	require.Contains(t, buf.String(), id.String())
}

func TestLogToolInvocation_ConWorkflow_PersisteElIDYTocaElWorkflow(t *testing.T) {
	id := NewWorkflowID()
	store := &stubStore{}
	inv := NewInvocationLogger(store, nil, 1, 8)
	wfStore := &countingWorkflowStore{}
	LogToolInvocation(WithWorkflowID(context.Background(), id), nil, inv,
		NewTracker(wfStore, nil, 0, 0),
		ToolCall{Tool: "domain_policy_list", Status: "error", DurationMS: 9})
	inv.Close()
	require.Len(t, store.calls, 1)
	require.Equal(t, id.String(), store.calls[0].WorkflowID)
	require.Equal(t, 1, wfStore.count())
	require.Equal(t, 1, wfStore.upserts[0].TotalErrors)
}

// invariante de DOMAINSERV-189: sin workflow no nace fila en `workflows`
func TestLogToolInvocation_SinWorkflow_NoCreaFilaDeWorkflow(t *testing.T) {
	wfStore := &countingWorkflowStore{}
	LogToolInvocation(context.Background(), nil, nil, NewTracker(wfStore, nil, 0, 0),
		ToolCall{Tool: "domain_health", Status: "ok", DurationMS: 1})
	require.Equal(t, 0, wfStore.count())
}
