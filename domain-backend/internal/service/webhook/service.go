// Package webhook — issue-10.2 inbound webhooks.
//
// Cliente externo (GitHub, GitLab, generic) hace POST /webhooks/:slug?token=...
// Domain verifica HMAC y dispatchea target (flow/agent/skill).
//
// Secret se cifra at-rest con crypto.AESGCM (issue-02.3). Cada delivery se
// persiste en webhook_deliveries para auditoría.
package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/crypto"
)

var (
	ErrSlugInvalid       = errors.New("slug must be lowercase ascii, digits, dashes")
	ErrSlugTaken         = errors.New("slug already taken")
	ErrInvalidSourceType = errors.New("source_type must be generic|github|gitlab|bitbucket")
	ErrInvalidTargetType = errors.New("target_type must be flow|agent|skill")
	ErrNotFound          = errors.New("webhook not found")
	ErrSignatureInvalid  = errors.New("HMAC signature invalid")
)

var (
	reSlug         = regexp.MustCompile(`^[a-z][a-z0-9-]{0,98}[a-z0-9]$|^[a-z]$`)
	validSources   = map[string]bool{"generic": true, "github": true, "gitlab": true, "bitbucket": true}
	validTargets   = map[string]bool{"flow": true, "agent": true, "skill": true}
)

type Webhook struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Slug           string
	Name           string
	SourceType     string
	TargetType     string
	TargetID       uuid.UUID
	InputsMapping  map[string]any
	Enabled        bool
	LastDeliveryAt *time.Time
}

type CreateInput struct {
	OrganizationID uuid.UUID
	CreatedBy      *uuid.UUID
	Slug           string
	Name           string
	Secret         string // plaintext, se cifra antes de persistir
	SourceType     string
	TargetType     string
	TargetID       uuid.UUID
	InputsMapping  map[string]any
	ActorID        uuid.UUID
}

type Service struct {
	Pool   *pgxpool.Pool
	Audit  audit.Recorder
	Crypto *crypto.Cipher // para cifrar secret at-rest
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*Webhook, error) {
	if !reSlug.MatchString(in.Slug) {
		return nil, ErrSlugInvalid
	}
	if !validSources[in.SourceType] {
		return nil, ErrInvalidSourceType
	}
	if !validTargets[in.TargetType] {
		return nil, ErrInvalidTargetType
	}
	if in.Secret == "" {
		return nil, errors.New("secret required")
	}

	encSecret, err := s.Crypto.Encrypt([]byte(in.Secret))
	if err != nil {
		return nil, fmt.Errorf("encrypt secret: %w", err)
	}
	if in.InputsMapping == nil {
		in.InputsMapping = map[string]any{}
	}
	mappingJSON, _ := json.Marshal(in.InputsMapping)

	var w Webhook
	err = s.Pool.QueryRow(ctx,
		`INSERT INTO webhooks
		   (organization_id, created_by, slug, name, secret_encrypted, source_type,
		    target_type, target_id, inputs_mapping)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, organization_id, slug, name, source_type, target_type, target_id,
		           inputs_mapping, enabled, last_delivery_at`,
		in.OrganizationID, in.CreatedBy, in.Slug, in.Name, encSecret, in.SourceType,
		in.TargetType, in.TargetID, mappingJSON,
	).Scan(&w.ID, &w.OrganizationID, &w.Slug, &w.Name, &w.SourceType, &w.TargetType, &w.TargetID,
		&w.InputsMapping, &w.Enabled, &w.LastDeliveryAt)
	if err != nil {
		if strings.Contains(err.Error(), "webhooks_organization_id_slug_key") ||
			strings.Contains(err.Error(), "duplicate key") {
			return nil, ErrSlugTaken
		}
		return nil, fmt.Errorf("insert webhook: %w", err)
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
			OrganizationID: &in.OrganizationID,
			ActorID:        &in.ActorID,
			ActorType:      audit.ActorUser,
			Action:         "webhook.created",
			EntityType:     "webhook",
			EntityID:       &w.ID,
			NewValues:      map[string]any{"slug": w.Slug, "target_type": w.TargetType},
		})
	}
	return &w, nil
}

// ResolveBySlug busca webhook + descifra secret para verificar HMAC.
func (s *Service) ResolveBySlug(ctx context.Context, slug string) (*Webhook, []byte, error) {
	var w Webhook
	var encSecret []byte
	err := s.Pool.QueryRow(ctx,
		`SELECT id, organization_id, slug, name, secret_encrypted, source_type,
		        target_type, target_id, inputs_mapping, enabled, last_delivery_at
		 FROM webhooks WHERE slug = $1 AND deleted_at IS NULL`, slug,
	).Scan(&w.ID, &w.OrganizationID, &w.Slug, &w.Name, &encSecret, &w.SourceType,
		&w.TargetType, &w.TargetID, &w.InputsMapping, &w.Enabled, &w.LastDeliveryAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("query webhook: %w", err)
	}
	if !w.Enabled {
		return nil, nil, ErrNotFound
	}
	secret, err := s.Crypto.Decrypt(encSecret)
	if err != nil {
		return nil, nil, fmt.Errorf("decrypt secret: %w", err)
	}
	return &w, secret, nil
}

// VerifyHMAC verifica una signature HMAC-SHA256 sobre el body.
// signatureHex viene del header (e.g. X-Hub-Signature-256 de GitHub).
func VerifyHMAC(secret, body []byte, signatureHex string) bool {
	// GitHub usa prefix "sha256="
	sig := strings.TrimPrefix(signatureHex, "sha256=")
	expected, err := hex.DecodeString(sig)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	actual := mac.Sum(nil)
	return hmac.Equal(expected, actual)
}

// RecordDelivery persiste un webhook_delivery (status + run_id + error).
func (s *Service) RecordDelivery(ctx context.Context, webhookID uuid.UUID,
	payload []byte, headers map[string]string, sourceIP, status string,
	triggeredRunID *uuid.UUID, errStr string) error {
	headersJSON, _ := json.Marshal(headers)
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO webhook_deliveries
		   (webhook_id, payload, headers, source_ip, status, error, triggered_run_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		webhookID, payload, headersJSON, nullStr(sourceIP), status,
		nullStr(errStr), triggeredRunID)
	if err != nil {
		return fmt.Errorf("record delivery: %w", err)
	}
	_, _ = s.Pool.Exec(ctx,
		`UPDATE webhooks SET last_delivery_at = NOW() WHERE id = $1`, webhookID)
	return nil
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
