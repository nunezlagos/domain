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

const webhookCols = `id, organization_id, slug, name, source_type,
	target_type, target_id, inputs_mapping, enabled, last_delivery_at`

// GetByID lookup sin descifrar secret (para management; el secret nunca sale).
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Webhook, error) {
	var w Webhook
	err := s.Pool.QueryRow(ctx,
		`SELECT `+webhookCols+` FROM webhooks WHERE id = $1 AND deleted_at IS NULL`, id,
	).Scan(&w.ID, &w.OrganizationID, &w.Slug, &w.Name, &w.SourceType,
		&w.TargetType, &w.TargetID, &w.InputsMapping, &w.Enabled, &w.LastDeliveryAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get webhook: %w", err)
	}
	return &w, nil
}

// List webhooks de la org (sin secrets).
func (s *Service) List(ctx context.Context, orgID uuid.UUID) ([]Webhook, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT `+webhookCols+` FROM webhooks
		 WHERE organization_id = $1 AND deleted_at IS NULL
		 ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list webhooks: %w", err)
	}
	defer rows.Close()
	var out []Webhook
	for rows.Next() {
		var w Webhook
		if err := rows.Scan(&w.ID, &w.OrganizationID, &w.Slug, &w.Name, &w.SourceType,
			&w.TargetType, &w.TargetID, &w.InputsMapping, &w.Enabled, &w.LastDeliveryAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// SetEnabled toggle.
func (s *Service) SetEnabled(ctx context.Context, id uuid.UUID, enabled bool) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE webhooks SET enabled = $1 WHERE id = $2 AND deleted_at IS NULL`,
		enabled, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SoftDelete con audit.
func (s *Service) SoftDelete(ctx context.Context, id, actorID uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE webhooks SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
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
	rows, err := s.Pool.Query(ctx,
		`SELECT id, webhook_id, payload, headers, COALESCE(source_ip,''),
		        status, COALESCE(error,''), triggered_run_id, received_at
		 FROM webhook_deliveries WHERE webhook_id = $1
		 ORDER BY received_at DESC LIMIT $2`, webhookID, limit)
	if err != nil {
		return nil, fmt.Errorf("deliveries: %w", err)
	}
	defer rows.Close()
	var out []Delivery
	for rows.Next() {
		var d Delivery
		if err := rows.Scan(&d.ID, &d.WebhookID, &d.Payload, &d.Headers, &d.SourceIP,
			&d.Status, &d.Error, &d.TriggeredRunID, &d.ReceivedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// GetDelivery una delivery puntual (para replay).
func (s *Service) GetDelivery(ctx context.Context, id uuid.UUID) (*Delivery, error) {
	var d Delivery
	err := s.Pool.QueryRow(ctx,
		`SELECT id, webhook_id, payload, headers, COALESCE(source_ip,''),
		        status, COALESCE(error,''), triggered_run_id, received_at
		 FROM webhook_deliveries WHERE id = $1`, id,
	).Scan(&d.ID, &d.WebhookID, &d.Payload, &d.Headers, &d.SourceIP,
		&d.Status, &d.Error, &d.TriggeredRunID, &d.ReceivedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get delivery: %w", err)
	}
	return &d, nil
}
