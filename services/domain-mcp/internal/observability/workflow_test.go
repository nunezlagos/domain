package observability

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestWorkflowID_RoundTrip(t *testing.T) {
	id := NewWorkflowID()
	require.NotEqual(t, uuid.Nil, id, "NewWorkflowID should generate non-nil UUID v7")
	require.Equal(t, byte(0x07), id[6]>>4, "v7 marker nibble expected at byte 6 high bits")

	ctx := WithWorkflowID(context.Background(), id)
	got := WorkflowIDFromContext(ctx)
	require.Equal(t, id, got)
}

func TestWorkflowIDFromContext_Empty(t *testing.T) {
	require.Equal(t, uuid.Nil, WorkflowIDFromContext(context.Background()))
}

func TestWithWorkflowID_NilGeneratesNew(t *testing.T) {
	ctx := WithWorkflowID(context.Background(), uuid.Nil)
	id := WorkflowIDFromContext(ctx)
	require.NotEqual(t, uuid.Nil, id)
}

func TestWorkflowName_RoundTrip(t *testing.T) {
	ctx := WithWorkflowName(context.Background(), "issue_create")
	require.Equal(t, "issue_create", WorkflowNameFromContext(ctx))
	require.Equal(t, "", WorkflowNameFromContext(context.Background()))
}

func TestWorkflowStart_RoundTrip(t *testing.T) {
	now := time.Now()
	ctx := WithWorkflowStart(context.Background(), now)
	got := WorkflowStartFromContext(ctx)
	require.WithinDuration(t, now, got, time.Millisecond)
}

func TestWithWorkflowStart_ZeroUsesNow(t *testing.T) {
	before := time.Now()
	ctx := WithWorkflowStart(context.Background(), time.Time{})
	after := time.Now()
	got := WorkflowStartFromContext(ctx)
	require.True(t, got.After(before) || got.Equal(before))
	require.True(t, got.Before(after) || got.Equal(after))
}

func TestEnsureWorkflowID_New(t *testing.T) {
	ctx := context.Background()
	id, ctx := EnsureWorkflowID(ctx)
	require.NotEqual(t, uuid.Nil, id)
	require.Equal(t, id, WorkflowIDFromContext(ctx))
}

func TestEnsureWorkflowID_Preserves(t *testing.T) {
	preset := NewWorkflowID()
	ctx := WithWorkflowID(context.Background(), preset)
	got, _ := EnsureWorkflowID(ctx)
	require.Equal(t, preset, got, "EnsureWorkflowID should not overwrite existing id")
}

func TestNewWorkflowID_Unique(t *testing.T) {
	seen := make(map[uuid.UUID]bool, 100)
	for i := 0; i < 100; i++ {
		id := NewWorkflowID()
		require.False(t, seen[id], "duplicate id %s", id)
		seen[id] = true
	}
}

func TestNewWorkflowID_TimestampOrderable(t *testing.T) {
	ids := make([]uuid.UUID, 5)
	for i := range ids {
		ids[i] = NewWorkflowID()
		time.Sleep(2 * time.Millisecond)
	}
	for i := 1; i < len(ids); i++ {
		require.True(t, ids[i].String() > ids[i-1].String(),
			"v7 ids must sort by timestamp: %s should be > %s", ids[i], ids[i-1])
	}
}

func TestTrackerConstructor_Defaults(t *testing.T) {
	store := &stubWorkflowStore{}
	tr := NewTracker(store, nil, 0, 0)
	require.Equal(t, TrackerIdleDefault, tr.idleTimeout)
	require.Equal(t, TrackerIntervalDefault, tr.idleCheckInterval)
}

type stubWorkflowStore struct {
	idleMarked int
}

func (s *stubWorkflowStore) UpsertWorkflow(_ context.Context, _ WorkflowRow) error { return nil }
func (s *stubWorkflowStore) MarkWorkflowIdle(_ context.Context, _ time.Duration) (int, error) {
	return s.idleMarked, nil
}
func (s *stubWorkflowStore) GetWorkflow(_ context.Context, id uuid.UUID) (WorkflowRow, error) {
	return WorkflowRow{ID: id}, nil
}

func TestTracker_Stop_Idempotent(t *testing.T) {
	tr := NewTracker(&stubWorkflowStore{}, nil, 0, 0)
	tr.Start(context.Background())
	tr.Stop()
	require.NotPanics(t, func() { tr.Stop() })
}

func TestTracker_Heartbeat_MarksIdle(t *testing.T) {
	store := &stubWorkflowStore{idleMarked: 3}
	logger, buf := captureLogger()
	tr := NewTracker(store, logger, 50*time.Millisecond, 30*time.Millisecond)
	tr.Start(context.Background())
	time.Sleep(100 * time.Millisecond)
	tr.Stop()
	require.Contains(t, buf.String(), "idle")
}

func TestNullableStartedAt_ZeroBecomesNil(t *testing.T) {
	// El zero-value de Go debe mapear a nil → SQL NULL → DEFAULT now().
	// Es la raíz del bug: pasar el zero-value directo persistía
	// started_at = 0001-01-01 en vez de disparar el DEFAULT.
	require.Nil(t, nullableStartedAt(time.Time{}))
}

func TestNullableStartedAt_RealValuePreserved(t *testing.T) {
	now := time.Now()
	got := nullableStartedAt(now)
	require.NotNil(t, got)
	require.Equal(t, now, *got)
}

func TestTracker_Touch_NilUUID_NoOp(t *testing.T) {
	store := &stubWorkflowStore{}
	tr := NewTracker(store, nil, 0, 0)
	require.NotPanics(t, func() {
		tr.Touch(context.Background(), WorkflowRow{ID: uuid.Nil})
	})
}
