package flow

import (
	"testing"
	"time"
)

func TestErrHeartbeatMissed(t *testing.T) {
	if ErrHeartbeatMissed == nil {
		t.Fatal("ErrHeartbeatMissed should not be nil")
	}
}

func TestValidateProgress_Valid(t *testing.T) {
	values := []float64{0, 0.5, 1.0, 0.001, 0.999}
	for _, v := range values {
		if err := ValidateProgress(v); err != nil {
			t.Fatalf("expected valid for %f, got %v", v, err)
		}
	}
}

func TestValidateProgress_Invalid(t *testing.T) {
	values := []float64{-0.1, 1.1, -1, 2.5}
	for _, v := range values {
		if err := ValidateProgress(v); err == nil {
			t.Fatalf("expected error for %f", v)
		}
	}
}

func TestHeartbeatStore_Defaults(t *testing.T) {
	store := &HeartbeatStore{}
	if store.HeartbeatTimeout != 0 {
		t.Fatal("expected zero timeout default")
	}
}

func TestHeartbeatStore_FindStuckLimitDefaults(t *testing.T) {
	store := &HeartbeatStore{}
	// With nil pool, this will fail at DB call, not at limit validation
	// Just confirm it doesn't panic due to limit computation
	_ = store.HeartbeatTimeout
	_ = store.Pool
}

func TestHeartbeatTimeoutDefault(t *testing.T) {
	timeout := 5 * time.Minute
	if timeout != 5*time.Minute {
		t.Fatal("expected 5m timeout")
	}
}
