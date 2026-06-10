package phases

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// sddApplySystemPrompt es la guía para el agente sdd-apply (cliente IDE).
// Sigue TDD strict (.claude/rules/sdd.md): test → impl mínima → refactor
// → sabotaje. Cada bullet referencia un rule del repo para que el modelo
// pueda navegar por contexto cuando tenga dudas.
const sddApplySystemPrompt = `Sos el agente sdd-apply del orquestador Domain. Tu trabajo es implementar
la tarea ATÓMICA descrita en el UserPrompt usando TDD estricto.

CONTRATO DURO:
- Una sola intención por iteración. Si detectás multi-concern, parás y
  devolvés error multi_concern_detected — no es tu lugar splittear.
- Orden obligatorio: TEST primero (debe fallar por la razón correcta) →
  implementación mínima → refactor → sabotaje opcional.
- Respetás .claude/rules/* — clean-architecture, db, security, testing,
  observability, api, go, migrations. No inventes convenciones nuevas.
- NUNCA hardcodeás secrets, NUNCA log de PII (ver security.md), NUNCA
  rompés convenciones de conventional commits (git.md).

OUTPUT esperado (JSON):
  {
    "files_changed":  ["path/a/file.go", ...],
    "tests_added":    ["path/test_file.go::TestX_Scenario", ...],
    "commits_made":   ["sha-or-pending"],
    "multi_concern":  false,
    "summary":        "1-3 oraciones sobre qué quedó hecho"
  }

OBLIGACIÓN D5 (suggested_saves):
- Antes de devolver el phase_result, ejecutás mem_save con type
  'code_reference' apuntando al archivo + identifier principal que
  cambiaste. Es REQUIRED — si no lo guardás, la fase no avanza.

Si te bloquea algo que no podés resolver solo (ambigüedad en spec,
decisión arquitectónica), devolvés output.blocked=true con un campo
question describiendo qué necesita el humano. NO inventes.`

// sddApplyHandler implementa Handler para la fase sdd-apply.
type sddApplyHandler struct{}

// NewSDDApplyHandler construye el handler. Stateless; podés tener uno
// global, pero el registry no obliga singleton.
func NewSDDApplyHandler() Handler { return &sddApplyHandler{} }

func (h *sddApplyHandler) Slug() PhaseSlug { return PhaseSlug("sdd-apply") }

// Build arma el prompt user-facing usando los outputs previos (tasks
// que el agente sdd-tasks generó) + el raw text del usuario.
func (h *sddApplyHandler) Build(_ context.Context, in Input) (*Output, error) {
	if in.RawText == "" {
		return nil, errors.New("sdd-apply: RawText vacío — el orquestador debe propagar el prompt original")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Tarea original del usuario:\n%s\n\n", in.RawText)
	if tasks, ok := in.PriorOutputs[PhaseSlug("sdd-tasks")]; ok {
		if list, ok := tasks["tasks"].([]any); ok && len(list) > 0 {
			fmt.Fprintln(&b, "Tasks previamente descompuestas por sdd-tasks:")
			for i, t := range list {
				fmt.Fprintf(&b, "  %d. %v\n", i+1, t)
			}
			fmt.Fprintln(&b)
		}
	}
	fmt.Fprintln(&b, "Implementá la tarea siguiendo TDD strict + las rules del repo.")
	fmt.Fprintln(&b, "Cuando termines, llamá domain_orchestrate_phase_result con el JSON descripto.")
	return &Output{
		AgentTemplateSlug: "sdd-apply",
		SystemPrompt:      sddApplySystemPrompt,
		UserPrompt:        b.String(),
		SuggestedSaves: []SuggestedSave{
			{
				Type:     "code_reference",
				Required: true, // RFC 0006 D5
				Hint:     "guardar code_reference apuntando al archivo + identifier principal modificado",
			},
		},
		// SkillThreshold lo override el service desde agent_templates.metadata
		// (D3); el handler declara el zero value para que el lookup decida.
		SkillThreshold: 0,
		// RetryPolicy "require-cleanup": si el step queda colgado o muere
		// con cambios parciales en disco, el saga handler debe disparar
		// cleanup_required (heartbeat-watcher issue-08.11 lo emite).
		RetryPolicy: RetryCleanup,
	}, nil
}

// Validate corre el contrato D5 + chequeos shape específicos de apply.
// El service llama ValidateRequiredSaves (centralizado) antes que esto;
// acá agregamos sólo lo propio del apply (output.multi_concern, etc).
func (h *sddApplyHandler) Validate(_ context.Context, _ *Output, result ClientResult) error {
	if result.Output == nil {
		return errors.New("sdd-apply: cliente devolvió Output nulo")
	}
	if multi, ok := result.Output["multi_concern"].(bool); ok && multi {
		// Multi-concern no es failure: es una señal para que el orquestador
		// haga split (D2). El handler lo devuelve como error tipado para
		// que el modo Express (single-concern por definición) lo trate
		// distinto que Full (que sí puede splittear).
		return ErrMultiConcernDetected
	}
	if blocked, _ := result.Output["blocked"].(bool); blocked {
		return ErrPhaseBlockedByClient
	}
	return nil
}

// Errores específicos del handler sdd-apply. Viven en el paquete phases
// porque otros handlers podrían reusarlos (sdd-explore también detecta
// multi-concern). El service convierte estos en errores http/mcp en el
// dispatcher.
var (
	ErrMultiConcernDetected = errors.New("phase reported multi_concern=true — orchestrator must split")
	ErrPhaseBlockedByClient = errors.New("phase reported blocked=true — needs human input")
)
