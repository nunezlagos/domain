package skill_metrics

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Aggregator computa y persiste los agregados de skill_metrics. Separado del
// Service (lectura) porque lo usan los crons de sistema, no los handlers.
type Aggregator struct {
	repo Repository
}

// NewAggregator inyecta el Repository.
func NewAggregator(repo Repository) *Aggregator {
	return &Aggregator{repo: repo}
}

// Aggregate computa el agregado de UN skill en UN dia (desde skill_executions)
// y lo persiste (upsert por PK). Idempotente: la misma pasada N veces deja el
// mismo resultado.
//
// La definicion EXACTA de exito/fallo, el guard de p95 (<10 invocaciones) y la
// division por cero (success_rate NULL si no hubo invocaciones contables) viven
// en la query AggregateDay (sql/query.sql). Aqui solo orquestamos.
func (a *Aggregator) Aggregate(ctx context.Context, skillID uuid.UUID, day time.Time) (AggregateResult, error) {
	res, err := a.repo.AggregateDay(ctx, skillID, day)
	if err != nil {
		return AggregateResult{}, err
	}
	if err := a.repo.PersistDaily(ctx, skillID, day, res); err != nil {
		return AggregateResult{}, err
	}
	return res, nil
}

// AggregateAll agrega el dia `day` para TODOS los skills activos. Devuelve
// cuantos skills se procesaron. Un error en un skill aborta la pasada (el cron
// reintenta en el proximo tick).
func (a *Aggregator) AggregateAll(ctx context.Context, day time.Time) (int, error) {
	ids, err := a.repo.ListActiveSkillIDs(ctx)
	if err != nil {
		return 0, err
	}
	processed := 0
	for _, id := range ids {
		if _, err := a.Aggregate(ctx, id, day); err != nil {
			return processed, fmt.Errorf("aggregate skill %s: %w", id, err)
		}
		processed++
	}
	return processed, nil
}

// RollupWeekly consolida la semana que contiene `ref` en skill_metrics_weekly.
// weekStart = lunes (ISO) de esa semana.
func (a *Aggregator) RollupWeekly(ctx context.Context, ref time.Time) error {
	return a.repo.RollupWeek(ctx, WeekStart(ref))
}

// Cleanup borra daily mas viejas que dailyRetention y weekly mas viejas que
// weeklyRetention. Devuelve (dailyBorradas, weeklyBorradas).
func (a *Aggregator) Cleanup(ctx context.Context, dailyRetention, weeklyRetention int) (int64, int64, error) {
	d, err := a.repo.CleanupDaily(ctx, dailyRetention)
	if err != nil {
		return 0, 0, err
	}
	w, err := a.repo.CleanupWeekly(ctx, weeklyRetention)
	if err != nil {
		return d, 0, err
	}
	return d, w, nil
}

// DetectAndFireAlerts busca skills con LowRateStreakDays dias consecutivos bajo
// el umbral y, por cada uno, dispara las usage_alerts activas que matcheen
// AlertMetricKey. Devuelve (skillsDetectados, firesEmitidos).
//
// Hook documentado (regla 5 de la spec): si NO hay usage_alerts configuradas
// para AlertMetricKey, la deteccion corre igual pero no se emite ningun fire
// (FireAlerts devuelve 0). No inventa ninguna tabla ni rompe nada.
func (a *Aggregator) DetectAndFireAlerts(ctx context.Context) (int, int, error) {
	streaks, err := a.repo.ListLowRateStreaks(ctx, SuccessRateThreshold)
	if err != nil {
		return 0, 0, err
	}
	totalFires := 0
	for _, s := range streaks {
		n, err := a.repo.FireAlerts(ctx, s)
		if err != nil {
			return len(streaks), totalFires, err
		}
		totalFires += n
	}
	return len(streaks), totalFires, nil
}

// WeekStart devuelve el lunes (00:00 UTC) de la semana ISO que contiene t.
func WeekStart(t time.Time) time.Time {
	t = t.UTC()
	// time.Weekday: Sunday=0..Saturday=6. ISO week empieza lunes.
	offset := (int(t.Weekday()) + 6) % 7
	d := t.AddDate(0, 0, -offset)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
}
