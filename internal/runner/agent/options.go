package agentrunner

import "github.com/google/uuid"

// RunOption configura una invocación de Runner.Run sin romper la signature
// histórica. Las opciones declarativas reemplazan flags ad-hoc en RunInput
// porque pertenecen al cómo del run, no al qué.
//
// issue-08.10: WithFlowRun + WithStandalone permiten al orquestador SDD
// (internal/service/orchestrator) atar el run a un flow_run paso explícito,
// mientras que invocaciones directas (CLI, MCP tool legacy, tests) declaran
// WithStandalone(true) y aceptan la marca metadata.standalone='true'.
type RunOption func(*runOpts)

type runOpts struct {
	// flowRunID, si presente, persiste agent_runs.flow_run_id. La auditoría
	// (issue-08.12) considera no-orphan cualquier run con flow_run_id NOT NULL.
	flowRunID *uuid.UUID
	// flowRunStepID puntea al step específico del DAG cuando el orquestador
	// despacha por fase. Persistido en agent_runs.flow_run_step_id si la
	// columna existe; ignorado si nil.
	flowRunStepID *uuid.UUID
	// standalone marca el run como direct_invocation legítimo. Cuando es true
	// (default histórico) el runner persiste metadata.standalone='true' para
	// que el cron orphan-runs-audit no lo cuente como bypass.
	//
	// El default es true para preservar compatibilidad: callers viejos
	// (skip variadic) siguen siendo standalone. El orquestador SDD debe
	// pasar WithStandalone(false) + WithFlowRun(id) para marcar el run como
	// gobernado.
	standalone bool
}

// WithFlowRun ata el agent_run al flow_run del orquestador. Implica
// standalone=false salvo que se combine con WithStandalone(true) (no se
// recomienda; el cron orphan considera no-orphan al tener flow_run_id de
// todos modos, pero la combinación es semánticamente contradictoria).
func WithFlowRun(id uuid.UUID) RunOption {
	return func(o *runOpts) {
		o.flowRunID = &id
		o.standalone = false
	}
}

// WithFlowRunStep ata el run al step específico del DAG (issue-09).
func WithFlowRunStep(id uuid.UUID) RunOption {
	return func(o *runOpts) {
		o.flowRunStepID = &id
	}
}

// WithStandalone fija el flag explícito. Útil para tests que quieran simular
// el path orquestado (false) sin tener un flow_run real.
func WithStandalone(v bool) RunOption {
	return func(o *runOpts) { o.standalone = v }
}

// defaultRunOpts devuelve la configuración legacy: standalone=true, sin
// flow_run_id. Mantiene compatibilidad para todos los callers actuales.
func defaultRunOpts() runOpts {
	return runOpts{standalone: true}
}

// resolveRunOpts aplica el slice variadic sobre el default.
func resolveRunOpts(opts []RunOption) runOpts {
	r := defaultRunOpts()
	for _, o := range opts {
		o(&r)
	}
	return r
}

// buildRunMetadata serializa el shape canónico del metadata JSONB de
// agent_runs. Si standalone=true, marca con reason para distinguir
// direct_invocation vs direct_invocation_failed (path failedRun).
//
// Cuando standalone=false y flow_run_id está presente, devuelve `{}` —
// el run es gobernado por el orquestador y no necesita marca.
func buildRunMetadata(o runOpts, reason string) []byte {
	if o.standalone {
		if reason == "" {
			reason = "direct_invocation"
		}
		return []byte(`{"standalone":true,"reason":"` + reason + `"}`)
	}
	return []byte(`{}`)
}
