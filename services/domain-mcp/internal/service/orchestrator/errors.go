package orchestrator

import "errors"

// Errores del orquestador. Conviven con los del runner (issue-08.10):
//   - runner.ErrOrphanRunNotAllowed → enforcement de agent_runs orphan
//   - orchestrator.ErrAsyncModeUnsupported → validación de combinaciones
//   - orchestrator.ErrRequiredSaveMissing → contract suggested_saves
var (



	ErrAsyncModeUnsupported = errors.New("async mode is not compatible with express mode (RFC 0006 D6)")





	ErrRequiredSaveMissing = errors.New("required suggested_save was not persisted by client (RFC 0006 D5)")


	ErrInvalidMode = errors.New("invalid orchestrator mode")


	ErrInvalidExecMode = errors.New("invalid exec_mode (auto|manual|hybrid)")


	ErrEmptyRawText = errors.New("orchestrator requires non-empty raw_text")





	ErrProjectIDRequired = errors.New("orchestrator requires a project_id")



	ErrUnknownPhase = errors.New("unknown phase slug")


	ErrFlowRunNotFound = errors.New("flow_run not found")


	ErrFlowRunStepNotFound = errors.New("flow_run_step not found")




	ErrFlowRunStepNotPending = errors.New("flow_run_step is not in pending/running state")


	ErrLLMFactoryRequired = errors.New("orchestrator: LLM factory required for Solo mode")


	ErrAsyncFlowRunNotFound = errors.New("orchestrator: async flow_run not found")


	ErrAsyncFlowNotAsync = errors.New("orchestrator: flow_run is not in async mode")
)
