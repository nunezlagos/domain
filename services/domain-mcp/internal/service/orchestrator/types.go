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
//   - ModeLite: camino reducido para cambios triviales (fix de 1 línea,
//     doc, refactor chico). Corre un SUBSET de fases (default
//     sdd-explore → sdd-apply → sdd-verify) salteando las pesadas
//     (propose/design/tasks/judge/archive/onboard). Más amplio que
//     Express (incluye explore para ubicar el cambio) pero mucho más
//     barato que Full. Opt-in: nunca es el default.
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
	ModeLite    Mode = "lite"
	ModeFull    Mode = "full"
	ModeSolo    Mode = "solo"
	ModeDetect  Mode = "detect"
	ModeAsync   Mode = "async"
)

// IsValid reporta si el string corresponde a un modo soportado.
func (m Mode) IsValid() bool {
	switch m {
	case ModeExpress, ModeLite, ModeFull, ModeSolo, ModeDetect, ModeAsync:
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

	OrganizationID uuid.UUID
	UserID         uuid.UUID




	ProjectID uuid.UUID





	ExecMode string




	Hardspec bool



	RawText string



	Mode Mode



	StartingPhase PhaseSlug



	SkipPhases []PhaseSlug


	AsyncTimeout time.Duration



	ExpressMaxLines int



	Metadata map[string]any
}

// OrchestrateResult es lo que devuelve Service.Run sincrónico. Los modos
// asíncronos devuelven inmediatamente con OrchestratorRunID + FlowRunID
// y status='pending'; el cliente debe pollear o suscribirse a signals.
type OrchestrateResult struct {


	OrchestratorRunID uuid.UUID


	FlowRunID uuid.UUID



	Mode Mode


	StartedAt time.Time




	SnapshotPrompt string




	Plan *PhasePlanSummary
}

// PhasePlanSummary es la vista exportada del modes.PhasePlan, sin
// importar el subpaquete modes desde callers externos.
type PhasePlanSummary struct {
	Mode  string
	Steps []PhaseStepSummary
}

// PhaseStepSummary es la fase individual desde la perspectiva del
// caller. El cliente IDE recibe esto y ejecuta usando AgentTemplateSlug
// como referencia para resolver agent_templates → agent_id real.
type PhaseStepSummary struct {
	ID                uuid.UUID
	Slug              PhaseSlug
	AgentTemplateSlug string
	SystemPrompt      string
	UserPrompt        string
	SuggestedSaves    []SuggestedSaveSummary
	RetryPolicy       string
	SkillThreshold    float64
}

// SuggestedSaveSummary expone el contrato D5 sin reexportar el tipo del
// subpaquete phases.
type SuggestedSaveSummary struct {
	Type     string
	Required bool
	Hint     string
}
