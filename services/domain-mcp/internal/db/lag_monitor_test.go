package db

import (
	"testing"
)

func TestFloatBitsRoundtrip(t *testing.T) {
	cases := []float64{0.0, 0.5, 5.0, 100.42, 1e10}
	for _, v := range cases {
		got := floatFromBits(floatToBits(v))
		if got != v {
			t.Fatalf("%v: roundtrip got %v", v, got)
		}
	}
}

func TestLagMonitor_LagSeconds_DefaultZero(t *testing.T) {
	m := &LagMonitor{}
	if m.LagSeconds() != 0.0 {
		t.Fatalf("default lag must be 0, got %v", m.LagSeconds())
	}
}

func TestLagMonitor_SetAndGet(t *testing.T) {
	m := &LagMonitor{}
	m.setLag(3.14)
	if m.LagSeconds() != 3.14 {
		t.Fatalf("got %v, want 3.14", m.LagSeconds())
	}
}

func TestLagMonitor_IsDegraded(t *testing.T) {
	m := &LagMonitor{ThresholdSecs: 10.0}
	m.setLag(5.0)
	if m.IsDegraded() {
		t.Fatal("5s < 10s should not be degraded")
	}
	m.setLag(15.0)
	if !m.IsDegraded() {
		t.Fatal("15s > 10s should be degraded")
	}
}

func TestPools_Read_NoReadOnly_FallbackApp(t *testing.T) {
	// nil pools just verifies routing logic. Pool dereference happens later.
	pools := &Pools{}
	if pools.Read() != pools.App {
		t.Fatal("Read() with no ReadOnly should return App")
	}
}

func TestPools_ReadFresh_AlwaysApp(t *testing.T) {
	pools := &Pools{}
	if pools.ReadFresh() != pools.App {
		t.Fatal("ReadFresh always returns App")
	}
}
