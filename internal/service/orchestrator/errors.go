package orchestrator

import "errors"

// Errores del orquestador. Conviven con los del runner (issue-08.10):
//   - runner.ErrOrphanRunNotAllowed → enforcement de agent_runs orphan
//   - orchestrator.ErrAsyncModeUnsupported → validación de combinaciones
//   - orchestrator.ErrRequiredSaveMissing → contract suggested_saves
var (
	// ErrAsyncModeUnsupported D6: ModeAsync + ModeExpress no son compatibles.
	// El runtime express no emite flow_signals para reanudar; tiene que ser
	// sincrónico. La combinación es rechazada en validator.
	ErrAsyncModeUnsupported = errors.New("async mode is not compatible with express mode (RFC 0006 D6)")

	// ErrRequiredSaveMissing D5: una fase declaró required=true para algún
	// suggested_save y el cliente no reportó la memory_ref en su respuesta.
	// La fase no avanza; el orquestador devuelve este error al cliente con
	// el detalle de qué save falta para que pueda re-emitir.
	ErrRequiredSaveMissing = errors.New("required suggested_save was not persisted by client (RFC 0006 D5)")

	// ErrInvalidMode el Mode no está en {express, full, solo, detect, async}.
	ErrInvalidMode = errors.New("invalid orchestrator mode")

	// ErrEmptyRawText el caller no proveyó prompt. El orquestador no inventa.
	ErrEmptyRawText = errors.New("orchestrator requires non-empty raw_text")

	// ErrUnknownPhase StartingPhase o SkipPhases referencia una fase no
	// registrada en phases.Registry.
	ErrUnknownPhase = errors.New("unknown phase slug")

	// ErrFlowRunNotFound lookup de un flow_run_id que no existe.
	ErrFlowRunNotFound = errors.New("flow_run not found")

	// ErrFlowRunStepNotFound lookup de step_id que no existe.
	ErrFlowRunStepNotFound = errors.New("flow_run_step not found")

	// ErrFlowRunStepNotPending el step ya está en estado terminal
	// (completed/failed/skipped/cancelled) — no se puede re-marcar.
	// El cliente debe verificar el estado antes de reportar phase_result.
	ErrFlowRunStepNotPending = errors.New("flow_run_step is not in pending/running state")

	// ErrLLMFactoryRequired Mode=Solo necesita un LLM factory inyectado.
	ErrLLMFactoryRequired = errors.New("orchestrator: LLM factory required for Solo mode")
)
