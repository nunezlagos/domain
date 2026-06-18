// Package billing — issue-21.3 plans + usage tracking + limit enforcement.
//
// Plans son globales (Free/Pro/Enterprise + custom). Cada org tiene plan_id +
// custom_limits JSONB que pueden override. usage_counters tracks consumo por
// período mensual (period_start = primer día del mes UTC).
//
// Lifecycle:
//   - IncrementTokens / IncrementRuns: actualiza contador atómico
//   - CheckLimit: lee plan + custom + usage actual; devuelve estado
//   - ResetMonthly: cron job el día 1 del mes — no-op si ya hay row del mes
package billing

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
	ErrPlanNotFound  = errors.New("plan not found")
	ErrOrgNotFound   = errors.New("organization not found")
	ErrQuotaExceeded = errors.New("quota exceeded")
)

type Plan struct {
	ID                uuid.UUID
	Slug              string
	Name              string
	TokensPerMonth    *int64 // NULL = ilimitado
	RunsPerMonth      *int32
	StorageGBMax      *int32
	MembersMax        *int32
	Seats             *int32
	SoftLimitRatio    float64
	MonthlyPriceUSD   float64
	IsActive          bool
}

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

// GetPlan por slug.
func (s *Service) GetPlan(ctx context.Context, slug string) (*Plan, error) {
	var p Plan
	err := s.Pool.QueryRow(ctx,
		`SELECT id, slug, name, tokens_per_month, runs_per_month, storage_gb_max,
		        members_max, seats, soft_limit_ratio, monthly_price_usd, is_active
		 FROM plans WHERE slug = $1`, slug,
	).Scan(&p.ID, &p.Slug, &p.Name, &p.TokensPerMonth, &p.RunsPerMonth, &p.StorageGBMax,
		&p.MembersMax, &p.Seats, &p.SoftLimitRatio, &p.MonthlyPriceUSD, &p.IsActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPlanNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get plan: %w", err)
	}
	return &p, nil
}

// AssignPlan asocia un plan a una org. Crea el plan_started_at = now si era NULL.
//
// ISSUE-21.6 Fase D clean Round 3: tabla organizations se dropea en
// Fase C. En single-org, no hay org-level plan_id (los plans son
// globales vía REQ-35.2 / issue-21.5). AssignPlan es ahora un no-op
// silencioso: el planSlug se valida contra la tabla global de plans
// (que sí existe) pero no se persiste en ninguna org.
func (s *Service) AssignPlan(ctx context.Context, orgID uuid.UUID, planSlug string) error {
	plan, err := s.GetPlan(ctx, planSlug)
	if err != nil {
		return err
	}
	if plan == nil {
		return ErrPlanNotFound
	}
	// No-op: plan_id era per-org pre-single-org. Ahora los plans son
	// globales; "asignar un plan" no tiene sentido en una sola org.
	// Mantenemos la firma para backward compat con callers existentes
	// (tests admin que verifican que la llamada no falla).
	_ = orgID
	_ = plan
	return nil
}

// ResolveLimits combina plan + custom_limits. Si custom tiene un campo,
// gana sobre el del plan.
//
// ISSUE-21.6 Fase D clean Round 3: en single-org no hay plan_id ni
// custom_limits por org. Devolvemos defaults (custom_limits vacío,
// plan_id nil → caller usa plan global o defaults).
func (s *Service) ResolveLimits(ctx context.Context, orgID uuid.UUID) (*Limits, *Plan, error) {
	_ = ctx
	_ = orgID
	// Defaults single-org: sin custom_limits, sin plan específico
	// (los plans globales se consultan via GetPlan cuando el caller los
	// necesita explícitamente).
	return &Limits{}, nil, nil
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
	Dimension     string  // "tokens" | "runs" | "storage" | "members"
	Used          int64
	Limit         int64   // 0 si ilimitado
	Unlimited     bool
	RatioUsed     float64 // 0..1+
	SoftLimitHit  bool    // ratio >= soft_limit_ratio
	HardLimitHit  bool    // ratio >= 1.0
}

// CheckTokens valida si la org puede consumir N tokens más.
// Si HardLimitHit, retorna ErrQuotaExceeded además del state.
func (s *Service) CheckTokens(ctx context.Context, orgID uuid.UUID, additional int64) (*LimitState, error) {
	limits, plan, err := s.ResolveLimits(ctx, orgID)
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
	softRatio := 0.8
	if plan != nil && plan.SoftLimitRatio > 0 {
		softRatio = plan.SoftLimitRatio
	}
	state.SoftLimitHit = state.RatioUsed >= softRatio
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
