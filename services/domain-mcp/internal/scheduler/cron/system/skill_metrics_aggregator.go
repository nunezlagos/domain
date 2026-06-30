// skill_metrics_aggregator.go — system crons (HU-52.2) que agregan
// skill_executions en skill_metrics_daily/weekly.
//
// NO son crons user-defined (tabla `crons`) ni agentes: son jobs internos
// hardcoded, leader-gated, cada uno acotado a UNA operacion. Mismo patron que
// EdgeInferencer/FeedbackAggregator: el caller los invoca dentro de un block
// RunAsLeader.
//
// Dos jobs:
//   - SkillMetricsAggregator (hourly): agrega el dia de HOY para todos los skills
//     activos y, despues, corre el hook de alertas (success_rate < 70% por 3 dias
//     consecutivos -> usage_alert_fires si hay alertas configuradas).
//   - SkillMetricsRollup (daily tick): rollup de la semana actual a weekly +
//     cleanup de retencion (daily 90d, weekly 365d).
//
// DEGRADACION ELEGANTE (regla dura 5): si el Aggregator no esta inyectado, loguea
// y sale limpio sin crashear el scheduler. Si una pasada falla, loguea el error y
// reintenta en el proximo tick.
//
// Opt-in por env (DOMAIN_SKILL_METRICS_ENABLED, default false).
package systemcron

import (
	"context"
	"log/slog"
	"time"

	skillmetrics "nunezlagos/domain/internal/service/skill_metrics"
)

// SkillMetricsAggregator agrega el dia de hoy periodicamente + dispara alertas.
type SkillMetricsAggregator struct {
	Aggregator *skillmetrics.Aggregator

	Tick   time.Duration // default 1h
	Logger *slog.Logger
}

// Start corre el loop hasta ctx cancel. Asume llamado dentro de RunAsLeader.
func (a *SkillMetricsAggregator) Start(ctx context.Context) {
	if a.Tick == 0 {
		a.Tick = time.Hour
	}
	logger := a.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if a.Aggregator == nil {
		logger.Warn("skill-metrics-aggregator disabled: Aggregator no inyectado")
		return
	}

	logger.Info("skill-metrics-aggregator started", slog.Duration("tick", a.Tick))

	a.runTick(ctx, logger)

	ticker := time.NewTicker(a.Tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Info("skill-metrics-aggregator stopping")
			return
		case <-ticker.C:
			a.runTick(ctx, logger)
		}
	}
}

func (a *SkillMetricsAggregator) runTick(ctx context.Context, logger *slog.Logger) {
	today := time.Now().UTC()
	n, err := a.Aggregator.AggregateAll(ctx, today)
	if err != nil {
		logger.Error("skill-metrics-aggregator: agregacion fallo", slog.Any("err", err))
		return
	}
	logger.Info("skill-metrics-aggregator: pasada completa", slog.Int("skills", n))

	detected, fired, err := a.Aggregator.DetectAndFireAlerts(ctx)
	if err != nil {
		// Degradacion: el hook de alertas no debe tumbar la agregacion.
		logger.Error("skill-metrics-aggregator: deteccion de alertas fallo", slog.Any("err", err))
		return
	}
	if detected > 0 {
		logger.Warn("skill-metrics-aggregator: skills con baja tasa de exito",
			slog.Int("detected", detected), slog.Int("alert_fires", fired))
	}
}

// SkillMetricsRollup hace rollup semanal + cleanup de retencion periodicamente.
type SkillMetricsRollup struct {
	Aggregator *skillmetrics.Aggregator

	Tick            time.Duration // default 24h
	DailyRetention  int           // default 90
	WeeklyRetention int           // default 365
	Logger          *slog.Logger
}

// Start corre el loop hasta ctx cancel. Asume llamado dentro de RunAsLeader.
func (r *SkillMetricsRollup) Start(ctx context.Context) {
	if r.Tick == 0 {
		r.Tick = 24 * time.Hour
	}
	if r.DailyRetention <= 0 {
		r.DailyRetention = skillmetrics.DefaultDailyRetentionDays
	}
	if r.WeeklyRetention <= 0 {
		r.WeeklyRetention = skillmetrics.DefaultWeeklyRetentionDays
	}
	logger := r.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if r.Aggregator == nil {
		logger.Warn("skill-metrics-rollup disabled: Aggregator no inyectado")
		return
	}

	logger.Info("skill-metrics-rollup started",
		slog.Duration("tick", r.Tick),
		slog.Int("daily_retention", r.DailyRetention),
		slog.Int("weekly_retention", r.WeeklyRetention))

	r.runTick(ctx, logger)

	ticker := time.NewTicker(r.Tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Info("skill-metrics-rollup stopping")
			return
		case <-ticker.C:
			r.runTick(ctx, logger)
		}
	}
}

func (r *SkillMetricsRollup) runTick(ctx context.Context, logger *slog.Logger) {
	if err := r.Aggregator.RollupWeekly(ctx, time.Now().UTC()); err != nil {
		logger.Error("skill-metrics-rollup: rollup semanal fallo", slog.Any("err", err))
		return
	}
	d, w, err := r.Aggregator.Cleanup(ctx, r.DailyRetention, r.WeeklyRetention)
	if err != nil {
		logger.Error("skill-metrics-rollup: cleanup fallo", slog.Any("err", err))
		return
	}
	logger.Info("skill-metrics-rollup: pasada completa",
		slog.Int64("daily_purged", d), slog.Int64("weekly_purged", w))
}
