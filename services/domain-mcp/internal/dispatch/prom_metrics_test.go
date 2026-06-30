package dispatch

import (
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/metrics"
)

func TestPromMetricsRecorder_ObserveDispatch(t *testing.T) {
	reg := metrics.New()
	p := &PromMetricsRecorder{Reg: reg}


	p.ObserveDispatch("cron", "flow", "success", 0.5)
	p.ObserveDispatch("mcp", "agent", "failed", 0.1)
	p.ObserveDispatch("webhook", "skill", "success", 1.2)


	families, err := reg.Prometheus().Gather()
	require.NoError(t, err)
	found := false
	for _, f := range families {
		if f.GetName() == "domain_dispatch_total" {
			found = true
			require.GreaterOrEqual(t, len(f.Metric), 3)
		}
	}
	require.True(t, found, "DispatchTotal metric not exposed")
}

func TestPromMetricsRecorder_NilSafe(t *testing.T) {

	var p *PromMetricsRecorder
	p.ObserveDispatch("cron", "flow", "success", 0.5) // safe nil receiver

	p2 := &PromMetricsRecorder{}
	p2.ObserveDispatch("cron", "flow", "success", 0.5) // Reg nil
}
