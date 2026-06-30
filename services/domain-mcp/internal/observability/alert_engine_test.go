package observability

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeNotifier struct {
	mu    sync.Mutex
	sent  int
	fail  bool
	lastE ErrorEvent
}

func (f *fakeNotifier) Send(_ context.Context, e ErrorEvent, _ AlertConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return errors.New("send failed")
	}
	f.sent++
	f.lastE = e
	return nil
}

func (f *fakeNotifier) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sent
}

func newTestEngine(cfg AlertConfig, n Notifier) *AlertEngine {
	ae := NewAlertEngine([]AlertConfig{cfg}, map[string]Notifier{cfg.Channel: n}, nil)
	return ae
}

func sqlEvent(sev string) ErrorEvent {
	return ErrorEvent{Category: CategorySQL, Severity: sev, Message: "x", Fingerprint: []byte("fp")}
}

func TestAlertEngine_Evaluate_FiresMatchingChannel(t *testing.T) {
	n := &fakeNotifier{}
	ae := newTestEngine(AlertConfig{Category: CategorySQL, SeverityMin: "error", Channel: "webhook"}, n)
	ae.Evaluate(context.Background(), sqlEvent("error"))
	if n.count() != 1 {
		t.Fatalf("expected 1 send, got %d", n.count())
	}
}

func TestAlertEngine_Evaluate_SkipsBelowSeverityMin(t *testing.T) {
	n := &fakeNotifier{}
	ae := newTestEngine(AlertConfig{Category: CategorySQL, SeverityMin: "critical", Channel: "webhook"}, n)
	ae.Evaluate(context.Background(), sqlEvent("error"))
	if n.count() != 0 {
		t.Fatalf("error < critical must not fire, got %d", n.count())
	}
}

func TestAlertEngine_Evaluate_SkipsNonMatchingCategory(t *testing.T) {
	n := &fakeNotifier{}
	ae := newTestEngine(AlertConfig{Category: CategoryAuth, SeverityMin: "warn", Channel: "webhook"}, n)
	ae.Evaluate(context.Background(), sqlEvent("error"))
	if n.count() != 0 {
		t.Fatalf("category mismatch must not fire, got %d", n.count())
	}
}

func TestAlertEngine_Evaluate_ThrottlesRepeatedFingerprint(t *testing.T) {
	n := &fakeNotifier{}
	ae := newTestEngine(AlertConfig{Category: CategorySQL, SeverityMin: "error", Channel: "webhook", ThrottleSeconds: 60}, n)
	now := time.Unix(1000, 0)
	ae.now = func() time.Time { return now }
	ae.Evaluate(context.Background(), sqlEvent("error"))
	now = now.Add(30 * time.Second) // dentro del throttle
	ae.Evaluate(context.Background(), sqlEvent("error"))
	if n.count() != 1 {
		t.Fatalf("repeated within throttle must fire once, got %d", n.count())
	}
	now = now.Add(60 * time.Second) // fuera del throttle
	ae.Evaluate(context.Background(), sqlEvent("error"))
	if n.count() != 2 {
		t.Fatalf("after throttle window must fire again, got %d", n.count())
	}
}

func TestAlertEngine_Evaluate_SendFailDoesNotMarkThrottle(t *testing.T) {
	n := &fakeNotifier{fail: true}
	ae := newTestEngine(AlertConfig{Category: CategorySQL, SeverityMin: "error", Channel: "webhook", ThrottleSeconds: 60}, n)
	ae.Evaluate(context.Background(), sqlEvent("error"))
	n.fail = false
	ae.Evaluate(context.Background(), sqlEvent("error"))
	if n.count() != 1 {
		t.Fatalf("after a failed send the retry should go through, got %d", n.count())
	}
}
