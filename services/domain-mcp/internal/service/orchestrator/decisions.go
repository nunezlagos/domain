package orchestrator

// decideMode selects the orchestrator Mode from a ComplexitySignal and
// the input constraints. Called when OrchestrateInput.Mode is empty.
//
//   - ExecMode="manual" → never ModeSolo; always routes to client IDE
//   - Trivial + LLM available → ModeSolo (orchestrator handles it server-side)
//   - Trivial + no LLM → ModeExpress (minimal phases, delegate to client)
//   - Simple → ModeExpress
//   - Moderate → ModeLite
//   - Complex or MultiConcern → ModeFull
func decideMode(sig ComplexitySignal, in OrchestrateInput, hasLLM bool) Mode {
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
	case ComplexityTrivial:
		if hasLLM {
			return ModeSolo
		}
		return ModeExpress
	case ComplexitySimple:
		return ModeExpress
	case ComplexityModerate:
		return ModeLite
	default:
		return ModeFull
	}
}
