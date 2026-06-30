package observability

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type stubResourceStore struct {
	mu    sync.Mutex
	rows  []ResourceSample
	fail  bool
	delay time.Duration
}

func (s *stubResourceStore) InsertResourceSample(ctx context.Context, sample ResourceSample) error {
	s.mu.Lock()
	failed := s.fail
	s.mu.Unlock()
	if failed {
		return errors.New("simulated insert fail")
	}
	if s.delay > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(s.delay):
		}
	}
	s.mu.Lock()
	s.rows = append(s.rows, sample)
	s.mu.Unlock()
	return nil
}

func (s *stubResourceStore) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.rows)
}

func TestResourceCollector_Capture(t *testing.T) {
	c := NewResourceCollector(&stubResourceStore{}, nil, 30)
	sample := c.Capture()
	require.GreaterOrEqual(t, sample.Goroutines, 1)
	require.Greater(t, sample.NumCPU, 0)
	require.False(t, sample.CapturedAt.IsZero())
}

func TestResourceCollector_StartStop_InsertsAtLeastOneSample(t *testing.T) {
	store := &stubResourceStore{}
	c := NewResourceCollector(store, nil, 1)
	c.Start()
	time.Sleep(50 * time.Millisecond)
	c.Stop()

	require.GreaterOrEqual(t, store.count(), 1)
}

func TestResourceCollector_Stop_Idempotent(t *testing.T) {
	c := NewResourceCollector(&stubResourceStore{}, nil, 30)
	c.Start()
	c.Stop()
	c.Stop()
}

func TestResourceCollector_InsertErrorDoesNotPanic(t *testing.T) {
	store := &stubResourceStore{fail: true}
	c := NewResourceCollector(store, nil, 1)
	c.Start()
	time.Sleep(20 * time.Millisecond)
	c.Stop()
}

func TestResourceCollector_DefaultInterval_30s(t *testing.T) {
	c := NewResourceCollector(&stubResourceStore{}, nil, 0)
	require.Equal(t, 30*time.Second, c.interval)
}

func TestResourceCollector_NegativeInterval_UsesDefault(t *testing.T) {
	c := NewResourceCollector(&stubResourceStore{}, nil, -1)
	require.Equal(t, 30*time.Second, c.interval)
}
