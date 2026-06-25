package outboundwebhook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/crypto"
	"nunezlagos/domain/internal/service/outboundwebhook/outboundwebhookdb"
)

var (
	ErrInvalidURL   = errors.New("invalid_url")
	ErrNoEvents     = errors.New("events_required")
	ErrInvalidEvent = errors.New("invalid_event")
	ErrUnknown      = errors.New("not_found")
	ErrSSRF         = errors.New("ssrf_blocked")
)

var AllowedEvents = map[string]bool{
	"agent_run.completed": true,
	"agent_run.failed":    true,
	"flow_run.completed":  true,
	"flow_run.failed":     true,
	"observation.created": true,
	"invitation.accepted": true,
	"invite.created":      true,
	"webhook.test_ping":   true,
	"usage.alert_fired":   true,
}

type Subscription struct {
	ID            uuid.UUID       `json:"id"`
	Name          string          `json:"name"`
	URL           string          `json:"url"`
	Events        []string        `json:"events"`
	Filters       json.RawMessage `json:"filters"`
	Active        bool            `json:"active"`
	FailureCount  int             `json:"failure_count"`
	LastSuccessAt *time.Time      `json:"last_success_at,omitempty"`
	LastFailureAt *time.Time      `json:"last_failure_at,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type CreateInput struct {
	Name    string
	URL     string
	Events  []string
	Filters json.RawMessage
	Secret  string
}

type Service struct {
	Pool   *pgxpool.Pool
	Cipher *crypto.Cipher
}

func ValidateURL(raw string, requireTLS bool) error {
	u, err := url.Parse(raw)
	if err != nil {
		return ErrInvalidURL
	}
	if requireTLS {
		if u.Scheme != "https" {
			return ErrInvalidURL
		}
	} else {
		if u.Scheme != "http" && u.Scheme != "https" {
			return ErrInvalidURL
		}
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return ErrInvalidURL
	}
	if host == "localhost" || host == "127.0.0.1" || host == "::1" ||
		strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return ErrSSRF
	}

	if strings.HasPrefix(host, "10.") ||
		strings.HasPrefix(host, "192.168.") ||
		strings.HasPrefix(host, "169.254.") {
		return ErrSSRF
	}
	if strings.HasPrefix(host, "172.") {
		parts := strings.Split(host, ".")
		if len(parts) >= 2 {
			var oct int
			fmt.Sscanf(parts[1], "%d", &oct)
			if oct >= 16 && oct <= 31 {
				return ErrSSRF
			}
		}
	}
	return nil
}

func toSubscription(id uuid.UUID, name, url string, events []string, filters []byte, active bool, failureCount int32, lastSuccessAt, lastFailureAt *time.Time, createdAt, updatedAt time.Time) Subscription {
	return Subscription{
		ID:            id,
		Name:          name,
		URL:           url,
		Events:        events,
		Filters:       json.RawMessage(filters),
		Active:        active,
		FailureCount:  int(failureCount),
		LastSuccessAt: lastSuccessAt,
		LastFailureAt: lastFailureAt,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
	}
}

func toSubscriptionFromGet(r outboundwebhookdb.GetByIDRow) Subscription {
	return toSubscription(r.ID, r.Name, r.Url, r.Events, r.Filters, r.Active, r.FailureCount, r.LastSuccessAt, r.LastFailureAt, r.CreatedAt, r.UpdatedAt)
}

func toSubscriptionFromList(r outboundwebhookdb.ListByEventRow) Subscription {
	return toSubscription(r.ID, r.Name, r.Url, r.Events, r.Filters, r.Active, r.FailureCount, r.LastSuccessAt, r.LastFailureAt, r.CreatedAt, r.UpdatedAt)
}

func toSubscriptionFromListAll(r outboundwebhookdb.ListAllRow) Subscription {
	return toSubscription(r.ID, r.Name, r.Url, r.Events, r.Filters, r.Active, r.FailureCount, r.LastSuccessAt, r.LastFailureAt, r.CreatedAt, r.UpdatedAt)
}

func (s *Service) Create(ctx context.Context, orgID uuid.UUID, in CreateInput, requireTLS bool) (*Subscription, error) {
	if err := ValidateURL(in.URL, requireTLS); err != nil {
		return nil, err
	}
	if len(in.Events) == 0 {
		return nil, ErrNoEvents
	}
	for _, e := range in.Events {
		if !AllowedEvents[e] {
			return nil, fmt.Errorf("%w: %s", ErrInvalidEvent, e)
		}
	}
	if in.Filters == nil {
		in.Filters = json.RawMessage(`{}`)
	}
	var secretCipher []byte
	if in.Secret != "" {
		if s.Cipher == nil {
			return nil, errors.New("cipher_required_for_secret")
		}
		ct, err := s.Cipher.Encrypt([]byte(in.Secret))
		if err != nil {
			return nil, fmt.Errorf("cipher: %w", err)
		}
		secretCipher = ct
	}

	q := outboundwebhookdb.New(s.Pool)
	row, err := q.InsertSubscription(ctx, outboundwebhookdb.InsertSubscriptionParams{
		OrganizationID: orgID,
		Name:           in.Name,
		Url:            in.URL,
		Events:         in.Events,
		Filters:        in.Filters,
		SecretCipher:   secretCipher,
	})
	if err != nil {
		return nil, fmt.Errorf("insert subscription: %w", err)
	}
	return &Subscription{
		ID:       row.ID,
		Name:     in.Name,
		URL:      in.URL,
		Events:   in.Events,
		Filters:  in.Filters,
		Active:   true,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}, nil
}

func (s *Service) ListByEvent(ctx context.Context, orgID uuid.UUID, eventType string) ([]Subscription, error) {
	rows, err := outboundwebhookdb.New(s.Pool).ListByEvent(ctx, eventType)
	if err != nil {
		return nil, err
	}
	out := make([]Subscription, len(rows))
	for i, r := range rows {
		out[i] = toSubscriptionFromList(r)
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, orgID, id uuid.UUID) (*Subscription, error) {
	row, err := outboundwebhookdb.New(s.Pool).GetByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUnknown
	}
	if err != nil {
		return nil, err
	}
	sub := toSubscriptionFromGet(row)
	return &sub, nil
}

func (s *Service) ListAll(ctx context.Context, orgID uuid.UUID) ([]Subscription, error) {
	rows, err := outboundwebhookdb.New(s.Pool).ListAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Subscription, len(rows))
	for i, r := range rows {
		out[i] = toSubscriptionFromListAll(r)
	}
	return out, nil
}

func (s *Service) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	n, err := outboundwebhookdb.New(s.Pool).DeleteSubscription(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrUnknown
	}
	return nil
}

func (s *Service) getByID(ctx context.Context, id uuid.UUID) (*Subscription, error) {
	row, err := outboundwebhookdb.New(s.Pool).GetByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUnknown
	}
	if err != nil {
		return nil, err
	}
	sub := toSubscriptionFromGet(row)
	return &sub, nil
}

func (s *Service) GetByIDInternal(ctx context.Context, id uuid.UUID) (*Subscription, error) {
	return s.getByID(ctx, id)
}

func (s *Service) DecryptSecret(ctx context.Context, id uuid.UUID) ([]byte, error) {
	ct, err := outboundwebhookdb.New(s.Pool).GetSecretCipher(ctx, id)
	if err != nil {
		return nil, err
	}
	if len(ct) == 0 || s.Cipher == nil {
		return nil, nil
	}
	return s.Cipher.Decrypt(ct)
}
