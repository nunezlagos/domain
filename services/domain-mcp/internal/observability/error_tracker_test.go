package observability

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
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

func (f *fakeErrStore) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.events)
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
	tr.Close() // drena workers antes de leer
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
	tr := NewErrorTracker(store, nil)
	tr.Record(context.Background(), nil, "src")
	tr.Close()
	if store.count() != 0 {
		t.Fatalf("nil error must not persist, got %d events", store.count())
	}
}

func TestErrorTracker_Record_FiresAlertAndHealHooks(t *testing.T) {
	store := &fakeErrStore{}
	tr := NewErrorTracker(store, nil)
	var mu sync.Mutex
	var fired ErrorEvent
	var alerted, healed bool
	tr.SetAlertHook(func(_ context.Context, e ErrorEvent) { mu.Lock(); fired, alerted = e, true; mu.Unlock() })
	tr.SetHealHook(func(_ context.Context, _ ErrorEvent) { mu.Lock(); healed = true; mu.Unlock() })
	tr.Record(context.Background(), errors.New("context deadline exceeded"), "http")
	tr.Close()
	mu.Lock()
	defer mu.Unlock()
	if !alerted || !healed {
		t.Fatalf("both hooks should fire after successful upsert (alert=%v heal=%v)", alerted, healed)
	}
	if fired.Category != CategoryTimeout {
		t.Fatalf("hook category: got %q want %q", fired.Category, CategoryTimeout)
	}
}

func TestErrorTracker_Record_StoreFails_NoHooksNoPanic(t *testing.T) {
	store := &fakeErrStore{fail: true}
	tr := NewErrorTracker(store, nil)
	var fired atomic.Bool
	tr.SetAlertHook(func(_ context.Context, _ ErrorEvent) { fired.Store(true) })
	tr.SetHealHook(func(_ context.Context, _ ErrorEvent) { fired.Store(true) })
	tr.Record(context.Background(), errors.New("panic: nil pointer"), "worker")
	tr.Close()
	if fired.Load() {
		t.Fatal("hooks must not fire when upsert fails")
	}
}

func TestErrorTracker_Close_Idempotent(t *testing.T) {
	tr := NewErrorTracker(&fakeErrStore{}, nil)
	tr.Close()
	tr.Close() // segunda llamada no debe panickear ni colgar
}

func TestErrorTracker_DefaultSeverity_PanicIsCritical(t *testing.T) {
	if got := defaultSeverity(CategoryPanic); got != "critical" {
		t.Fatalf("got %q want critical", got)
	}
}
