package phases

import (
	"context"
	"errors"
	"fmt"
	"strings"
)





type sddVerifyHandler struct{}

func NewSDDVerifyHandler() Handler { return &sddVerifyHandler{} }

func (h *sddVerifyHandler) Slug() PhaseSlug { return PhaseSlug("sdd-verify") }

func (h *sddVerifyHandler) Build(_ context.Context, in Input) (*Output, error) {
	if in.RawText == "" {
		return nil, errors.New("sdd-verify: RawText vacío")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Tarea original del usuario:\n%s\n\n", in.RawText)






	if apply, ok := in.PriorOutputs[PhaseSlug("sdd-apply")]; ok {
		if summary, ok := apply["summary"].(string); ok && summary != "" {
			fmt.Fprintf(&b, "Resumen de lo que reportó sdd-apply:\n%s\n\n", summary)
		}
		if files, ok := apply["files_changed"].([]any); ok && len(files) > 0 {
			fmt.Fprintln(&b, "Archivos modificados:")
			for _, f := range files {
				fmt.Fprintf(&b, "  - %v\n", f)
			}
			fmt.Fprintln(&b)
		}
	} else {
		fmt.Fprintln(&b, "Valida la salida que el agente sdd-apply produzca en este flow_run.")
		fmt.Fprintln(&b)
	}
	fmt.Fprintln(&b, "Valida los escenarios Gherkin del issue.md. NO modifiques código.")
	fmt.Fprintln(&b, "Corre la verificación con domain_verify_start → domain_verify_update_item (por escenario) → domain_verify_complete.")
	fmt.Fprintln(&b, "Al terminar, llama a domain_orchestrate_phase_result con el JSON descrito y tool_calls=[las tools que invocaste].")
	return &Output{
		AgentTemplateSlug: "sdd-verify",
		SystemPrompt:      "",
		UserPrompt:        b.String(),
		SuggestedSaves: []SuggestedSave{
			{
				Type:     "verification_result",
				Required: false, // RFC 0006 D5 — verify NO tiene required saves
				Hint:     "guardar verification_result apuntando al run si dura >5min",
			},
		},
		SkillThreshold: 0,

		// REQ-54 issue-54.1: fase piloto del contrato fase→tools. verify DEBE
		// haber corrido el ciclo de verificación server-side; sin estas tools el
		// orquestador rechaza el cierre (reintentable). Override editable en
		// agent_templates.metadata.required_tool_calls.
		RequiredToolCalls: []string{"domain_verify_start", "domain_verify_complete"},
		// REQ-54 issue-54.5: validación de escenarios en paralelo.
		SubagentPlan: "Agrupa los escenarios Gherkin del issue en lotes INDEPENDIENTES (sin estado compartido) y valida cada lote en un subagente paralelo. Cada subagente reporta scenarios_passed/scenarios_failed de su lote con evidencia. Combina los lotes; un escenario failed en cualquier lote = failed global.",

		RetryPolicy: RetryReemit,
	}, nil
}

func (h *sddVerifyHandler) Validate(_ context.Context, _ *Output, result ClientResult) error {
	if result.Output == nil {
		return errors.New("sdd-verify: cliente devolvió Output nulo")
	}



	failed, _ := result.Output["scenarios_failed"].([]any)
	if blockers, ok := result.Output["blockers"].([]any); ok && len(blockers) > 0 {


		return ErrPhaseBlockedByClient
	}
	if len(failed) > 0 {


		return ErrVerificationFailed
	}
	return nil
}

// ErrVerificationFailed señala que scenarios_failed tiene contenido.
// El dispatcher de Full mode debe reagendar sdd-apply; en Express
// el flow termina marcando el run failed (el caller debe iterar).
var ErrVerificationFailed = errors.New("verify reported failed scenarios — orchestrator must re-loop apply")
