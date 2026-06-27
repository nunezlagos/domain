package skill_metrics

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Repository contrato de persistencia/lectura de skill_metrics.
//
// AggregateDay computa el agregado de UN dia desde skill_executions (no toca
// skill_metrics_daily). PersistDaily lo escribe (upsert por PK). Esa separacion
// permite testear el computo sin DB y reusar el resultado en el hook de alertas.
type Repository interface {
	// ListActiveSkillIDs devuelve los skills vivos (deleted_at IS NULL).
	ListActiveSkillIDs(ctx context.Context) ([]uuid.UUID, error)

	// AggregateDay computa (no persiste) el agregado de un skill en un dia UTC.
	AggregateDay(ctx context.Context, skillID uuid.UUID, day time.Time) (AggregateResult, error)
	// PersistDaily hace upsert del agregado diario por (skill_id, day).
	PersistDaily(ctx context.Context, skillID uuid.UUID, day time.Time, r AggregateResult) error

	// GetDailyBySkill series diarias de un skill (ultimos N dias).
	GetDailyBySkill(ctx context.Context, skillID uuid.UUID, days int) ([]DailyMetric, error)
	// GetDailyByDay metricas de un dia (todos los skills).
	GetDailyByDay(ctx context.Context, day time.Time) ([]DailyMetric, error)
	// ListTopFailed peores success_rate en N dias (limit filas).
	ListTopFailed(ctx context.Context, days, limit int) ([]TopFailed, error)
	// ListSlowest peores p95 en N dias (limit filas).
	ListSlowest(ctx context.Context, days, limit int) ([]Slowest, error)

	// RollupWeek consolida la semana de weekStart en skill_metrics_weekly.
	RollupWeek(ctx context.Context, weekStart time.Time) error
	// CleanupDaily borra daily mas viejas que retentionDays; devuelve filas.
	CleanupDaily(ctx context.Context, retentionDays int) (int64, error)
	// CleanupWeekly borra weekly mas viejas que retentionDays; devuelve filas.
	CleanupWeekly(ctx context.Context, retentionDays int) (int64, error)

	// ListLowRateStreaks skills con 3 dias consecutivos < threshold.
	ListLowRateStreaks(ctx context.Context, threshold float64) ([]LowRateStreak, error)
	// FireAlerts registra un fire por cada usage_alert activa que matchee
	// AlertMetricKey, respetando cooldown. Devuelve cuantos fires registro.
	// Si no hay alertas configuradas, devuelve 0 sin error (hook documentado).
	FireAlerts(ctx context.Context, streak LowRateStreak) (int, error)
}
