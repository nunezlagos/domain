// Package orchestrator — issue-08.10 sdd-pipeline-orchestrator.
//
// El orquestador SDD es el patrón plug-and-play que convierte un prompt
// libre en una secuencia gobernada de fases (sdd-explore → sdd-spec →
// sdd-propose → sdd-design → sdd-tasks → sdd-apply → sdd-verify →
// sdd-judge → sdd-archive → sdd-onboard).
//
// El servidor mantiene state + LLM + memoria + skills; el cliente IDE
// ejecuta las operaciones reales (bash, edit, test, commit). Decisiones
// arquitectónicas: ver docs/rfc/0006-sdd-pipeline-orchestrator.md.
//
// Este archivo declara solamente los tipos del contrato (input + modos
// + ids). Las implementaciones de fases viven en
// internal/service/orchestrator/phases/.
package orchestrator

import (
	"time"

	"github.com/google/uuid"
)

// Mode enumera los cinco modos soportados por el orquestador (RFC 0006).
//
//   - ModeExpress: fast path para cambios ≤10 líneas single-file. Sólo
//     sdd-apply + sdd-verify. Confirm condicional D1.
//   - ModeFull: pipeline completo de 10 fases.
//   - ModeSolo: ejecución inline server-side via LLM provider directo
//     (sin cliente IDE colaborador).
//   - ModeDetect: dry-run; persiste todo a status='draft' sin ejecutar
//     acciones del cliente. Útil para preview.
//   - ModeAsync: emite flow_signals y resume vía worker; el caller
//     desconecta. NO compatible con ModeExpress (D6).
type Mode string

const (
	ModeExpress Mode = "express"
	ModeFull    Mode = "full"
	ModeSolo    Mode = "solo"
	ModeDetect  Mode = "detect"
	ModeAsync   Mode = "async"
)

// IsValid reporta si el string corresponde a un modo soportado.
func (m Mode) IsValid() bool {
	switch m {
	case ModeExpress, ModeFull, ModeSolo, ModeDetect, ModeAsync:
		return true
	}
	return false
}

// PhaseSlug identifica una fase del pipeline. El catálogo v3 declara los
// 11 templates sdd-* (1 orchestrator + 10 phase-workers). El registry
// (internal/service/orchestrator/phases) resuelve slug → handler.
type PhaseSlug string

const (
	PhaseExplore  PhaseSlug = "sdd-explore"
	PhaseSpec     PhaseSlug = "sdd-spec"
	PhasePropose  PhaseSlug = "sdd-propose"
	PhaseDesign   PhaseSlug = "sdd-design"
	PhaseTasks    PhaseSlug = "sdd-tasks"
	PhaseApply    PhaseSlug = "sdd-apply"
	PhaseVerify   PhaseSlug = "sdd-verify"
	PhaseJudge    PhaseSlug = "sdd-judge"
	PhaseArchive  PhaseSlug = "sdd-archive"
	PhaseOnboard  PhaseSlug = "sdd-onboard"
)

// OrchestrateInput es el contrato externo del orquestador. PromptRouter,
// MCP tools y CLI lo construyen y se lo pasan a Service.Run.
type OrchestrateInput struct {
	// OrganizationID + UserID identifican el caller. Obligatorios.
	OrganizationID uuid.UUID
	UserID         uuid.UUID

	// RawText es el prompt libre del usuario (después de PromptRouter
	// classification). El orquestador NO re-clasifica.
	RawText string

	// Mode selecciona el modo de ejecución. Si vacío, el orquestador
	// infiere (default ModeFull). Validación se hace en modes/validator.
	Mode Mode

	// StartingPhase permite reanudar/resumir desde una fase específica
	// (caso resume cross-session). Si vacío, arranca en sdd-explore.
	StartingPhase PhaseSlug

	// SkipPhases lista fases a omitir (ej: ya hechas en sesión previa).
	// El orquestador valida que el grafo resultante sigue siendo válido.
	SkipPhases []PhaseSlug

	// AsyncTimeout aplica sólo en ModeAsync. Si zero, default 30 min.
	AsyncTimeout time.Duration

	// ExpressMaxLines override del default 10. Sólo aplica en ModeExpress
	// (D1). Si zero, se usa el default global.
	ExpressMaxLines int

	// Metadata viaja al flow_run.metadata sin procesamiento (correlación,
	// origen del prompt, etc.).
	Metadata map[string]any
}

// OrchestrateResult es lo que devuelve Service.Run sincrónico. Los modos
// asíncronos devuelven inmediatamente con OrchestratorRunID + FlowRunID
// y status='pending'; el cliente debe pollear o suscribirse a signals.
type OrchestrateResult struct {
	// OrchestratorRunID identifica unívocamente esta invocación del
	// orquestador. Persistido en flow_runs.metadata.orchestrator_run_id.
	OrchestratorRunID uuid.UUID

	// FlowRunID es el flow_run real que ejecuta el DAG sdd-pipeline-v1.
	FlowRunID uuid.UUID

	// Mode resuelto (puede diferir del input si era vacío o si hubo
	// validación que lo cambió — p.ej. detect forzado por dry_run).
	Mode Mode

	// StartedAt es el wall-clock del primer step despachado.
	StartedAt time.Time

	// SnapshotPrompt opcional: cuando el caller pide preview (detect) o
	// modo async, devolvemos el prompt rendered para que IDE lo muestre
	// sin tener que polear inmediatamente.
	SnapshotPrompt string
}
