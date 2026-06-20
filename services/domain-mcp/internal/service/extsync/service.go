// Package extsync — issue-04.9 external provider sync state tracking.
//
// MVP scope: schema + state CRUD + drift marker. Los drivers reales (Jira HTTP
// client, GitHub Issues API, webhooks pull) viven en HUs siguientes que
// implementarán la interface ExternalProviderDriver consumida por workers.
package extsync

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
	ErrProviderNotFound = errors.New("external provider not found")
	ErrStateNotFound    = errors.New("sync state not found")
	ErrInvalidProvider  = errors.New("invalid provider")
	ErrInvalidEntity    = errors.New("invalid entity_kind")
)

const (
	ProviderJira   = "jira"
	ProviderGitHub = "github"
	ProviderLinear = "linear"
	ProviderAsana  = "asana"

	EntityREQ = "req"
	EntityHU  = "hu"

	DirPushOnly    = "push_only"
	DirPullOnly    = "pull_only"
	DirBidirectional = "bidirectional"

	StatusPending  = "pending"
	StatusOK       = "ok"
	StatusPartial  = "partial"
	StatusConflict = "conflict"
	StatusDisabled = "disabled"
	StatusFailed   = "failed"
)

type Provider struct {
	ID             uuid.UUID       `json:"id"`
	OrganizationID uuid.UUID       `json:"organization_id"`
	Provider       string          `json:"provider"`
	DisplayName    string          `json:"display_name"`
	BaseURL        string          `json:"base_url"`
	ProjectKey     *string         `json:"project_key,omitempty"`
	Config         json.RawMessage `json:"config"`
	Enabled        bool            `json:"enabled"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type SyncState struct {
	ID                 uuid.UUID       `json:"id"`
	ProviderID         uuid.UUID       `json:"provider_id"`
	EntityKind         string          `json:"entity_kind"`
	EntityID           uuid.UUID       `json:"entity_id"`
	ExternalKey        string          `json:"external_key"`
	ExternalURL        string          `json:"external_url"`
	ExternalType       *string         `json:"external_type,omitempty"`
	SyncDirection      string          `json:"sync_direction"`
	SyncStatus         string          `json:"sync_status"`
	FieldMapping       json.RawMessage `json:"field_mapping"`
	LastPushedAt       *time.Time      `json:"last_pushed_at,omitempty"`
	LastPulledAt       *time.Time      `json:"last_pulled_at,omitempty"`
	LastSyncedAt       *time.Time      `json:"last_synced_at,omitempty"`
	DriftDetectedAt    *time.Time      `json:"drift_detected_at,omitempty"`
	DriftFields        json.RawMessage `json:"drift_fields,omitempty"`
	PartialFailures    json.RawMessage `json:"partial_failures,omitempty"`
	RetryCount         int             `json:"retry_count"`
	NextRetryAt        *time.Time      `json:"next_retry_at,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

type Service struct {
	Pool *pgxpool.Pool
}

var validProviders = map[string]bool{
	ProviderJira: true, ProviderGitHub: true,
	ProviderLinear: true, ProviderAsana: true,
}

// RegisterProvider crea o actualiza un provider.
func (s *Service) RegisterProvider(ctx context.Context, orgID uuid.UUID, provider, displayName, baseURL, projectKey string, config map[string]any) (*Provider, error) {
	if !validProviders[provider] {
		return nil, fmt.Errorf("%w: %s", ErrInvalidProvider, provider)
	}
	if config == nil {
		config = map[string]any{}
	}
	cfgJSON, _ := json.Marshal(config)
	var pk *string
	if projectKey != "" {
		pk = &projectKey
	}

	var p Provider
	// ISSUE-21.6 Fase D clean Round 3: organization_id se omite del INSERT
	// (la columna es nullable post-migration 000145; single-org = NULL).
	// El UNIQUE constraint external_providers_org_provider_unique se dropeó
	// en 000145 — el caller garantiza unicidad via app.
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO external_providers (provider, display_name, base_url, project_key, config)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (provider, project_key)
		DO UPDATE SET display_name = EXCLUDED.display_name,
		              base_url = EXCLUDED.base_url,
		              config = EXCLUDED.config,
		              updated_at = now()
		RETURNING id, provider, display_name, base_url, project_key,
		          config, enabled, created_at, updated_at`,
		provider, displayName, baseURL, pk, cfgJSON,
	).Scan(&p.ID, &p.Provider, &p.DisplayName, &p.BaseURL,
		&p.ProjectKey, &p.Config, &p.Enabled, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("register provider: %w", err)
	}
	return &p, nil
}

// RegisterPush registra un sync state nuevo después de un push exitoso inicial.
func (s *Service) RegisterPush(ctx context.Context, providerID uuid.UUID, entityKind string, entityID uuid.UUID, externalKey, externalURL, externalType string, fieldMapping map[string]any) (*SyncState, error) {
	if entityKind != EntityREQ && entityKind != EntityHU {
		return nil, fmt.Errorf("%w: %s", ErrInvalidEntity, entityKind)
	}
	if fieldMapping == nil {
		fieldMapping = map[string]any{}
	}
	fmJSON, _ := json.Marshal(fieldMapping)
	var et *string
	if externalType != "" {
		et = &externalType
	}

	var st SyncState
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO external_sync_state (provider_id, entity_kind, entity_id, external_key,
		                                  external_url, external_type, sync_direction,
		                                  sync_status, field_mapping, last_pushed_at,
		                                  last_synced_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now(), now())
		RETURNING id, provider_id, entity_kind, entity_id, external_key, external_url,
		          external_type, sync_direction, sync_status, field_mapping,
		          last_pushed_at, last_pulled_at, last_synced_at, drift_detected_at,
		          drift_fields, partial_failures, retry_count, next_retry_at,
		          created_at, updated_at`,
		providerID, entityKind, entityID, externalKey, externalURL, et,
		DirPushOnly, StatusOK, fmJSON,
	).Scan(scanStateCols(&st)...)
	if err != nil {
		return nil, fmt.Errorf("register sync state: %w", err)
	}
	s.recordEvent(ctx, st.ID, "push.ok", "push", map[string]any{
		"external_key": externalKey, "external_type": externalType,
	}, "")
	return &st, nil
}

// MarkDrift marca un sync state como conflicto por edición externa.
func (s *Service) MarkDrift(ctx context.Context, stateID uuid.UUID, driftFields map[string]any) (*SyncState, error) {
	dfJSON, _ := json.Marshal(driftFields)
	_, err := s.Pool.Exec(ctx, `
		UPDATE external_sync_state
		SET sync_status = $1, drift_detected_at = now(), drift_fields = $2,
		    updated_at = now()
		WHERE id = $3`,
		StatusConflict, dfJSON, stateID,
	)
	if err != nil {
		return nil, fmt.Errorf("mark drift: %w", err)
	}
	s.recordEvent(ctx, stateID, "drift.detected", "pull", driftFields, "")
	return s.Get(ctx, stateID)
}

// MarkPartial marca push parcial (algunos attachments fallaron).
func (s *Service) MarkPartial(ctx context.Context, stateID uuid.UUID, failures []any) (*SyncState, error) {
	fJSON, _ := json.Marshal(failures)
	_, err := s.Pool.Exec(ctx, `
		UPDATE external_sync_state
		SET sync_status = $1, partial_failures = $2, updated_at = now()
		WHERE id = $3`,
		StatusPartial, fJSON, stateID,
	)
	if err != nil {
		return nil, fmt.Errorf("mark partial: %w", err)
	}
	s.recordEvent(ctx, stateID, "push.partial", "push", map[string]any{"failures": len(failures)}, "")
	return s.Get(ctx, stateID)
}

// MarkResolved resuelve un conflict cuando humano elige una versión.
func (s *Service) MarkResolved(ctx context.Context, stateID uuid.UUID) (*SyncState, error) {
	_, err := s.Pool.Exec(ctx, `
		UPDATE external_sync_state
		SET sync_status = $1, drift_detected_at = NULL, drift_fields = NULL,
		    last_synced_at = now(), updated_at = now()
		WHERE id = $2`,
		StatusOK, stateID,
	)
	if err != nil {
		return nil, fmt.Errorf("mark resolved: %w", err)
	}
	s.recordEvent(ctx, stateID, "conflict.resolved", "push", nil, "")
	return s.Get(ctx, stateID)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*SyncState, error) {
	var st SyncState
	err := s.Pool.QueryRow(ctx, `
		SELECT id, provider_id, entity_kind, entity_id, external_key, external_url,
		       external_type, sync_direction, sync_status, field_mapping,
		       last_pushed_at, last_pulled_at, last_synced_at, drift_detected_at,
		       drift_fields, partial_failures, retry_count, next_retry_at,
		       created_at, updated_at
		FROM external_sync_state WHERE id = $1`, id,
	).Scan(scanStateCols(&st)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrStateNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get state: %w", err)
	}
	return &st, nil
}

// GetByEntity busca sync_state por (entity_kind, entity_id, provider).
func (s *Service) GetByEntity(ctx context.Context, providerID uuid.UUID, entityKind string, entityID uuid.UUID) (*SyncState, error) {
	var st SyncState
	err := s.Pool.QueryRow(ctx, `
		SELECT id, provider_id, entity_kind, entity_id, external_key, external_url,
		       external_type, sync_direction, sync_status, field_mapping,
		       last_pushed_at, last_pulled_at, last_synced_at, drift_detected_at,
		       drift_fields, partial_failures, retry_count, next_retry_at,
		       created_at, updated_at
		FROM external_sync_state
		WHERE provider_id = $1 AND entity_kind = $2 AND entity_id = $3`,
		providerID, entityKind, entityID,
	).Scan(scanStateCols(&st)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrStateNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get by entity: %w", err)
	}
	return &st, nil
}

// ListConflicts retorna sync states con drift sin resolver.
func (s *Service) ListConflicts(ctx context.Context, limit int) ([]SyncState, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id, provider_id, entity_kind, entity_id, external_key, external_url,
		       external_type, sync_direction, sync_status, field_mapping,
		       last_pushed_at, last_pulled_at, last_synced_at, drift_detected_at,
		       drift_fields, partial_failures, retry_count, next_retry_at,
		       created_at, updated_at
		FROM external_sync_state
		WHERE sync_status = $1
		ORDER BY drift_detected_at DESC NULLS LAST LIMIT $2`,
		StatusConflict, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list conflicts: %w", err)
	}
	defer rows.Close()

	var out []SyncState
	for rows.Next() {
		var st SyncState
		if err := rows.Scan(scanStateCols(&st)...); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

func (s *Service) recordEvent(ctx context.Context, stateID uuid.UUID, eventType, direction string, payload map[string]any, errMsg string) {
	if payload == nil {
		payload = map[string]any{}
	}
	pJSON, _ := json.Marshal(payload)
	var em *string
	if errMsg != "" {
		em = &errMsg
	}
	_, _ = s.Pool.Exec(ctx, `
		INSERT INTO external_sync_events (sync_state_id, event_type, direction, payload, error_message)
		VALUES ($1, $2, $3, $4, $5)`,
		stateID, eventType, direction, pJSON, em,
	)
}

func scanStateCols(st *SyncState) []any {
	return []any{
		&st.ID, &st.ProviderID, &st.EntityKind, &st.EntityID,
		&st.ExternalKey, &st.ExternalURL, &st.ExternalType,
		&st.SyncDirection, &st.SyncStatus, &st.FieldMapping,
		&st.LastPushedAt, &st.LastPulledAt, &st.LastSyncedAt,
		&st.DriftDetectedAt, &st.DriftFields, &st.PartialFailures,
		&st.RetryCount, &st.NextRetryAt,
		&st.CreatedAt, &st.UpdatedAt,
	}
}
