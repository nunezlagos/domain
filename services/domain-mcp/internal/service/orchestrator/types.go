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
//   - ModeMicro: fast path MÍNIMO para ediciones triviales sin lógica
//     (texto de front, crear un script, 1 archivo, doc/config). Sólo
//     sdd-apply — NO corre sdd-verify. El commit-gate del cliente EXENTA
//     estos flows del requisito de tests (marker mode=micro). Rutea aquí
//     cuando el cambio no toca lógica testeable. Opt-in explícito.
//   - ModeExpress: fast path para cambios ≤10 líneas single-file. Sólo
//     sdd-apply + sdd-verify. Confirm condicional D1.
//   - ModeLite: camino reducido para cambios triviales (fix de 1 línea,
//     doc, refactor chico). Corre un SUBSET de fases (default
//     sdd-explore → sdd-apply → sdd-verify) salteando las pesadas
//     (propose/design/tasks/judge/archive/onboard). Más amplio que
//     Express (incluye explore para ubicar el cambio) pero mucho más
//     barato que Full. Opt-in: nunca es el default.
//   - ModeFull: pipeline completo de 12 fases.
//   - ModeSolo: ejecución inline server-side via LLM provider directo
//     (sin cliente IDE colaborador).
//   - ModeDetect: dry-run; persiste todo a status='draft' sin ejecutar
//     acciones del cliente. Útil para preview.
//   - ModeAsync: emite flow_signals y resume vía worker; el caller
//     desconecta. NO compatible con ModeExpress (D6).
type Mode string

const (
	ModeMicro   Mode = "micro"
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
	case ModeMicro, ModeExpress, ModeLite, ModeFull, ModeSolo, ModeDetect, ModeAsync:
		return true
	}
	return false
}

// PhaseSlug identifica una fase del pipeline. El catálogo v3 declara los
// 11 templates sdd-* (1 orchestrator + 10 phase-workers). El registry
// (internal/service/orchestrator/phases) resuelve slug → handler.
type PhaseSlug string

const (
	PhaseExplore PhaseSlug = "sdd-explore"
	PhaseSpec    PhaseSlug = "sdd-spec"
	PhasePropose PhaseSlug = "sdd-propose"
	PhaseDesign  PhaseSlug = "sdd-design"
	PhaseTasks   PhaseSlug = "sdd-tasks"
	PhaseApply   PhaseSlug = "sdd-apply"
	PhaseVerify  PhaseSlug = "sdd-verify"
	PhaseJudge   PhaseSlug = "sdd-judge"
	Phase4R      PhaseSlug = "sdd-4r"
	PhaseReview  PhaseSlug = "sdd-review"
	PhaseArchive PhaseSlug = "sdd-archive"
	PhaseOnboard PhaseSlug = "sdd-onboard"
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
	OrchestratorRunID uuid.UUID `json:"orchestrator_run_id"`

	// DOMAINSERV-108: json tag snake_case OBLIGATORIO. El hook post-orchestrate
	// extrae flow_run_id del resultado para mintear el token del gate SDD; sin
	// tag marshalaba "FlowRunID" (PascalCase) y el hook nunca lo encontraba.
	FlowRunID uuid.UUID `json:"flow_run_id"`

	Mode Mode `json:"mode"`

	StartedAt time.Time `json:"started_at"`

	SnapshotPrompt string `json:"snapshot_prompt,omitempty"`

	Plan *PhasePlanSummary `json:"plan,omitempty"`
}

// PhasePlanSummary es la vista exportada del modes.PhasePlan, sin
// importar el subpaquete modes desde callers externos.
type PhasePlanSummary struct {
	Mode  string             `json:"mode"`
	Steps []PhaseStepSummary `json:"steps"`
}

// PhaseStepSummary es la fase individual desde la perspectiva del
// caller. El cliente IDE recibe esto y ejecuta usando AgentTemplateSlug
// como referencia para resolver agent_templates → agent_id real.
type PhaseStepSummary struct {
	ID                uuid.UUID              `json:"id"`
	Slug              PhaseSlug              `json:"slug"`
	AgentTemplateSlug string                 `json:"agent_template_slug,omitempty"`
	SystemPrompt      string                 `json:"system_prompt,omitempty"`
	UserPrompt        string                 `json:"user_prompt,omitempty"`
	SuggestedSaves    []SuggestedSaveSummary `json:"suggested_saves,omitempty"`
	RetryPolicy       string                 `json:"retry_policy,omitempty"`
	SkillThreshold    float64                `json:"skill_threshold,omitempty"`

	// RequiredToolCalls: tools domain_* que la fase exige que el cliente
	// invoque antes de cerrar el step (R5-A). Se declara upfront para que el
	// cliente conozca el contrato sin descubrirlo por rechazo. Aditivo.
	RequiredToolCalls []string `json:"required_tool_calls,omitempty"`

	// OutputSchema: JSON Schema del output esperado por la fase (R5-A). Permite
	// al cliente validar su reporte antes de enviarlo. Aditivo; vacío si la
	// fase no lo declara.
	OutputSchema map[string]any `json:"output_schema,omitempty"`
}

// SuggestedSaveSummary expone el contrato D5 sin reexportar el tipo del
// subpaquete phases.
type SuggestedSaveSummary struct {
	Type     string `json:"type"`
	Required bool   `json:"required"`
	Hint     string `json:"hint,omitempty"`
}
