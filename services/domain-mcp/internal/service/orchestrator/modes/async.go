package modes

import (
	"context"
	"time"

	"nunezlagos/domain/internal/service/orchestrator/phases"
)

// BuildAsyncPlan construye el plan para el modo Async. Reusa BuildFullPlan
// porque Async ejecuta las 12 fases (igual que Full) pero de forma asíncrona:
// el caller recibe flow_run_id inmediatamente y un worker procesa los steps
// emitiendo flow_signals en cada paso.
//
// La diferencia semántica entre Full y Async está en el dispatch, no en el
// plan. BuildFullPlan produce exactamente el DAG que Async necesita; acá
// sólo etiquetamos el mode para que el service sepa que es async.
func BuildAsyncPlan(ctx context.Context, reg *phases.Registry, in phases.Input,
	startingPhase phases.PhaseSlug, skipPhases []phases.PhaseSlug, now time.Time,
) (*PhasePlan, error) {
	plan, err := BuildFullPlan(ctx, reg, in, startingPhase, skipPhases, now)
	if err != nil {
		return nil, err
	}
	plan.Mode = "async"
	return plan, nil
}
