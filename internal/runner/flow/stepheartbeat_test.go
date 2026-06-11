package flowrunner

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStepHeartbeater_ShouldBeat_Throttle(t *testing.T) {
	h := &StepHeartbeater{MinInterval: 5 * time.Second}
	base := time.Now()

	require.True(t, h.shouldBeat(base), "primer beat siempre pasa")
	require.False(t, h.shouldBeat(base.Add(1*time.Second)), "dentro de la ventana → throttled")
	require.False(t, h.shouldBeat(base.Add(4999*time.Millisecond)), "justo antes del límite → throttled")
	require.True(t, h.shouldBeat(base.Add(6*time.Second)), "pasada la ventana → pasa")
	require.False(t, h.shouldBeat(base.Add(7*time.Second)), "ventana se resetea con el último beat efectivo")
}

func TestStepHeartbeater_DefaultInterval(t *testing.T) {
	h := &StepHeartbeater{} // MinInterval 0 → default 5s
	base := time.Now()
	require.True(t, h.shouldBeat(base))
	require.False(t, h.shouldBeat(base.Add(2*time.Second)))
}

func TestStepHeartbeater_NilSafe(t *testing.T) {
	var h *StepHeartbeater
	require.NoError(t, h.Beat(context.Background(), 0.5, "ok"), "nil heartbeater es no-op")

	hb := HeartbeaterFrom(context.Background())
	require.NotNil(t, hb)
	require.NoError(t, hb.Beat(context.Background(), 0.5, "ok"), "context sin heartbeater es no-op")
}

func TestWithHeartbeater_RoundTrip(t *testing.T) {
	h := &StepHeartbeater{StepKey: "s1"}
	ctx := WithHeartbeater(context.Background(), h)
	got := HeartbeaterFrom(ctx)
	require.Equal(t, "s1", got.StepKey)
}

// Sabotaje: sin throttle, beats consecutivos pasarían todos — el throttle
// debe atrapar el flood.
func TestSabotage_Heartbeat_FloodThrottled(t *testing.T) {
	h := &StepHeartbeater{MinInterval: 5 * time.Second}
	base := time.Now()
	passed := 0
	for i := 0; i < 100; i++ {
		if h.shouldBeat(base.Add(time.Duration(i) * 100 * time.Millisecond)) {
			passed++
		}
	}
	require.Equal(t, 2, passed, "100 beats en 10s con throttle 5s → solo 2 efectivos")
}
