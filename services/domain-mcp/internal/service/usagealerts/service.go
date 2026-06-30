// Package usagealerts — issue-15.3 alerts configurables sobre cost/tokens.
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
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/service/usagealerts/usagealertsdb"
	"nunezlagos/domain/internal/store/txctx"
)

var (
	ErrUnknown          = errors.New("not_found")
	ErrInvalidMetric    = errors.New("invalid_metric")
	ErrInvalidCondition = errors.New("invalid_condition")
)

// Metric enum de las métricas soportadas.
const (
	MetricCostPerRun   = "cost_per_run"
	MetricCostPerDay   = "cost_per_day"
	MetricCostPerMonth = "cost_per_month"
	MetricTokensPerRun = "tokens_per_run"
	MetricTokensPerDay = "tokens_per_day"
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

// EmailSender envía alertas por email (issue-15.3 channel email handler).
type EmailSender interface {
	SendAlertEmail(ctx context.Context, to []string, subject, body string) error
}

// Alert es la config de una regla.
type Alert struct {
	ID           uuid.UUID  `json:"id"`
	Name         string     `json:"name"`
	Metric       string     `json:"metric"`
	Threshold    float64    `json:"threshold"`
	Condition    string     `json:"condition"`
	Channel      string     `json:"channel"`
	Recipients   []string   `json:"recipients"`
	CooldownSecs int        `json:"cooldown_secs"`
	Active       bool       `json:"active"`
	LastFiredAt  *time.Time `json:"last_fired_at,omitempty"`
	FireCount    int        `json:"fire_count"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
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

// UpdateInput para PATCH — punteros para detectar campos a actualizar.
type UpdateInput struct {
	Name         *string
	Metric       *string
	Threshold    *float64
	Condition    *string
	Channel      *string
	Recipients   []string // nil = no update, empty slice = clear
	CooldownSecs *int
}

// AlertFire representa un disparo registrado de una alerta.
type AlertFire struct {
	ID            uuid.UUID      `json:"id"`
	AlertID       uuid.UUID      `json:"alert_id"`
	Metric        string         `json:"metric"`
	Threshold     float64        `json:"threshold"`
	ObservedValue float64        `json:"observed_value"`
	Payload       map[string]any `json:"payload,omitempty"`
	FiredAt       time.Time      `json:"fired_at"`
}

// Service operaciones CRUD + evaluate.
type Service struct {
	Pool        *pgxpool.Pool
	EmailSender EmailSender
	Logger      *slog.Logger
}

func (s *Service) q(ctx context.Context) *usagealertsdb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return usagealertsdb.New(tx)
	}
	return usagealertsdb.New(s.Pool)
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
	row, err := s.q(ctx).InsertAlert(ctx, usagealertsdb.InsertAlertParams{
		Name:         in.Name,
		Metric:       in.Metric,
		Threshold:    in.Threshold,
		Condition:    in.Condition,
		Channel:      in.Channel,
		Recipients:   in.Recipients,
		CooldownSecs: int32(in.CooldownSecs),
	})
	if err != nil {
		return nil, fmt.Errorf("insert: %w", err)
	}
	a := &Alert{
		ID:           row.ID,
		Name:         in.Name,
		Metric:       in.Metric,
		Threshold:    in.Threshold,
		Condition:    in.Condition,
		Channel:      in.Channel,
		Recipients:   in.Recipients,
		CooldownSecs: in.CooldownSecs,
		Active:       true,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
	return a, nil
}

func (s *Service) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]Alert, error) {
	rows, err := s.q(ctx).ListAlerts(ctx)
	if err != nil {
		return nil, err
	}
	var out []Alert
	for _, r := range rows {
		out = append(out, toAlertFromList(r))
	}
	return out, nil
}

func (s *Service) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	n, err := s.q(ctx).DeleteAlert(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrUnknown
	}
	return nil
}

func (s *Service) SetActive(ctx context.Context, orgID, id uuid.UUID, active bool) error {
	n, err := s.q(ctx).SetAlertActive(ctx, usagealertsdb.SetAlertActiveParams{ID: id, Active: active})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrUnknown
	}
	return nil
}

// Update actualiza campos del alert. Solo envía campos no-nil.
func (s *Service) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput) (*Alert, error) {
	if in.Name == nil && in.Metric == nil && in.Threshold == nil &&
		in.Condition == nil && in.Channel == nil && in.Recipients == nil &&
		in.CooldownSecs == nil {
		return s.Get(ctx, orgID, id)
	}

	var cooldown *int32
	if in.CooldownSecs != nil {
		c := int32(*in.CooldownSecs)
		cooldown = &c
	}

	row, err := s.q(ctx).UpdateAlert(ctx, usagealertsdb.UpdateAlertParams{
		ID:           id,
		Name:         in.Name,
		Metric:       in.Metric,
		Threshold:    in.Threshold,
		Condition:    in.Condition,
		Channel:      in.Channel,
		Recipients:   in.Recipients,
		CooldownSecs: cooldown,
	})
	if err != nil {
		return nil, fmt.Errorf("update: %w", err)
	}
	a := toAlertFromUpdate(row)
	return &a, nil
}

// ListFires devuelve el historial de disparos de una alerta.
func (s *Service) ListFires(ctx context.Context, orgID, alertID uuid.UUID, limit int) ([]AlertFire, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.q(ctx).ListAlertFires(ctx, usagealertsdb.ListAlertFiresParams{
		AlertID: alertID,
		Limit:   int32(limit),
	})
	if err != nil {
		return nil, err
	}
	var out []AlertFire
	for _, r := range rows {
		f := AlertFire{
			ID:            r.ID,
			AlertID:       r.AlertID,
			Metric:        r.Metric,
			Threshold:     r.Threshold,
			ObservedValue: r.ObservedValue,
			FiredAt:       r.FiredAt,
		}
		if len(r.Payload) > 0 {
			_ = json.Unmarshal(r.Payload, &f.Payload)
		}
		out = append(out, f)
	}
	return out, nil
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

	rows, err := s.q(ctx).ListActiveAlertsByMetrics(ctx,
		[]string{MetricCostPerRun, MetricTokensPerRun})
	if err != nil {
		return nil, err
	}
	var alerts []Alert
	for _, r := range rows {
		a := toAlertFromActive(r)
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
			if a.Channel == ChannelEmail && len(a.Recipients) > 0 && s.EmailSender != nil {
				s.sendEmailAlertAsync(a, observed)
			}
		}
	}
	return alerts, nil
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
	q := s.q(ctx)
	if err := q.InsertAlertFire(ctx, usagealertsdb.InsertAlertFireParams{
		AlertID:       a.ID,
		Metric:        a.Metric,
		Threshold:     a.Threshold,
		ObservedValue: observed,
		Payload:       payload,
	}); err != nil {
		return err
	}
	return q.TouchAlertFired(ctx, a.ID)
}

// Get devuelve un alert por id+org (cross-org guard).
func (s *Service) Get(ctx context.Context, orgID, id uuid.UUID) (*Alert, error) {
	row, err := s.q(ctx).GetAlert(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUnknown
	}
	if err != nil {
		return nil, err
	}
	a := toAlertFromGet(row)
	return &a, nil
}

// mapping generado -> dominio. Las distintas row structs comparten la misma
// forma de Alert; cada adaptador delega en buildAlert.
func buildAlert(id uuid.UUID, name, metric string, threshold float64,
	condition, channel string, recipients []string, cooldownSecs int32,
	active bool, lastFiredAt *time.Time, fireCount int32,
	createdAt, updatedAt time.Time) Alert {
	return Alert{
		ID:           id,
		Name:         name,
		Metric:       metric,
		Threshold:    threshold,
		Condition:    condition,
		Channel:      channel,
		Recipients:   recipients,
		CooldownSecs: int(cooldownSecs),
		Active:       active,
		LastFiredAt:  lastFiredAt,
		FireCount:    int(fireCount),
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}
}

func toAlertFromList(r usagealertsdb.ListAlertsRow) Alert {
	return buildAlert(r.ID, r.Name, r.Metric, r.Threshold, r.Condition, r.Channel,
		r.Recipients, r.CooldownSecs, r.Active, r.LastFiredAt, r.FireCount,
		r.CreatedAt, r.UpdatedAt)
}

func toAlertFromGet(r usagealertsdb.GetAlertRow) Alert {
	return buildAlert(r.ID, r.Name, r.Metric, r.Threshold, r.Condition, r.Channel,
		r.Recipients, r.CooldownSecs, r.Active, r.LastFiredAt, r.FireCount,
		r.CreatedAt, r.UpdatedAt)
}

func toAlertFromUpdate(r usagealertsdb.UpdateAlertRow) Alert {
	return buildAlert(r.ID, r.Name, r.Metric, r.Threshold, r.Condition, r.Channel,
		r.Recipients, r.CooldownSecs, r.Active, r.LastFiredAt, r.FireCount,
		r.CreatedAt, r.UpdatedAt)
}

func toAlertFromActive(r usagealertsdb.ListActiveAlertsByMetricsRow) Alert {
	return buildAlert(r.ID, r.Name, r.Metric, r.Threshold, r.Condition, r.Channel,
		r.Recipients, r.CooldownSecs, r.Active, r.LastFiredAt, r.FireCount,
		r.CreatedAt, r.UpdatedAt)
}

func (s *Service) sendEmailAlertAsync(a Alert, observed float64) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		subject := fmt.Sprintf("Usage Alert: %s exceeded threshold", a.Name)
		body := fmt.Sprintf(
			"Alert: %s\nMetric: %s\nThreshold: %.4f\nObserved: %.4f\nChannel: %s\n\nThis is an automated alert from Domain.",
			a.Name, a.Metric, a.Threshold, observed, a.Channel,
		)

		if err := s.EmailSender.SendAlertEmail(ctx, a.Recipients, subject, body); err != nil {
			if s.Logger != nil {
				s.Logger.Warn("failed to send alert email",
					slog.String("alert_id", a.ID.String()),
					slog.String("error", err.Error()),
				)
			}
		}
	}()
}
