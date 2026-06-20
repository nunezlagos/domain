package usagealerts

import (
	"context"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/service/outboundwebhook"
)

// EvaluateRunEventFired implementa outboundwebhook.UsageAlerter retornando
// la lista de alerts disparadas con sus observed values.
//
// El Service.EvaluateRunEvent devuelve []Alert sin observed_value; este wrapper
// recomputa observed por cada alert para fill the struct.
func (s *Service) EvaluateRunEventFired(ctx context.Context, orgID uuid.UUID,
	costUSD float64, tokensTotal int64) ([]outboundwebhook.AlertFired, error) {

	alerts, err := s.EvaluateRunEvent(ctx, orgID, costUSD, tokensTotal)
	if err != nil {
		return nil, err
	}
	out := make([]outboundwebhook.AlertFired, 0, len(alerts))
	for _, a := range alerts {
		var observed float64
		switch a.Metric {
		case MetricCostPerRun:
			observed = costUSD
		case MetricTokensPerRun:
			observed = float64(tokensTotal)
		}
		out = append(out, outboundwebhook.AlertFired{
			ID:         a.ID,
			Name:       a.Name,
			Metric:     a.Metric,
			Threshold:  a.Threshold,
			Observed:   observed,
			Channel:    a.Channel,
			Recipients: a.Recipients,
		})
	}
	return out, nil
}

// usageAlerterShim adapta Service.EvaluateRunEventFired al interface
// outboundwebhook.UsageAlerter sin causar import cycle.
type usageAlerterShim struct{ s *Service }

func (a *usageAlerterShim) EvaluateRunEvent(ctx context.Context, orgID uuid.UUID,
	costUSD float64, tokensTotal int64) ([]outboundwebhook.AlertFired, error) {
	return a.s.EvaluateRunEventFired(ctx, orgID, costUSD, tokensTotal)
}

// AsUsageAlerter retorna un adaptador del Service compatible con outboundwebhook.UsageAlerter.
func (s *Service) AsUsageAlerter() outboundwebhook.UsageAlerter {
	return &usageAlerterShim{s: s}
}
