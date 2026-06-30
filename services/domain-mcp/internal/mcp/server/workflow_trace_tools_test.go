package mcpserver

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/observability"
)

type stubWorkflowStore struct {
	workflow observability.WorkflowRow
	err     error
}

func (s *stubWorkflowStore) GetWorkflow(_ context.Context, id uuid.UUID) (observability.WorkflowRow, error) {
	if s.err != nil {
		return observability.WorkflowRow{}, s.err
	}
	return s.workflow, nil
}

func TestWorkflowTrace_MissingStore_ReturnsError(t *testing.T) {
	h := &workflowTraceHandlers{}
	res, err := h.handleWorkflowTrace(context.Background(), mcp.CallToolRequest{})
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Content[0].(mcp.TextContent).Text, "store not configured")
}

func TestWorkflowTrace_InvalidUUID(t *testing.T) {
	h := &workflowTraceHandlers{store: &stubWorkflowStore{}}
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"workflow_id": "not-a-uuid"}
	res, err := h.handleWorkflowTrace(context.Background(), req)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Content[0].(mcp.TextContent).Text, "invalid workflow_id")
}

func TestWorkflowTrace_StoreError_ReturnsError(t *testing.T) {
	h := &workflowTraceHandlers{store: &stubWorkflowStore{err: errors.New("boom")}}
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"workflow_id": uuid.New().String()}
	res, err := h.handleWorkflowTrace(context.Background(), req)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Content[0].(mcp.TextContent).Text, "boom")
}

func TestWorkflowTrace_Success(t *testing.T) {
	id := uuid.New()
	now := time.Now()
	h := &workflowTraceHandlers{store: &stubWorkflowStore{
		workflow: observability.WorkflowRow{
			ID:             id,
			Name:           "issue_create",
			Status:         observability.WorkflowRunning,
			StartedAt:      now,
			LastActivityAt: now,
			TotalToolCalls: 5,
			TotalErrors:    1,
		},
	}}
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"workflow_id": id.String()}
	res, err := h.handleWorkflowTrace(context.Background(), req)
	require.NoError(t, err)
	require.False(t, res.IsError)
	body := res.Content[0].(mcp.TextContent).Text
	require.Contains(t, body, "issue_create")
	require.Contains(t, body, id.String())
}

func TestParseSinceOrDefault(t *testing.T) {
	got, err := parseSinceOrDefault("", 24*time.Hour)
	require.NoError(t, err)
	require.WithinDuration(t, time.Now().Add(-24*time.Hour), got, time.Second)

	got, err = parseSinceOrDefault("not-a-date", 1*time.Hour)
	require.NoError(t, err)
	require.WithinDuration(t, time.Now().Add(-1*time.Hour), got, time.Second)

	got, err = parseSinceOrDefault("2026-06-30T15:00:00Z", 24*time.Hour)
	require.NoError(t, err)
	require.Equal(t, 2026, got.Year())
}

func TestWorkflowRowToMap_OmitsNilValues(t *testing.T) {
	w := observability.WorkflowRow{
		ID:     uuid.New(),
		Status: observability.WorkflowRunning,
	}
	m := workflowRowToMap(w)
	require.NotEmpty(t, m["id"])
	require.Equal(t, "running", m["status"])
}