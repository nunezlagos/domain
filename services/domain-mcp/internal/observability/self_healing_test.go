package observability

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type fakeKnownStore struct {
	ke  *KnownError
	hit bool
	err error
}

func (f *fakeKnownStore) LookupKnownError(_ context.Context, _ []byte) (*KnownError, bool, error) {
	return f.ke, f.hit, f.err
}

func newHealer(store KnownErrorStore, action HealFunc) *SelfHealer {
	h := NewSelfHealer(store, nil)
	h.sleep = func(time.Duration) {}
	if action != nil {
		h.actions["retry"] = action
	}
	return h
}

func knownRetry() *KnownError {
	return &KnownError{Name: "transient", Recoverable: true, AutoHealAction: "retry"}
}

func TestSelfHealer_Heal_RunsActionForKnownRecoverable(t *testing.T) {
	var calls int32
	h := newHealer(&fakeKnownStore{ke: knownRetry(), hit: true}, func(context.Context, map[string]any) error {
		atomic.AddInt32(&calls, 1)
		return nil
	})
	h.Heal(context.Background(), sqlEvent("error"))
	if calls != 1 {
		t.Fatalf("expected 1 action call, got %d", calls)
	}
}

func TestSelfHealer_Heal_SkipsUnknownFingerprint(t *testing.T) {
	var calls int32
	h := newHealer(&fakeKnownStore{hit: false}, func(context.Context, map[string]any) error {
		atomic.AddInt32(&calls, 1)
		return nil
	})
	h.Heal(context.Background(), sqlEvent("error"))
	if calls != 0 {
		t.Fatalf("unknown fingerprint must not heal, got %d", calls)
	}
}

func TestSelfHealer_Heal_SkipsNonRecoverable(t *testing.T) {
	var calls int32
	ke := knownRetry()
	ke.Recoverable = false
	h := newHealer(&fakeKnownStore{ke: ke, hit: true}, func(context.Context, map[string]any) error {
		atomic.AddInt32(&calls, 1)
		return nil
	})
	h.Heal(context.Background(), sqlEvent("error"))
	if calls != 0 {
		t.Fatalf("non-recoverable must not heal, got %d", calls)
	}
}

func TestSelfHealer_Heal_SkipsActionNone(t *testing.T) {
	var calls int32
	ke := &KnownError{Name: "manual", Recoverable: true, AutoHealAction: "none"}
	h := newHealer(&fakeKnownStore{ke: ke, hit: true}, func(context.Context, map[string]any) error {
		atomic.AddInt32(&calls, 1)
		return nil
	})
	h.Heal(context.Background(), sqlEvent("error"))
	if calls != 0 {
		t.Fatalf("action=none must not heal, got %d", calls)
	}
}

func TestSelfHealer_Heal_RetriesUpToMaxThenAborts(t *testing.T) {
	var calls int32
	h := newHealer(&fakeKnownStore{ke: knownRetry(), hit: true}, func(context.Context, map[string]any) error {
		atomic.AddInt32(&calls, 1)
		return errors.New("still failing")
	})
	h.Heal(context.Background(), sqlEvent("error"))
	if calls != 3 {
		t.Fatalf("expected 3 attempts before abort, got %d", calls)
	}
}

func TestSelfHealer_Heal_StopsOnFirstSuccess(t *testing.T) {
	var calls int32
	h := newHealer(&fakeKnownStore{ke: knownRetry(), hit: true}, func(context.Context, map[string]any) error {
		if atomic.AddInt32(&calls, 1) < 2 {
			return errors.New("first fails")
		}
		return nil
	})
	h.Heal(context.Background(), sqlEvent("error"))
	if calls != 2 {
		t.Fatalf("expected stop after 2nd success, got %d", calls)
	}
}
