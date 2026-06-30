// feedback_aggregator.go — system cron (HU-52.1) que consolida skill_feedback
// en agregados diarios (skill_feedback_daily).
//
// NO es un cron user-defined (tabla `crons`) ni un agente: es un job interno
// hardcoded, leader-gated y acotado a UNA operacion (feedback.Service.
// ConsolidateDaily). Mismo patron que EdgeInferencer/HeartbeatWatcher: el caller
// lo invoca dentro de un block RunAsLeader.
//
// SELF-CONTAINED (nota de dependencia del HU): la spec original lo hacia escribir
// a skill_metrics, pero skill_metrics se crea en HU-52.2 (aun NO implementada).
// Para no acoplar 52.1 a 52.2, este aggregator computa los agregados desde
// skill_feedback y los persiste en su PROPIA tabla (skill_feedback_daily, mig
// 000180). En HU-52.2 se integrara con skill_metrics.
//
// DEGRADACION ELEGANTE (regla dura 5): si el Service no esta inyectado, loguea y
// sale limpio sin crashear el scheduler. Si una pasada falla, se loguea el error
// y se reintenta en el proximo tick.
//
// Opt-in por env (DOMAIN_FEEDBACK_AGGREGATOR_ENABLED, default false).
package systemcron

import (
	"context"
	"log/slog"
	"time"

	feedbacksvc "nunezlagos/domain/internal/service/feedback"
)

// FeedbackAggregator consolida feedback en agregados diarios, periodicamente.
//
// Depende SOLO de:
//   - Feedback: para ConsolidateDaily (computa y persiste agregados).
//
// No recibe ninguna otra capability: el scope es leer skill_feedback y escribir
// skill_feedback_daily, nada mas.
type FeedbackAggregator struct {
	Feedback *feedbacksvc.Service

	Tick   time.Duration // default 6h
	Days   int           // ventana a consolidar por pasada; default 7
	Logger *slog.Logger
}

// Start corre el loop hasta ctx cancel. Asume llamado dentro de RunAsLeader.
func (a *FeedbackAggregator) Start(ctx context.Context) {
	if a.Tick == 0 {
		a.Tick = 6 * time.Hour
	}
	if a.Days <= 0 {
		a.Days = 7
	}
	logger := a.Logger
	if logger == nil {
		logger = slog.Default()
	}

	if a.Feedback == nil {
		logger.Warn("feedback-aggregator disabled: Service no inyectado")
		return
	}

	logger.Info("feedback-aggregator started",
		slog.Duration("tick", a.Tick),
		slog.Int("days", a.Days))

	a.runTick(ctx, logger)

	ticker := time.NewTicker(a.Tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Info("feedback-aggregator stopping")
			return
		case <-ticker.C:
			a.runTick(ctx, logger)
		}
	}
}

func (a *FeedbackAggregator) runTick(ctx context.Context, logger *slog.Logger) {
	n, err := a.Feedback.ConsolidateDaily(ctx, a.Days)
	if err != nil {
		// Degradacion: loguear y seguir; el proximo tick reintenta.
		logger.Error("feedback-aggregator: consolidacion fallo", slog.Any("err", err))
		return
	}
	logger.Info("feedback-aggregator: pasada completa", slog.Int("rows", n))
}
