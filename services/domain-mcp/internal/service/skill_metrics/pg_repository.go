package skill_metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/service/skill_metrics/skillmetricsdb"
	"nunezlagos/domain/internal/store/txctx"
)

type pgRepository struct {
	pool *pgxpool.Pool
}

// NewPgRepository crea el Repository concreto sobre pgx.
func NewPgRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

// q resuelve el Queries contra la tx-context (RLS/tx) si existe, o el pool.
func (r *pgRepository) q(ctx context.Context) *skillmetricsdb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return skillmetricsdb.New(tx)
	}
	return skillmetricsdb.New(r.pool)
}

func (r *pgRepository) ListActiveSkillIDs(ctx context.Context) ([]uuid.UUID, error) {
	ids, err := r.q(ctx).ListActiveSkillIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active skill ids: %w", err)
	}
	return ids, nil
}

func (r *pgRepository) AggregateDay(ctx context.Context, skillID uuid.UUID, day time.Time) (AggregateResult, error) {
	row, err := r.q(ctx).AggregateDay(ctx, skillmetricsdb.AggregateDayParams{
		SkillID: skillID,
		Day:     pgtype.Date{Time: day, Valid: true},
	})
	if err != nil {
		return AggregateResult{}, fmt.Errorf("aggregate day: %w", err)
	}
	return AggregateResult{
		InvocationsCount:   int(row.InvocationsCount),
		SuccessCount:       int(row.SuccessCount),
		FailureCount:       int(row.FailureCount),
		SuccessRate:        numericToFloatPtr(row.SuccessRate),
		AvgDurationMs:      numericToIntPtr(row.AvgDurationMs),
		P95DurationMs:      numericToIntPtr(row.P95DurationMs),
		UniqueCallersCount: int(row.UniqueCallersCount),
	}, nil
}

func (r *pgRepository) PersistDaily(ctx context.Context, skillID uuid.UUID, day time.Time, res AggregateResult) error {
	err := r.q(ctx).UpsertDaily(ctx, skillmetricsdb.UpsertDailyParams{
		SkillID:            skillID,
		Day:                pgtype.Date{Time: day, Valid: true},
		InvocationsCount:   int32(res.InvocationsCount),
		SuccessCount:       int32(res.SuccessCount),
		FailureCount:       int32(res.FailureCount),
		SuccessRate:        floatPtrToNumeric(res.SuccessRate),
		AvgDurationMs:      intPtrToInt32Ptr(res.AvgDurationMs),
		P95DurationMs:      intPtrToInt32Ptr(res.P95DurationMs),
		UniqueCallersCount: int32(res.UniqueCallersCount),
	})
	if err != nil {
		return fmt.Errorf("persist daily: %w", err)
	}
	return nil
}

func (r *pgRepository) GetDailyBySkill(ctx context.Context, skillID uuid.UUID, days int) ([]DailyMetric, error) {
	rows, err := r.q(ctx).GetDailyBySkill(ctx, skillmetricsdb.GetDailyBySkillParams{
		SkillID: skillID,
		Days:    int32(days),
	})
	if err != nil {
		return nil, fmt.Errorf("get daily by skill: %w", err)
	}
	out := make([]DailyMetric, 0, len(rows))
	for _, row := range rows {
		m := DailyMetric{
			SkillID:            row.SkillID,
			InvocationsCount:   int(row.InvocationsCount),
			SuccessCount:       int(row.SuccessCount),
			FailureCount:       int(row.FailureCount),
			SuccessRate:        nonZeroRatePtr(row.SuccessRate),
			AvgDurationMs:      int32PtrToIntPtr(row.AvgDurationMs),
			P95DurationMs:      int32PtrToIntPtr(row.P95DurationMs),
			UniqueCallersCount: int(row.UniqueCallersCount),
		}
		if row.Day.Valid {
			m.Day = row.Day.Time
		}
		ca, ua := row.CreatedAt, row.UpdatedAt
		m.CreatedAt, m.UpdatedAt = &ca, &ua
		out = append(out, m)
	}
	return out, nil
}

func (r *pgRepository) GetDailyByDay(ctx context.Context, day time.Time) ([]DailyMetric, error) {
	rows, err := r.q(ctx).GetDailyByDay(ctx, pgtype.Date{Time: day, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("get daily by day: %w", err)
	}
	out := make([]DailyMetric, 0, len(rows))
	for _, row := range rows {
		m := DailyMetric{
			SkillID:            row.SkillID,
			InvocationsCount:   int(row.InvocationsCount),
			SuccessCount:       int(row.SuccessCount),
			FailureCount:       int(row.FailureCount),
			SuccessRate:        nonZeroRatePtr(row.SuccessRate),
			AvgDurationMs:      int32PtrToIntPtr(row.AvgDurationMs),
			P95DurationMs:      int32PtrToIntPtr(row.P95DurationMs),
			UniqueCallersCount: int(row.UniqueCallersCount),
		}
		if row.Day.Valid {
			m.Day = row.Day.Time
		}
		ca, ua := row.CreatedAt, row.UpdatedAt
		m.CreatedAt, m.UpdatedAt = &ca, &ua
		out = append(out, m)
	}
	return out, nil
}

func (r *pgRepository) ListTopFailed(ctx context.Context, days, limit int) ([]TopFailed, error) {
	rows, err := r.q(ctx).ListTopFailed(ctx, skillmetricsdb.ListTopFailedParams{
		Days:        int32(days),
		ResultLimit: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list top failed: %w", err)
	}
	out := make([]TopFailed, 0, len(rows))
	for _, row := range rows {
		sr := row.SuccessRate
		out = append(out, TopFailed{
			SkillID:          row.SkillID,
			InvocationsCount: row.InvocationsCount,
			SuccessCount:     row.SuccessCount,
			FailureCount:     row.FailureCount,
			SuccessRate:      &sr,
		})
	}
	return out, nil
}

func (r *pgRepository) ListSlowest(ctx context.Context, days, limit int) ([]Slowest, error) {
	rows, err := r.q(ctx).ListSlowest(ctx, skillmetricsdb.ListSlowestParams{
		Days:        int32(days),
		ResultLimit: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list slowest: %w", err)
	}
	out := make([]Slowest, 0, len(rows))
	for _, row := range rows {
		out = append(out, Slowest{
			SkillID:          row.SkillID,
			P95DurationMs:    int(row.P95DurationMs),
			InvocationsCount: row.InvocationsCount,
		})
	}
	return out, nil
}

func (r *pgRepository) RollupWeek(ctx context.Context, weekStart time.Time) error {
	if err := r.q(ctx).RollupWeek(ctx, pgtype.Date{Time: weekStart, Valid: true}); err != nil {
		return fmt.Errorf("rollup week: %w", err)
	}
	return nil
}

func (r *pgRepository) CleanupDaily(ctx context.Context, retentionDays int) (int64, error) {
	n, err := r.q(ctx).CleanupDaily(ctx, int32(retentionDays))
	if err != nil {
		return 0, fmt.Errorf("cleanup daily: %w", err)
	}
	return n, nil
}

func (r *pgRepository) CleanupWeekly(ctx context.Context, retentionDays int) (int64, error) {
	n, err := r.q(ctx).CleanupWeekly(ctx, int32(retentionDays))
	if err != nil {
		return 0, fmt.Errorf("cleanup weekly: %w", err)
	}
	return n, nil
}

func (r *pgRepository) ListLowRateStreaks(ctx context.Context, threshold float64) ([]LowRateStreak, error) {
	rows, err := r.q(ctx).ListLowRateStreaks(ctx, floatToNumeric(threshold))
	if err != nil {
		return nil, fmt.Errorf("list low rate streaks: %w", err)
	}
	out := make([]LowRateStreak, 0, len(rows))
	for _, row := range rows {
		s := LowRateStreak{
			SkillID:        row.SkillID,
			AvgSuccessRate: row.AvgSuccessRate,
		}
		if row.StreakStart.Valid {
			s.StreakStart = row.StreakStart.Time
		}
		if row.StreakEnd.Valid {
			s.StreakEnd = row.StreakEnd.Time
		}
		out = append(out, s)
	}
	return out, nil
}

// FireAlerts registra un fire por cada usage_alert activa con metric=AlertMetricKey,
// respetando el cooldown configurado. Si NO hay alertas configuradas para ese
// metric, devuelve 0 sin error: la deteccion sigue corriendo y el operador puede
// crear la alerta despues (hook documentado, no inventa la tabla).
func (r *pgRepository) FireAlerts(ctx context.Context, streak LowRateStreak) (int, error) {
	q := r.q(ctx)
	alerts, err := q.ListActiveAlertsByMetric(ctx, AlertMetricKey)
	if err != nil {
		return 0, fmt.Errorf("list active alerts: %w", err)
	}
	if len(alerts) == 0 {
		return 0, nil
	}
	payload, err := json.Marshal(map[string]any{
		"skill_id":     streak.SkillID.String(),
		"streak_start": streak.StreakStart.Format("2006-01-02"),
		"streak_end":   streak.StreakEnd.Format("2006-01-02"),
		"streak_days":  LowRateStreakDays,
	})
	if err != nil {
		return 0, fmt.Errorf("marshal alert payload: %w", err)
	}
	fired := 0
	now := time.Now()
	for _, a := range alerts {
		// Respeta cooldown: si se disparo hace menos de cooldown_secs, skip.
		if a.LastFiredAt.Valid && a.CooldownSecs > 0 {
			elapsed := now.Sub(a.LastFiredAt.Time)
			if elapsed < time.Duration(a.CooldownSecs)*time.Second {
				continue
			}
		}
		if err := q.InsertAlertFire(ctx, skillmetricsdb.InsertAlertFireParams{
			AlertID:       a.ID,
			Metric:        AlertMetricKey,
			Threshold:     a.Threshold,
			ObservedValue: streak.AvgSuccessRate,
			Payload:       payload,
		}); err != nil {
			return fired, fmt.Errorf("insert alert fire: %w", err)
		}
		if err := q.TouchAlertFired(ctx, a.ID); err != nil {
			return fired, fmt.Errorf("touch alert fired: %w", err)
		}
		fired++
	}
	return fired, nil
}

// ---- conversiones pgtype <-> Go ----

// numericToFloatPtr: pgtype.Numeric nullable -> *float64; nil si NULL/invalido.
func numericToFloatPtr(n pgtype.Numeric) *float64 {
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return nil
	}
	v := f.Float64
	return &v
}

// numericToIntPtr: pgtype.Numeric nullable -> *int (redondeado); nil si NULL.
func numericToIntPtr(n pgtype.Numeric) *int {
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return nil
	}
	v := int(math.Round(f.Float64))
	return &v
}

// nonZeroRatePtr: success_rate llega como float8 NOT NULL en las queries de
// lectura (cast ::float8). pgx entrega 0 para NULL; aqui lo devolvemos tal cual
// como puntero para preservar el 0%/NULL aguas arriba.
func nonZeroRatePtr(v float64) *float64 {
	out := v
	return &out
}

// floatPtrToNumeric: *float64 -> pgtype.Numeric (Valid=false si nil = NULL).
func floatPtrToNumeric(p *float64) pgtype.Numeric {
	if p == nil {
		return pgtype.Numeric{Valid: false}
	}
	return floatToNumeric(*p)
}

// floatToNumeric construye un pgtype.Numeric desde float64.
func floatToNumeric(v float64) pgtype.Numeric {
	var n pgtype.Numeric
	// ScanScientific acepta notacion decimal; robusto para 2 decimales.
	_ = n.Scan(fmt.Sprintf("%.6f", v))
	return n
}

// intPtrToInt32Ptr: *int -> *int32 (nil preserva NULL).
func intPtrToInt32Ptr(p *int) *int32 {
	if p == nil {
		return nil
	}
	v := int32(*p)
	return &v
}

// int32PtrToIntPtr: *int32 -> *int (nil preserva NULL).
func int32PtrToIntPtr(p *int32) *int {
	if p == nil {
		return nil
	}
	v := int(*p)
	return &v
}
