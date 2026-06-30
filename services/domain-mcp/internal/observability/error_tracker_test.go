package observability

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

type fakeErrStore struct {
	mu     sync.Mutex
	events []ErrorEvent
	fail   bool
}

func (f *fakeErrStore) UpsertErrorEvent(_ context.Context, e ErrorEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return errors.New("boom")
	}
	f.events = append(f.events, e)
	return nil
}

func (f *fakeErrStore) last() ErrorEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.events[len(f.events)-1]
}

func TestErrorTracker_Record_PersistsCategorizedEvent(t *testing.T) {
	store := &fakeErrStore{}
	tr := NewErrorTracker(store, nil)
	tr.Record(context.Background(), &pgconn.PgError{Code: "42P01", Message: "relation x does not exist"}, "bootstrap")
	got := store.last()
	if got.Category != CategorySQL {
		t.Fatalf("category: got %q want %q", got.Category, CategorySQL)
	}
	if got.Source != "bootstrap" {
		t.Fatalf("source: got %q", got.Source)
	}
	if len(got.Fingerprint) == 0 {
		t.Fatal("fingerprint should be set")
	}
	if got.Severity != "error" {
		t.Fatalf("severity for SQL: got %q want error", got.Severity)
	}
}

func TestErrorTracker_Record_NilError_NoOp(t *testing.T) {
	store := &fakeErrStore{}
	NewErrorTracker(store, nil).Record(context.Background(), nil, "src")
	if len(store.events) != 0 {
		t.Fatalf("nil error must not persist, got %d events", len(store.events))
	}
}

func TestErrorTracker_Record_FiresAlertHook(t *testing.T) {
	store := &fakeErrStore{}
	tr := NewErrorTracker(store, nil)
	var fired ErrorEvent
	var ok bool
	tr.SetAlertHook(func(_ context.Context, e ErrorEvent) { fired, ok = e, true })
	tr.Record(context.Background(), errors.New("context deadline exceeded"), "http")
	if !ok {
		t.Fatal("alert hook should fire after successful upsert")
	}
	if fired.Category != CategoryTimeout {
		t.Fatalf("hook category: got %q want %q", fired.Category, CategoryTimeout)
	}
}

func TestErrorTracker_Record_StoreFails_NoAlertNoPanic(t *testing.T) {
	store := &fakeErrStore{fail: true}
	tr := NewErrorTracker(store, nil)
	fired := false
	tr.SetAlertHook(func(_ context.Context, _ ErrorEvent) { fired = true })
	tr.Record(context.Background(), errors.New("panic: nil pointer"), "worker")
	if fired {
		t.Fatal("alert hook must not fire when upsert fails")
	}
}

func TestErrorTracker_DefaultSeverity_PanicIsCritical(t *testing.T) {
	if got := defaultSeverity(CategoryPanic); got != "critical" {
		t.Fatalf("got %q want critical", got)
	}
}
