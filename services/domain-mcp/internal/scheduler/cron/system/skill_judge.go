// skill_judge.go — system cron (HU-52.3) que corre el LLM-as-judge semanalmente.
//
// NO es un cron user-defined (tabla `crons`) ni un agente: es un job interno
// hardcoded, leader-gated y acotado a UNA operacion (skill_suggestions.
// Aggregator.Run). Mismo patron que FeedbackAggregator/SkillMetricsAggregator:
// el caller lo invoca dentro de un block RunAsLeader.
//
// HUMAN-IN-THE-LOOP (regla dura 6): el cron SOLO genera sugerencias 'pending'
// (Aggregator.Run -> Service.Create). NADA se auto-aplica. El Apply (que muta
// `skills`) corre exclusivamente por accion humana (approve+apply desde UI/CLI).
//
// DEGRADACION (regla dura 7): si no hay LLM (sin MINIMAX_API_KEY) Run devuelve
// ErrJudgeUnavailable; el cron lo loguea limpio (Info, no Error) y reintenta en
// la proxima ventana, sin crashear.
//
// PROGRAMACION: por defecto corre los LUNES a las 03:00 (hora local del proceso),
// re-programandose cada semana. Si Weekday/Hour no se configuran, usa esos
// defaults. Una corrida manual inicial NO se dispara al arrancar (evita ruido en
// cada reinicio del leader); el primer run ocurre en la proxima ventana semanal.
//
// Opt-in por env (DOMAIN_SKILL_JUDGE_ENABLED, default false): sin el flag el cron
// ni se registra (ver server_runners.go).
package systemcron

import (
	"context"
	"errors"
	"log/slog"
	"time"

	skillsuggestions "nunezlagos/domain/internal/service/skill_suggestions"
)

// SkillJudge corre el aggregator del judge en una ventana semanal.
//
// Depende SOLO de:
//   - Aggregator: para Run (escanea skills, evalua via LLM, persiste pending).
//
// No recibe ninguna otra capability: el scope es leer skills/metricas/feedback y
// escribir skill_suggestions (pending), nada mas. JAMAS aplica nada.
type SkillJudge struct {
	Aggregator *skillsuggestions.Aggregator

	// Weekday/Hour definen la ventana semanal (hora local del proceso).
	// Defaults: lunes 03:00.
	Weekday time.Weekday
	Hour    int

	Logger *slog.Logger
}

// Start corre el loop hasta ctx cancel. Asume llamado dentro de RunAsLeader.
func (j *SkillJudge) Start(ctx context.Context) {
	logger := j.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if j.Aggregator == nil {
		logger.Warn("skill-judge disabled: Aggregator no inyectado")
		return
	}
	// Default: lunes 03:00. Weekday cero-value es Sunday; usamos Monday salvo
	// que se configure explicitamente otra cosa via Hour>0 o Weekday!=Sunday.
	if j.Weekday == time.Sunday && j.Hour == 0 {
		j.Weekday = time.Monday
		j.Hour = 3
	}

	logger.Info("skill-judge started",
		slog.String("weekday", j.Weekday.String()),
		slog.Int("hour", j.Hour))

	for {
		next := nextWeekly(time.Now(), j.Weekday, j.Hour)
		wait := time.Until(next)
		logger.Info("skill-judge: proxima corrida",
			slog.Time("at", next), slog.Duration("in", wait))

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			logger.Info("skill-judge stopping")
			return
		case <-timer.C:
			j.runOnce(ctx, logger)
		}
	}
}

// runOnce corre una pasada. Degrada limpio sin LLM (regla dura 7) y nunca
// crashea: cualquier error se loguea y el loop continua a la proxima ventana.
func (j *SkillJudge) runOnce(ctx context.Context, logger *slog.Logger) {
	res, err := j.Aggregator.Run(ctx)
	if errors.Is(err, skillsuggestions.ErrJudgeUnavailable) {
		// Sin LLM: log limpio (Info), no crash, no Error.
		logger.Info("skill-judge: omitido (LLM no disponible, requiere MINIMAX_API_KEY)")
		return
	}
	if err != nil {
		logger.Error("skill-judge: corrida fallo", slog.Any("err", err))
		return
	}
	logger.Info("skill-judge: corrida completa",
		slog.Int("skills_scanned", res.SkillsScanned),
		slog.Int("suggestions", res.Suggestions),
		slog.Int("persisted", res.Persisted),
		slog.Int("deduped", res.Deduped),
		slog.Int("skills_with_error", res.SkillsWithError))
}

// nextWeekly devuelve el proximo instante en weekday a la hora `hour` (minuto 0),
// estrictamente despues de `from` (hora local). Si hoy es el weekday pero ya paso
// la hora, salta a la semana siguiente.
func nextWeekly(from time.Time, weekday time.Weekday, hour int) time.Time {
	loc := from.Location()
	// Candidato de hoy a la hora pedida.
	candidate := time.Date(from.Year(), from.Month(), from.Day(), hour, 0, 0, 0, loc)
	// Dias hasta el weekday objetivo (0..6).
	delta := (int(weekday) - int(from.Weekday()) + 7) % 7
	candidate = candidate.AddDate(0, 0, delta)
	if !candidate.After(from) {
		candidate = candidate.AddDate(0, 0, 7)
	}
	return candidate
}
