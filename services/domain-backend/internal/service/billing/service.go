// Package billing — issue-21.3 usage tracking sobre usage_counters.
//
// REQ-42.2: el dominio de planes/cuotas (tabla plans) se eliminó — producto
// single-org sin facturación. Este paquete conserva SOLO la observabilidad de
// uso sobre usage_counters (consumo por período mensual, period_start = primer
// día del mes UTC).
//
// Lifecycle:
//   - IncrementTokens / IncrementRuns: actualiza contador atómico
//   - GetUsage: lee el contador del período actual
package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrOrgNotFound   = errors.New("organization not found")
	ErrQuotaExceeded = errors.New("quota exceeded")
)

type Usage struct {
	OrganizationID uuid.UUID
	PeriodStart    time.Time
	TokensUsed     int64
	RunsCount      int32
	StorageBytes   int64
	CostUSD        float64
	Warned80       bool
	Warned100      bool
}

// Limits combina plan + custom_limits (custom override gana).
type Limits struct {
	TokensPerMonth *int64
	RunsPerMonth   *int32
	StorageGBMax   *int32
	MembersMax     *int32
}

type Service struct {
	Pool *pgxpool.Pool
	Now  func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now().UTC()
}

// ResolveLimits devuelve las cuotas vigentes.
//
// REQ-42.2: el dominio de planes se eliminó (single-org sin facturación).
// Sin plans ni custom_limits, devolvemos límites vacíos (sin caps).
func (s *Service) ResolveLimits(ctx context.Context, orgID uuid.UUID) (*Limits, error) {
	_ = ctx
	_ = orgID
	return &Limits{}, nil
}

// IncrementTokens suma tokens al período actual; UPSERT idempotente.
func (s *Service) IncrementTokens(ctx context.Context, orgID uuid.UUID, n int64) (*Usage, error) {
	if n < 0 {
		return nil, fmt.Errorf("n must be >= 0")
	}
	period := monthStart(s.now())
	return s.incrementCounter(ctx, orgID, period, n, 0, 0)
}

// IncrementRuns suma 1 run al contador.
func (s *Service) IncrementRuns(ctx context.Context, orgID uuid.UUID) (*Usage, error) {
	period := monthStart(s.now())
	return s.incrementCounter(ctx, orgID, period, 0, 1, 0)
}

// incrementCounter: single-org global. usage_counters keyed por period_start
// (sin organization_id). El param orgID se conserva por compat de signatura.
func (s *Service) incrementCounter(ctx context.Context, orgID uuid.UUID, period time.Time, tokens int64, runs int32, storage int64) (*Usage, error) {
	_ = orgID
	var u Usage
	err := s.Pool.QueryRow(ctx, `
INSERT INTO usage_counters (period_start, tokens_used, runs_count, storage_bytes)
VALUES ($1, $2, $3, $4)
ON CONFLICT (period_start) DO UPDATE
  SET tokens_used   = usage_counters.tokens_used + EXCLUDED.tokens_used,
      runs_count    = usage_counters.runs_count + EXCLUDED.runs_count,
      storage_bytes = usage_counters.storage_bytes + EXCLUDED.storage_bytes
RETURNING period_start, tokens_used, runs_count, storage_bytes,
          cost_usd, warned_80pct, warned_100pct`,
		period, tokens, runs, storage,
	).Scan(&u.PeriodStart, &u.TokensUsed, &u.RunsCount, &u.StorageBytes,
		&u.CostUSD, &u.Warned80, &u.Warned100)
	if err != nil {
		return nil, fmt.Errorf("upsert counter: %w", err)
	}
	return &u, nil
}

// GetUsage del período actual (mes corriente). Single-org global: keyed por
// period_start. El param orgID se conserva por compat de signatura.
func (s *Service) GetUsage(ctx context.Context, orgID uuid.UUID) (*Usage, error) {
	_ = orgID
	period := monthStart(s.now())
	var u Usage
	err := s.Pool.QueryRow(ctx,
		`SELECT period_start, tokens_used, runs_count, storage_bytes,
		        cost_usd, warned_80pct, warned_100pct
		 FROM usage_counters
		 WHERE period_start = $1`,
		period,
	).Scan(&u.PeriodStart, &u.TokensUsed, &u.RunsCount, &u.StorageBytes,
		&u.CostUSD, &u.Warned80, &u.Warned100)
	if errors.Is(err, pgx.ErrNoRows) {
		return &Usage{PeriodStart: period}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get usage: %w", err)
	}
	return &u, nil
}

// LimitState resultado de CheckLimit.
type LimitState struct {
	Dimension    string // "tokens" | "runs" | "storage" | "members"
	Used         int64
	Limit        int64 // 0 si ilimitado
	Unlimited    bool
	RatioUsed    float64 // 0..1+
	SoftLimitHit bool    // ratio >= soft_limit_ratio
	HardLimitHit bool    // ratio >= 1.0
}

// CheckTokens valida si la org puede consumir N tokens más.
// Si HardLimitHit, retorna ErrQuotaExceeded además del state.
func (s *Service) CheckTokens(ctx context.Context, orgID uuid.UUID, additional int64) (*LimitState, error) {
	limits, err := s.ResolveLimits(ctx, orgID)
	if err != nil {
		return nil, err
	}
	usage, err := s.GetUsage(ctx, orgID)
	if err != nil {
		return nil, err
	}
	state := &LimitState{Dimension: "tokens", Used: usage.TokensUsed + additional}
	if limits.TokensPerMonth == nil {
		state.Unlimited = true
		return state, nil
	}
	state.Limit = *limits.TokensPerMonth
	if state.Limit > 0 {
		state.RatioUsed = float64(state.Used) / float64(state.Limit)
	}
	state.SoftLimitHit = state.RatioUsed >= 0.8
	state.HardLimitHit = state.RatioUsed >= 1.0
	if state.HardLimitHit {
		return state, ErrQuotaExceeded
	}
	return state, nil
}

// monthStart devuelve el primer día del mes UTC en time.Time.
func monthStart(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), 1, 0, 0, 0, 0, time.UTC)
}
