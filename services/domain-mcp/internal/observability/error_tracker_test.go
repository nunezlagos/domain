package observability

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
	defer tr.Close()
	alertCh := make(chan ErrorEvent, 1)
	healCh := make(chan struct{}, 1)
	tr.SetAlertHook(func(_ context.Context, e ErrorEvent) { alertCh <- e })
	tr.SetHealHook(func(_ context.Context, _ ErrorEvent) { healCh <- struct{}{} })
	tr.Record(context.Background(), errors.New("context deadline exceeded"), "http")
	select {
	case e := <-alertCh:
		if e.Category != CategoryTimeout {
			t.Fatalf("hook category: got %q want %q", e.Category, CategoryTimeout)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("alert hook should fire after successful upsert")
	}
	select {
	case <-healCh:
	case <-time.After(2 * time.Second):
		t.Fatal("heal hook should fire after successful upsert")
	}
}

func TestErrorTracker_Record_StoreFails_NoHooksNoPanic(t *testing.T) {
	store := &fakeErrStore{fail: true}
	tr := NewErrorTracker(store, nil)
	defer tr.Close()
	fired := make(chan struct{}, 2)
	tr.SetAlertHook(func(_ context.Context, _ ErrorEvent) { fired <- struct{}{} })
	tr.SetHealHook(func(_ context.Context, _ ErrorEvent) { fired <- struct{}{} })
	tr.Record(context.Background(), errors.New("panic: nil pointer"), "worker")
	select {
	case <-fired:
		t.Fatal("hooks must not fire when upsert fails")
	case <-time.After(200 * time.Millisecond):
		// ok: no se dispararon
	}
}

// TestErrorTracker_Persist_DrainSkipsHooks verifica que el camino de drain
// (fireHooks=false) persiste el evento pero NO dispara los hooks (evita el
// leak de goroutines contra el pool en shutdown).
func TestErrorTracker_Persist_DrainSkipsHooks(t *testing.T) {
	store := &fakeErrStore{}
	tr := NewErrorTracker(store, nil)
	defer tr.Close()
	var fired atomic.Bool
	tr.SetAlertHook(func(_ context.Context, _ ErrorEvent) { fired.Store(true) })
	tr.SetHealHook(func(_ context.Context, _ ErrorEvent) { fired.Store(true) })
	tr.persist(ErrorEvent{Category: CategorySQL, Fingerprint: []byte("x")}, false)
	if store.count() == 0 {
		t.Fatal("drain must still persist the event")
	}
	if fired.Load() {
		t.Fatal("drain must not fire hooks")
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
