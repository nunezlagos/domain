package phases

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// sddApplyHandler implementa Handler para la fase sdd-apply.
//
// El system_prompt NO vive en este archivo: es source-of-truth en BD
// (agent_templates.system_prompt, seedeado por SeedAgentTemplatesForOrg
// con catálogo v3). El Service.Run hace lookup vía Repository y rellena
// PhaseStep.SystemPrompt antes de despachar al cliente IDE. Esto permite
// que el operador del despliegue customice los prompts vía MCP/UI sin
// recompilar el binario, y mantiene la convención de
// .claude/rules/ai-generation.md (TODO en BD).
type sddApplyHandler struct{}

// NewSDDApplyHandler construye el handler. Stateless; puede existir
// uno global, pero el registry no obliga singleton.
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
	fmt.Fprintln(&b, "Implementa la tarea siguiendo TDD estricto y las reglas del repositorio.")
	fmt.Fprintln(&b, "Al terminar, llama a domain_orchestrate_phase_result con el JSON descrito.")
	return &Output{
		AgentTemplateSlug: "sdd-apply",

		SystemPrompt: "",
		UserPrompt:   b.String(),
		SuggestedSaves: []SuggestedSave{
			{
				Type: "code_reference",
				// code_graph retirado (2026-07-07): los agentes ya no producen
				// code_reference, asi que exigirlo mataba todo sdd-apply. Queda
				// sugerido (opcional) para no romper el pipeline.
				Required: false,
				Hint:     "guardar code_reference apuntando al archivo + identifier principal modificado",
			},
		},

		SkillThreshold: 0,

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
