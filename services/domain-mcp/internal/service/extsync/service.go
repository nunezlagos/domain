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

	"nunezlagos/domain/internal/service/extsync/extsyncdb"
	"nunezlagos/domain/internal/store/txctx"
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

	DirPushOnly      = "push_only"
	DirPullOnly      = "pull_only"
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
	ID              uuid.UUID       `json:"id"`
	ProviderID      uuid.UUID       `json:"provider_id"`
	EntityKind      string          `json:"entity_kind"`
	EntityID        uuid.UUID       `json:"entity_id"`
	ExternalKey     string          `json:"external_key"`
	ExternalURL     string          `json:"external_url"`
	ExternalType    *string         `json:"external_type,omitempty"`
	SyncDirection   string          `json:"sync_direction"`
	SyncStatus      string          `json:"sync_status"`
	FieldMapping    json.RawMessage `json:"field_mapping"`
	LastPushedAt    *time.Time      `json:"last_pushed_at,omitempty"`
	LastPulledAt    *time.Time      `json:"last_pulled_at,omitempty"`
	LastSyncedAt    *time.Time      `json:"last_synced_at,omitempty"`
	DriftDetectedAt *time.Time      `json:"drift_detected_at,omitempty"`
	DriftFields     json.RawMessage `json:"drift_fields,omitempty"`
	PartialFailures json.RawMessage `json:"partial_failures,omitempty"`
	RetryCount      int             `json:"retry_count"`
	NextRetryAt     *time.Time      `json:"next_retry_at,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type Service struct {
	Pool *pgxpool.Pool
}

var validProviders = map[string]bool{
	ProviderJira: true, ProviderGitHub: true,
	ProviderLinear: true, ProviderAsana: true,
}

func (s *Service) q(ctx context.Context) *extsyncdb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return extsyncdb.New(tx)
	}
	return extsyncdb.New(s.Pool)
}

func syncStateFromRow(row extsyncdb.ExternalSyncState) SyncState {
	st := SyncState{
		ID:              row.ID,
		ProviderID:      row.ProviderID,
		EntityKind:      row.EntityKind,
		EntityID:        row.EntityID,
		ExternalKey:     row.ExternalKey,
		ExternalURL:     row.ExternalUrl,
		ExternalType:    row.ExternalType,
		SyncDirection:   row.SyncDirection,
		SyncStatus:      row.SyncStatus,
		FieldMapping:    json.RawMessage(row.FieldMapping),
		DriftFields:     json.RawMessage(row.DriftFields),
		PartialFailures: json.RawMessage(row.PartialFailures),
		RetryCount:      int(row.RetryCount),
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
	if row.LastPushedAt.Valid {
		st.LastPushedAt = &row.LastPushedAt.Time
	}
	if row.LastPulledAt.Valid {
		st.LastPulledAt = &row.LastPulledAt.Time
	}
	if row.LastSyncedAt.Valid {
		st.LastSyncedAt = &row.LastSyncedAt.Time
	}
	if row.DriftDetectedAt.Valid {
		st.DriftDetectedAt = &row.DriftDetectedAt.Time
	}
	if row.NextRetryAt.Valid {
		st.NextRetryAt = &row.NextRetryAt.Time
	}
	return st
}

func providerFromRow(row extsyncdb.RegisterProviderRow) Provider {
	return Provider{
		ID:          row.ID,
		Provider:    row.Provider,
		DisplayName: row.DisplayName,
		BaseURL:     row.BaseUrl,
		ProjectKey:  row.ProjectKey,
		Config:      json.RawMessage(row.Config),
		Enabled:     row.Enabled,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

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

	row, err := s.q(ctx).RegisterProvider(ctx, extsyncdb.RegisterProviderParams{
		Provider:    provider,
		DisplayName: displayName,
		BaseUrl:     baseURL,
		ProjectKey:  pk,
		Config:      cfgJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("register provider: %w", err)
	}
	p := providerFromRow(row)
	return &p, nil
}

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

	row, err := s.q(ctx).RegisterPush(ctx, extsyncdb.RegisterPushParams{
		ProviderID:    providerID,
		EntityKind:    entityKind,
		EntityID:      entityID,
		ExternalKey:   externalKey,
		ExternalUrl:   externalURL,
		ExternalType:  et,
		SyncDirection: DirPushOnly,
		SyncStatus:    StatusOK,
		FieldMapping:  fmJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("register sync state: %w", err)
	}
	st := syncStateFromRow(row)
	s.recordEvent(ctx, st.ID, "push.ok", "push", map[string]any{
		"external_key": externalKey, "external_type": externalType,
	}, "")
	return &st, nil
}

func (s *Service) MarkDrift(ctx context.Context, stateID uuid.UUID, driftFields map[string]any) (*SyncState, error) {
	dfJSON, _ := json.Marshal(driftFields)
	err := s.q(ctx).MarkDrift(ctx, extsyncdb.MarkDriftParams{
		SyncStatus:  StatusConflict,
		DriftFields: dfJSON,
		ID:          stateID,
	})
	if err != nil {
		return nil, fmt.Errorf("mark drift: %w", err)
	}
	s.recordEvent(ctx, stateID, "drift.detected", "pull", driftFields, "")
	return s.Get(ctx, stateID)
}

func (s *Service) MarkPartial(ctx context.Context, stateID uuid.UUID, failures []any) (*SyncState, error) {
	fJSON, _ := json.Marshal(failures)
	err := s.q(ctx).MarkPartial(ctx, extsyncdb.MarkPartialParams{
		SyncStatus:      StatusPartial,
		PartialFailures: fJSON,
		ID:              stateID,
	})
	if err != nil {
		return nil, fmt.Errorf("mark partial: %w", err)
	}
	s.recordEvent(ctx, stateID, "push.partial", "push", map[string]any{"failures": len(failures)}, "")
	return s.Get(ctx, stateID)
}

func (s *Service) MarkResolved(ctx context.Context, stateID uuid.UUID) (*SyncState, error) {
	err := s.q(ctx).MarkResolved(ctx, extsyncdb.MarkResolvedParams{
		SyncStatus: StatusOK,
		ID:         stateID,
	})
	if err != nil {
		return nil, fmt.Errorf("mark resolved: %w", err)
	}
	s.recordEvent(ctx, stateID, "conflict.resolved", "push", nil, "")
	return s.Get(ctx, stateID)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*SyncState, error) {
	row, err := s.q(ctx).GetSyncState(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrStateNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get state: %w", err)
	}
	st := syncStateFromRow(row)
	return &st, nil
}

func (s *Service) GetByEntity(ctx context.Context, providerID uuid.UUID, entityKind string, entityID uuid.UUID) (*SyncState, error) {
	row, err := s.q(ctx).GetByEntity(ctx, extsyncdb.GetByEntityParams{
		ProviderID: providerID,
		EntityKind: entityKind,
		EntityID:   entityID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrStateNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get by entity: %w", err)
	}
	st := syncStateFromRow(row)
	return &st, nil
}

func (s *Service) ListConflicts(ctx context.Context, limit int) ([]SyncState, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.q(ctx).ListConflicts(ctx, extsyncdb.ListConflictsParams{
		SyncStatus: StatusConflict,
		Limit:      int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list conflicts: %w", err)
	}
	out := make([]SyncState, len(rows))
	for i, row := range rows {
		out[i] = syncStateFromRow(row)
	}
	return out, nil
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
	_ = s.q(ctx).InsertSyncEvent(ctx, extsyncdb.InsertSyncEventParams{
		SyncStateID:  stateID,
		EventType:    eventType,
		Direction:    direction,
		Payload:      pJSON,
		ErrorMessage: em,
	})
}
