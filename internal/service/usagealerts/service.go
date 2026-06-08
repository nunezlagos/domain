// Package usagealerts — HU-15.3 alerts configurables sobre cost/tokens.
//
// Métricas soportadas:
//   - cost_per_run     — cost USD de un agent_run individual
//   - cost_per_day     — total cost USD del día (org)
//   - cost_per_month   — total cost USD del mes actual (org)
//   - tokens_per_run   — tokens totales de un agent_run
//   - tokens_per_day   — tokens totales del día
//
// Evaluación: cada agent_run completion + cron periódico para métricas agregadas.
// Cooldown previene spam: alert no se re-dispara hasta cooldown_secs después.
package usagealerts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUnknown          = errors.New("not_found")
	ErrInvalidMetric    = errors.New("invalid_metric")
	ErrInvalidCondition = errors.New("invalid_condition")
)

// Metric enum de las métricas soportadas.
const (
	MetricCostPerRun    = "cost_per_run"
	MetricCostPerDay    = "cost_per_day"
	MetricCostPerMonth  = "cost_per_month"
	MetricTokensPerRun  = "tokens_per_run"
	MetricTokensPerDay  = "tokens_per_day"
)

var validMetrics = map[string]bool{
	MetricCostPerRun: true, MetricCostPerDay: true, MetricCostPerMonth: true,
	MetricTokensPerRun: true, MetricTokensPerDay: true,
}

const (
	ConditionGT = "greater_than"
	ConditionLT = "less_than"
	ConditionEQ = "equals"
)

var validConditions = map[string]bool{
	ConditionGT: true, ConditionLT: true, ConditionEQ: true,
}

const (
	ChannelWebhook = "webhook"
	ChannelEmail   = "email"
	ChannelLogOnly = "log_only"
)

// Alert es la config de una regla.
type Alert struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	Name           string     `json:"name"`
	Metric         string     `json:"metric"`
	Threshold      float64    `json:"threshold"`
	Condition      string     `json:"condition"`
	Channel        string     `json:"channel"`
	Recipients     []string   `json:"recipients"`
	CooldownSecs   int        `json:"cooldown_secs"`
	Active         bool       `json:"active"`
	LastFiredAt    *time.Time `json:"last_fired_at,omitempty"`
	FireCount      int        `json:"fire_count"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// CreateInput para POST.
type CreateInput struct {
	Name         string
	Metric       string
	Threshold    float64
	Condition    string
	Channel      string
	Recipients   []string
	CooldownSecs int
}

// Service operaciones CRUD + evaluate.
type Service struct {
	Pool *pgxpool.Pool
}

func (s *Service) Create(ctx context.Context, orgID uuid.UUID, in CreateInput) (*Alert, error) {
	if !validMetrics[in.Metric] {
		return nil, ErrInvalidMetric
	}
	if in.Condition == "" {
		in.Condition = ConditionGT
	}
	if !validConditions[in.Condition] {
		return nil, ErrInvalidCondition
	}
	if in.Channel == "" {
		in.Channel = ChannelWebhook
	}
	if in.Threshold < 0 {
		return nil, fmt.Errorf("threshold must be >= 0")
	}
	if in.CooldownSecs <= 0 {
		in.CooldownSecs = 3600
	}
	row := s.Pool.QueryRow(ctx,
		`INSERT INTO usage_alerts
			(organization_id, name, metric, threshold, condition, channel,
			 recipients, cooldown_secs)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 RETURNING id, created_at, updated_at`,
		orgID, in.Name, in.Metric, in.Threshold, in.Condition, in.Channel,
		in.Recipients, in.CooldownSecs)
	a := &Alert{
		OrganizationID: orgID, Name: in.Name, Metric: in.Metric,
		Threshold: in.Threshold, Condition: in.Condition, Channel: in.Channel,
		Recipients: in.Recipients, CooldownSecs: in.CooldownSecs, Active: true,
	}
	if err := row.Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return nil, fmt.Errorf("insert: %w", err)
	}
	return a, nil
}

func (s *Service) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]Alert, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, organization_id, name, metric, threshold, condition, channel,
			recipients, cooldown_secs, active, last_fired_at, fire_count,
			created_at, updated_at
		 FROM usage_alerts WHERE organization_id = $1 ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Alert
	for rows.Next() {
		var a Alert
		if err := rows.Scan(&a.ID, &a.OrganizationID, &a.Name, &a.Metric,
			&a.Threshold, &a.Condition, &a.Channel, &a.Recipients,
			&a.CooldownSecs, &a.Active, &a.LastFiredAt, &a.FireCount,
			&a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Service) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	ct, err := s.Pool.Exec(ctx,
		`DELETE FROM usage_alerts WHERE id=$1 AND organization_id=$2`, id, orgID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrUnknown
	}
	return nil
}

func (s *Service) SetActive(ctx context.Context, orgID, id uuid.UUID, active bool) error {
	ct, err := s.Pool.Exec(ctx,
		`UPDATE usage_alerts SET active=$1 WHERE id=$2 AND organization_id=$3`,
		active, id, orgID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrUnknown
	}
	return nil
}

// CompareThreshold evalúa la condición. true → debe disparar.
func CompareThreshold(observed, threshold float64, condition string) bool {
	switch condition {
	case ConditionGT:
		return observed > threshold
	case ConditionLT:
		return observed < threshold
	case ConditionEQ:
		return observed == threshold
	}
	return false
}

// EvaluateRunEvent evalúa alertas per-run (cost_per_run, tokens_per_run) tras
// un agent_run completed. Retorna las alertas que dispararon.
func (s *Service) EvaluateRunEvent(ctx context.Context, orgID uuid.UUID,
	costUSD float64, tokensTotal int64) ([]Alert, error) {

	rows, err := s.Pool.Query(ctx,
		`SELECT id, organization_id, name, metric, threshold, condition, channel,
			recipients, cooldown_secs, active, last_fired_at, fire_count,
			created_at, updated_at
		 FROM usage_alerts
		 WHERE organization_id = $1 AND active = TRUE
		   AND metric IN ($2, $3)`,
		orgID, MetricCostPerRun, MetricTokensPerRun)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var alerts []Alert
	for rows.Next() {
		var a Alert
		if err := rows.Scan(&a.ID, &a.OrganizationID, &a.Name, &a.Metric,
			&a.Threshold, &a.Condition, &a.Channel, &a.Recipients,
			&a.CooldownSecs, &a.Active, &a.LastFiredAt, &a.FireCount,
			&a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		var observed float64
		switch a.Metric {
		case MetricCostPerRun:
			observed = costUSD
		case MetricTokensPerRun:
			observed = float64(tokensTotal)
		}
		if CompareThreshold(observed, a.Threshold, a.Condition) && !s.inCooldown(&a) {
			alerts = append(alerts, a)
			_ = s.recordFire(ctx, &a, observed, map[string]any{
				"cost_usd": costUSD, "tokens_total": tokensTotal,
			})
		}
	}
	return alerts, rows.Err()
}

// inCooldown true si LastFiredAt + Cooldown > now.
func (s *Service) inCooldown(a *Alert) bool {
	if a.LastFiredAt == nil {
		return false
	}
	return a.LastFiredAt.Add(time.Duration(a.CooldownSecs) * time.Second).After(time.Now())
}

func (s *Service) recordFire(ctx context.Context, a *Alert, observed float64, extra map[string]any) error {
	payload, _ := json.Marshal(extra)
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO usage_alert_fires
			(alert_id, organization_id, metric, threshold, observed_value, payload)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		a.ID, a.OrganizationID, a.Metric, a.Threshold, observed, payload)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx,
		`UPDATE usage_alerts SET last_fired_at=NOW(), fire_count=fire_count+1 WHERE id=$1`, a.ID)
	return err
}

// Get devuelve un alert por id+org (cross-org guard).
func (s *Service) Get(ctx context.Context, orgID, id uuid.UUID) (*Alert, error) {
	row := s.Pool.QueryRow(ctx,
		`SELECT id, organization_id, name, metric, threshold, condition, channel,
			recipients, cooldown_secs, active, last_fired_at, fire_count,
			created_at, updated_at
		 FROM usage_alerts WHERE id=$1 AND organization_id=$2`, id, orgID)
	var a Alert
	err := row.Scan(&a.ID, &a.OrganizationID, &a.Name, &a.Metric,
		&a.Threshold, &a.Condition, &a.Channel, &a.Recipients,
		&a.CooldownSecs, &a.Active, &a.LastFiredAt, &a.FireCount,
		&a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUnknown
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}
