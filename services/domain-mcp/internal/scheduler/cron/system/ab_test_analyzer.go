// ab_test_analyzer.go — system cron (HU-52.4) que corre el Analyzer (z-test de
// proporciones, ESTADISTICA PURA, SIN LLM) sobre todos los skill_ab_tests
// 'running' cada N horas (default 6h).
//
// NO es un cron user-defined (tabla `crons`) ni un agente: es un job interno
// hardcoded, leader-gated y acotado a UNA operacion (skill_ab_test.Service.
// AnalyzeRunning). Mismo patron que FeedbackAggregator/SkillJudge: el caller lo
// invoca dentro de un block RunAsLeader.
//
// auto_apply_winner (regla del HU): por DEFAULT FALSE. Por default el cron SOLO
// declara el ganador (y lo deja registrado/notificable), NO toca el pin del
// skill. Si el env DOMAIN_AB_TEST_AUTO_APPLY=true (global) o el test trae
// auto_apply_winner=TRUE (por-test), entonces pinea la version ganadora via
// skills.pinned_version. JAMAS usa organization_id (no existe en skills en
// runtime; bug HU-52.3 a NO repetir).
//
// DEGRADACION ELEGANTE (regla dura 5): si el Service no esta inyectado, loguea y
// sale limpio sin crashear el scheduler. Si una pasada falla, loguea el error y
// reintenta en el proximo tick.
//
// Opt-in por env (DOMAIN_AB_TEST_ENABLED, default false): sin el flag el cron ni
// se registra (ver server_runners.go).
package systemcron

import (
	"context"
	"log/slog"
	"time"

	skillabtest "nunezlagos/domain/internal/service/skill_ab_test"
)

// ABTestAnalyzer corre el z-test sobre los A/B tests running, periodicamente.
//
// Depende SOLO de:
//   - Service: para AnalyzeRunning (lee resultados, corre z-test, declara/aplica).
//
// El Analyzer estadistico se construye aqui con Alpha (default 0.05).
type ABTestAnalyzer struct {
	Service *skillabtest.Service

	Tick      time.Duration // default 6h
	Alpha     float64       // nivel de significancia; <=0 usa 0.05
	AutoApply bool          // global auto-apply del ganador; default false
	Logger    *slog.Logger
}

// Start corre el loop hasta ctx cancel. Asume llamado dentro de RunAsLeader.
func (a *ABTestAnalyzer) Start(ctx context.Context) {
	if a.Tick == 0 {
		a.Tick = 6 * time.Hour
	}
	logger := a.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if a.Service == nil {
		logger.Warn("ab-test-analyzer disabled: Service no inyectado")
		return
	}

	logger.Info("ab-test-analyzer started",
		slog.Duration("tick", a.Tick),
		slog.Float64("alpha", a.effectiveAlpha()),
		slog.Bool("auto_apply", a.AutoApply))

	a.runTick(ctx, logger)

	ticker := time.NewTicker(a.Tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Info("ab-test-analyzer stopping")
			return
		case <-ticker.C:
			a.runTick(ctx, logger)
		}
	}
}

func (a *ABTestAnalyzer) effectiveAlpha() float64 {
	if a.Alpha <= 0 {
		return skillabtest.DefaultAlpha
	}
	return a.Alpha
}

func (a *ABTestAnalyzer) runTick(ctx context.Context, logger *slog.Logger) {
	analyzer := skillabtest.NewAnalyzer(a.effectiveAlpha())
	results, err := a.Service.AnalyzeRunning(ctx, analyzer, a.AutoApply)
	if err != nil {
		// Degradacion: loguear y seguir; el proximo tick reintenta.
		logger.Error("ab-test-analyzer: pasada fallo", slog.Any("err", err))
		return
	}
	declared, pinned := 0, 0
	for _, r := range results {
		if r.Declared {
			declared++
			logger.Info("ab-test-analyzer: ganador declarado",
				slog.String("test_id", r.TestID.String()),
				slog.String("skill_slug", r.SkillSlug),
				slog.String("winner", r.Verdict.Winner),
				slog.Float64("confidence", r.Verdict.Confidence),
				slog.Bool("pin_applied", r.PinApplied))
		}
		if r.PinApplied {
			pinned++
		}
	}
	logger.Info("ab-test-analyzer: pasada completa",
		slog.Int("running", len(results)),
		slog.Int("declared", declared),
		slog.Int("pinned", pinned))
}
