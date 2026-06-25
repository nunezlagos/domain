// Management de webhooks inbound — issue-10.2 (CRUD admin + deliveries).
package webhook

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/service/webhook/webhookdb"
)

// Delivery entrada del log webhook_deliveries.
type Delivery struct {
	ID             uuid.UUID      `json:"id"`
	WebhookID      uuid.UUID      `json:"webhook_id"`
	Payload        map[string]any `json:"payload"`
	Headers        map[string]any `json:"headers"`
	SourceIP       string         `json:"source_ip,omitempty"`
	Status         string         `json:"status"`
	Error          string         `json:"error,omitempty"`
	TriggeredRunID *uuid.UUID     `json:"triggered_run_id,omitempty"`
	ReceivedAt     time.Time      `json:"received_at"`
}

// GetByID lookup sin descifrar secret (para management; el secret nunca sale).
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Webhook, error) {
	row, err := s.q(ctx).GetWebhookByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get webhook: %w", err)
	}
	w := webhookFromRow(row)
	return &w, nil
}

// List webhooks de la org (sin secrets).
func (s *Service) List(ctx context.Context, orgID uuid.UUID) ([]Webhook, error) {
	rows, err := s.q(ctx).ListWebhooks(ctx)
	if err != nil {
		return nil, fmt.Errorf("list webhooks: %w", err)
	}
	out := make([]Webhook, len(rows))
	for i, row := range rows {
		out[i] = webhookFromRow(row)
	}
	return out, nil
}

// SetEnabled toggle.
func (s *Service) SetEnabled(ctx context.Context, id uuid.UUID, enabled bool) error {
	_, err := s.q(ctx).SetWebhookEnabled(ctx, webhookdb.SetWebhookEnabledParams{
		Enabled: enabled,
		ID:      id,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// SoftDelete con audit.
func (s *Service) SoftDelete(ctx context.Context, id, actorID uuid.UUID) error {
	_, err := s.q(ctx).SoftDeleteWebhook(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorID: &actorID, ActorType: audit.ActorUser,
			Action: "webhook.deleted", EntityType: "webhook", EntityID: &id,
		})
	}
	return nil
}

// Deliveries historial de un webhook, más reciente primero.
func (s *Service) Deliveries(ctx context.Context, webhookID uuid.UUID, limit int) ([]Delivery, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.q(ctx).ListDeliveries(ctx, webhookdb.ListDeliveriesParams{
		WebhookID: webhookID,
		Limit:     int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("deliveries: %w", err)
	}
	out := make([]Delivery, len(rows))
	for i, row := range rows {
		out[i] = Delivery{
			ID:             row.ID,
			WebhookID:      row.WebhookID,
			Payload:        unmarshalMap(row.Payload),
			Headers:        unmarshalMap(row.Headers),
			SourceIP:       row.SourceIp,
			Status:         row.Status,
			Error:          row.Error,
			TriggeredRunID: row.TriggeredRunID,
			ReceivedAt:     row.ReceivedAt,
		}
	}
	return out, nil
}

// GetDelivery una delivery puntual (para replay).
func (s *Service) GetDelivery(ctx context.Context, id uuid.UUID) (*Delivery, error) {
	row, err := s.q(ctx).GetDelivery(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get delivery: %w", err)
	}
	return &Delivery{
		ID:             row.ID,
		WebhookID:      row.WebhookID,
		Payload:        unmarshalMap(row.Payload),
		Headers:        unmarshalMap(row.Headers),
		SourceIP:       row.SourceIp,
		Status:         row.Status,
		Error:          row.Error,
		TriggeredRunID: row.TriggeredRunID,
		ReceivedAt:     row.ReceivedAt,
	}, nil
}
