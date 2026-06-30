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
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/crypto"
	"nunezlagos/domain/internal/service/webhook/webhookdb"
	"nunezlagos/domain/internal/store/txctx"
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

func (s *Service) q(ctx context.Context) *webhookdb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return webhookdb.New(tx)
	}
	return webhookdb.New(s.Pool)
}

func webhookFromRow(row webhookdb.Webhook) Webhook {
	w := Webhook{
		ID:         row.ID,
		Slug:       row.Slug,
		Name:       row.Name,
		SourceType: row.SourceType,
		TargetType: row.TargetType,
		TargetID:   row.TargetID,
		Enabled:    row.Enabled,
	}
	if len(row.InputsMapping) > 0 {
		_ = json.Unmarshal(row.InputsMapping, &w.InputsMapping)
	}
	if w.InputsMapping == nil {
		w.InputsMapping = map[string]any{}
	}
	if row.LastDeliveryAt.Valid {
		w.LastDeliveryAt = &row.LastDeliveryAt.Time
	}
	return w
}

func unmarshalMap(b []byte) map[string]any {
	var m map[string]any
	if len(b) > 0 {
		_ = json.Unmarshal(b, &m)
	}
	if m == nil {
		m = map[string]any{}
	}
	return m
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
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

	row, err := s.q(ctx).InsertWebhook(ctx, webhookdb.InsertWebhookParams{
		CreatedBy:       in.CreatedBy,
		Slug:            in.Slug,
		Name:            in.Name,
		SecretEncrypted: encSecret,
		SourceType:      in.SourceType,
		TargetType:      in.TargetType,
		TargetID:        in.TargetID,
		InputsMapping:   mappingJSON,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return nil, ErrSlugTaken
		}
		return nil, fmt.Errorf("insert webhook: %w", err)
	}
	w := webhookFromRow(row)
	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
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
	row, err := s.q(ctx).GetWebhookBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("query webhook: %w", err)
	}
	if !row.Enabled {
		return nil, nil, ErrNotFound
	}
	secret, err := s.Crypto.Decrypt(row.SecretEncrypted)
	if err != nil {
		return nil, nil, fmt.Errorf("decrypt secret: %w", err)
	}
	w := webhookFromRow(row)
	return &w, secret, nil
}

// VerifyHMAC verifica una signature HMAC-SHA256 sobre el body.
// signatureHex viene del header (e.g. X-Hub-Signature-256 de GitHub).
func VerifyHMAC(secret, body []byte, signatureHex string) bool {

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

	err := s.q(ctx).InsertDelivery(ctx, webhookdb.InsertDeliveryParams{
		WebhookID:      webhookID,
		Payload:        payload,
		Headers:        headersJSON,
		SourceIp:       strPtr(sourceIP),
		Status:         status,
		Error:          strPtr(errStr),
		TriggeredRunID: triggeredRunID,
	})
	if err != nil {
		return fmt.Errorf("record delivery: %w", err)
	}
	_ = s.q(ctx).UpdateLastDelivery(ctx, webhookID)
	return nil
}
