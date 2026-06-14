package notifications

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Sender persiste cada envío en notification_deliveries.
type Sender struct {
	Pool     *pgxpool.Pool
	Registry *Registry
	Logger   *slog.Logger
}

// Send invoca el canal + persiste resultado en delivery log.
// Devuelve el ID del delivery row para tracking.
func (s *Sender) Send(ctx context.Context, channelSlug string, msg Message) (uuid.UUID, error) {
	ch, err := s.Registry.Get(channelSlug)
	if err != nil {
		return uuid.Nil, err
	}
	if msg.Body == "" {
		return uuid.Nil, fmt.Errorf("%w: body required", ErrInvalidMessage)
	}
	deliveryID := uuid.New()

	// Insert pre-envío con status=pending.
	if _, err := s.Pool.Exec(ctx,
		`INSERT INTO notification_deliveries
			(id, organization_id, channel_slug, recipient, template_slug, subject, body, status, attempt)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,'pending',1)`,
		deliveryID, msg.OrganizationID, channelSlug, msg.Recipient,
		nullableSlug(msg.TemplateSlug), nullableString(msg.Subject), msg.Body); err != nil {
		return uuid.Nil, fmt.Errorf("insert delivery: %w", err)
	}

	start := time.Now()
	res, err := ch.Send(ctx, msg)
	latency := int(time.Since(start) / time.Millisecond)

	status := res.Status
	errMsg := res.ErrorMessage
	if err != nil {
		status = "failed"
		if errMsg == "" {
			errMsg = err.Error()
		}
	}
	if status == "" {
		status = "sent"
	}

	var deliveredAt *time.Time
	if status == "sent" {
		t := time.Now()
		deliveredAt = &t
	}

	_, _ = s.Pool.Exec(ctx,
		`UPDATE notification_deliveries
		 SET status=$1, response_code=$2, error_message=$3, latency_ms=$4, delivered_at=$5
		 WHERE id=$6`,
		status, nullableInt(res.ResponseCode), nullableString(errMsg),
		latency, deliveredAt, deliveryID)

	if s.Logger != nil {
		s.Logger.InfoContext(ctx, "notification sent",
			slog.String("channel", channelSlug),
			slog.String("status", status),
			slog.Int("latency_ms", latency))
	}

	if err != nil {
		return deliveryID, err
	}
	return deliveryID, nil
}

func nullableSlug(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func nullableInt(n int) any {
	if n == 0 {
		return nil
	}
	return n
}
