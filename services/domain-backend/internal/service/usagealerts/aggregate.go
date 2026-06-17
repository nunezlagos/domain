// issue-15.3 aggregate metric alert evaluator (cron-driven).

package usagealerts

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// aggregateResult agrupa valores observados para una org.
type aggregateResult struct {
	OrgID        uuid.UUID
	CostPerDay   float64
	TokensPerDay float64
	CostPerMonth float64
}

// EvaluateAggregates evalua alertas de métricas agregadas (cost/tokens per day/month)
// para TODAS las orgs con alertas activas. Se invoca desde un cron/leader worker.
func (s *Service) EvaluateAggregates(ctx context.Context) (int, error) {
	alerts, err := s.listAggregateAlerts(ctx)
	if err != nil {
		return 0, fmt.Errorf("list aggregate alerts: %w", err)
	}
	if len(alerts) == 0 {
		return 0, nil
	}

	// Single-org: compute aggregates once for all alerts.
	agg, err := s.computeAggregates(ctx, uuid.Nil)
	if err != nil {
		return 0, fmt.Errorf("compute aggregates: %w", err)
	}

	// Evaluate and fire
	fired := 0
	for _, a := range alerts {
		var observed float64
		switch a.Metric {
		case MetricCostPerDay:
			observed = agg.CostPerDay
		case MetricTokensPerDay:
			observed = agg.TokensPerDay
		case MetricCostPerMonth:
			observed = agg.CostPerMonth
		default:
			continue
		}
		if CompareThreshold(observed, a.Threshold, a.Condition) && !s.inCooldown(&a) {
			_ = s.recordFire(ctx, &a, observed, map[string]any{
				"observed": observed,
				"evaluated_at": time.Now().UTC().Format(time.RFC3339),
			})
			if a.Channel == ChannelEmail && len(a.Recipients) > 0 && s.EmailSender != nil {
				s.sendEmailAlertAsync(a, observed)
			}
			fired++
		}
	}
	return fired, nil
}

// listAggregateAlerts retorna alertas activas con métricas agregadas.
func (s *Service) listAggregateAlerts(ctx context.Context) ([]Alert, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, name, metric, threshold, condition, channel,
			recipients, cooldown_secs, active, last_fired_at, fire_count,
			created_at, updated_at
		 FROM usage_alerts
		 WHERE active = TRUE
		   AND metric IN ($1, $2, $3)`,
		MetricCostPerDay, MetricTokensPerDay, MetricCostPerMonth)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Alert
	for rows.Next() {
		var a Alert
		if err := rows.Scan(&a.ID, &a.Name, &a.Metric,
			&a.Threshold, &a.Condition, &a.Channel, &a.Recipients,
			&a.CooldownSecs, &a.Active, &a.LastFiredAt, &a.FireCount,
			&a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// computeAggregates obtiene cost/tokens del día y del mes para una org.
func (s *Service) computeAggregates(ctx context.Context, orgID uuid.UUID) (*aggregateResult, error) {
	var r aggregateResult
	r.OrgID = orgID

	err := s.Pool.QueryRow(ctx,
		`SELECT
			COALESCE(SUM(total_cost_usd), 0),
			COALESCE(SUM(total_tokens), 0)
		FROM agent_runs
		WHERE created_at >= CURRENT_DATE
		  AND status = 'completed'`,
	).Scan(&r.CostPerDay, &r.TokensPerDay)
	if err != nil {
		return nil, fmt.Errorf("daily aggregates: %w", err)
	}

	err = s.Pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(total_cost_usd), 0)
		FROM agent_runs
		WHERE created_at >= DATE_TRUNC('month', CURRENT_DATE)
		  AND status = 'completed'`,
	).Scan(&r.CostPerMonth)
	if err != nil {
		return nil, fmt.Errorf("monthly aggregates: %w", err)
	}

	return &r, nil
}
