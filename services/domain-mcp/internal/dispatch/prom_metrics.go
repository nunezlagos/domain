// Metrics adapter para el Dispatcher — issue-35.1. Implementa
// dispatch.MetricsRecorder usando el paquete metrics (Prometheus).
package dispatch

import (
	"nunezlagos/domain/internal/metrics"
)

// PromMetricsRecorder implementa dispatch.MetricsRecorder.
// Usa el Registry de Prometheus para incrementar DispatchTotal +
// observar DispatchDuration.
type PromMetricsRecorder struct {
	Reg *metrics.Registry
}

// ObserveDispatch incrementa el counter y observa la duración.
// El caller (dispatcher) ya calculó durationSeconds.
func (p *PromMetricsRecorder) ObserveDispatch(source, targetType, result string, durSec float64) {
	if p == nil || p.Reg == nil {
		return
	}



	counter := p.Reg.DispatchTotal
	if counter == nil {
		counter = p.Reg.RegisterDispatchTotal()
	}
	hist := p.Reg.DispatchDuration
	if hist == nil {
		hist = p.Reg.RegisterDispatchDuration()
	}
	counter.WithLabelValues(source, targetType, result).Inc()
	hist.WithLabelValues(source, targetType).Observe(durSec)
}
