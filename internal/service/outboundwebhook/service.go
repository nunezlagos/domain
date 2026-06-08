// Package outboundwebhook — HU-10.4 outbound webhooks.
//
// Permite suscribir endpoints HTTPS a eventos del sistema (agent_run.completed,
// flow_run.completed, observation.created, etc.) y entrega async con HMAC signing
// + retry exponencial + DLQ.
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
)

var (
	ErrInvalidURL    = errors.New("invalid_url")
	ErrNoEvents      = errors.New("events_required")
	ErrInvalidEvent  = errors.New("invalid_event")
	ErrUnknown       = errors.New("not_found")
	ErrSSRF          = errors.New("ssrf_blocked")
)

// AllowedEvents catalog cerrado para evitar typos del cliente.
var AllowedEvents = map[string]bool{
	"agent_run.completed":  true,
	"agent_run.failed":     true,
	"flow_run.completed":   true,
	"flow_run.failed":      true,
	"observation.created":  true,
	"invitation.accepted":  true,
	"webhook.test_ping":    true,
}

type Subscription struct {
	ID             uuid.UUID       `json:"id"`
	OrganizationID uuid.UUID       `json:"organization_id"`
	Name           string          `json:"name"`
	URL            string          `json:"url"`
	Events         []string        `json:"events"`
	Filters        json.RawMessage `json:"filters"`
	Active         bool            `json:"active"`
	FailureCount   int             `json:"failure_count"`
	LastSuccessAt  *time.Time      `json:"last_success_at,omitempty"`
	LastFailureAt  *time.Time      `json:"last_failure_at,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
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

// ValidateURL bloquea esquemas no http/https + hosts internos (SSRF prevention).
//
// Política: solo https en prod (env DOMAIN_OUTBOUND_REQUIRE_TLS=true), http permitido
// para dev. Bloquea localhost, .local, IPs privadas (10.*, 172.16-31.*, 192.168.*, fe80::*).
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
	// IPv4 privadas
	if strings.HasPrefix(host, "10.") ||
		strings.HasPrefix(host, "192.168.") ||
		strings.HasPrefix(host, "169.254.") {
		return ErrSSRF
	}
	if strings.HasPrefix(host, "172.") {
		// 172.16.0.0/12 → segundo octeto 16-31
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

// Create persiste una nueva subscription. Si secret != "" lo cifra at-rest.
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

	row := s.Pool.QueryRow(ctx,
		`INSERT INTO outbound_webhook_subscriptions
			(organization_id, name, url, events, filters, secret_cipher)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 RETURNING id, created_at, updated_at`,
		orgID, in.Name, in.URL, in.Events, in.Filters, secretCipher)
	sub := &Subscription{
		OrganizationID: orgID,
		Name:           in.Name,
		URL:            in.URL,
		Events:         in.Events,
		Filters:        in.Filters,
		Active:         true,
	}
	if err := row.Scan(&sub.ID, &sub.CreatedAt, &sub.UpdatedAt); err != nil {
		return nil, fmt.Errorf("insert subscription: %w", err)
	}
	return sub, nil
}

// ListByEvent devuelve subscriptions activas de la org que matchean el event_type.
// La función dispatch verifica filtros adicionales por payload.
func (s *Service) ListByEvent(ctx context.Context, orgID uuid.UUID, eventType string) ([]Subscription, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, organization_id, name, url, events, filters, active,
			failure_count, last_success_at, last_failure_at, created_at, updated_at
		 FROM outbound_webhook_subscriptions
		 WHERE organization_id = $1 AND active = TRUE
		   AND $2 = ANY(events)`,
		orgID, eventType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Subscription
	for rows.Next() {
		var sub Subscription
		if err := rows.Scan(&sub.ID, &sub.OrganizationID, &sub.Name, &sub.URL,
			&sub.Events, &sub.Filters, &sub.Active, &sub.FailureCount,
			&sub.LastSuccessAt, &sub.LastFailureAt, &sub.CreatedAt, &sub.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, sub)
	}
	return out, rows.Err()
}

func (s *Service) Get(ctx context.Context, orgID, id uuid.UUID) (*Subscription, error) {
	row := s.Pool.QueryRow(ctx,
		`SELECT id, organization_id, name, url, events, filters, active,
			failure_count, last_success_at, last_failure_at, created_at, updated_at
		 FROM outbound_webhook_subscriptions
		 WHERE id = $1 AND organization_id = $2`, id, orgID)
	var sub Subscription
	err := row.Scan(&sub.ID, &sub.OrganizationID, &sub.Name, &sub.URL, &sub.Events,
		&sub.Filters, &sub.Active, &sub.FailureCount, &sub.LastSuccessAt,
		&sub.LastFailureAt, &sub.CreatedAt, &sub.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUnknown
	}
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (s *Service) ListAll(ctx context.Context, orgID uuid.UUID) ([]Subscription, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, organization_id, name, url, events, filters, active,
			failure_count, last_success_at, last_failure_at, created_at, updated_at
		 FROM outbound_webhook_subscriptions
		 WHERE organization_id = $1 ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Subscription
	for rows.Next() {
		var sub Subscription
		if err := rows.Scan(&sub.ID, &sub.OrganizationID, &sub.Name, &sub.URL, &sub.Events,
			&sub.Filters, &sub.Active, &sub.FailureCount, &sub.LastSuccessAt,
			&sub.LastFailureAt, &sub.CreatedAt, &sub.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, sub)
	}
	return out, rows.Err()
}

func (s *Service) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	ct, err := s.Pool.Exec(ctx,
		`DELETE FROM outbound_webhook_subscriptions WHERE id=$1 AND organization_id=$2`,
		id, orgID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrUnknown
	}
	return nil
}

// getByID lookup interno sin scope de org (uso del dispatcher).
func (s *Service) getByID(ctx context.Context, id uuid.UUID) (*Subscription, error) {
	row := s.Pool.QueryRow(ctx,
		`SELECT id, organization_id, name, url, events, filters, active,
			failure_count, last_success_at, last_failure_at, created_at, updated_at
		 FROM outbound_webhook_subscriptions WHERE id = $1`, id)
	var sub Subscription
	err := row.Scan(&sub.ID, &sub.OrganizationID, &sub.Name, &sub.URL, &sub.Events,
		&sub.Filters, &sub.Active, &sub.FailureCount, &sub.LastSuccessAt,
		&sub.LastFailureAt, &sub.CreatedAt, &sub.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUnknown
	}
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// GetByIDInternal exposes getByID for the dispatcher.
func (s *Service) GetByIDInternal(ctx context.Context, id uuid.UUID) (*Subscription, error) {
	return s.getByID(ctx, id)
}

// DecryptSecret devuelve el secret en plain para signing. Solo se usa en dispatcher.
func (s *Service) DecryptSecret(ctx context.Context, id uuid.UUID) ([]byte, error) {
	var ct []byte
	err := s.Pool.QueryRow(ctx,
		`SELECT secret_cipher FROM outbound_webhook_subscriptions WHERE id=$1`, id).Scan(&ct)
	if err != nil {
		return nil, err
	}
	if len(ct) == 0 || s.Cipher == nil {
		return nil, nil
	}
	return s.Cipher.Decrypt(ct)
}
