package orchestrator

// decideMode selects the orchestrator Mode from a ComplexitySignal and
// the input constraints. Called when OrchestrateInput.Mode is empty.
//
//   - ExecMode="manual" → always routes to client IDE
//   - Trivial / Simple → ModeExpress (delegate to client)
//   - Moderate → ModeLite
//   - Complex or MultiConcern → ModeFull
//
// ModeSolo (ejecución server-side via LLM directo) NUNCA se auto-selecciona:
// la arquitectura es client-delegada (el cliente Claude Code/OpenCode ejecuta
// con su LLM y sus subagentes; el server no tiene LLM en prod — VPS). El
// parámetro hasLLM se conserva en la firma por compatibilidad pero ya no
// dispara ModeSolo. ModeSolo solo es alcanzable pasándolo EXPLÍCITO con un LLM
// configurado (uso dev/test); en prod sin LLM falla con ErrLLMFactoryRequired.
func decideMode(sig ComplexitySignal, in OrchestrateInput, hasLLM bool) Mode {
	_ = hasLLM // ya no influye: ModeSolo no se auto-selecciona
	if in.ExecMode == "manual" {
		switch sig.Level {
		case ComplexityTrivial, ComplexitySimple:
			return ModeExpress
		case ComplexityModerate:
			return ModeLite
		default:
			return ModeFull
		}
	}

	switch sig.Level {
	case ComplexityTrivial, ComplexitySimple:
		return ModeExpress
	case ComplexityModerate:
		return ModeLite
	default:
		return ModeFull
	}
}
