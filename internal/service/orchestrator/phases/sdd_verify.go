package phases

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// sddVerifyHandler — fase sdd-verify. El system_prompt es source-of-truth
// en BD (agent_templates.system_prompt). Service.Run lo rellena via
// Repository.GetAgentTemplateSystemPrompt antes de despachar al cliente.

type sddVerifyHandler struct{}

func NewSDDVerifyHandler() Handler { return &sddVerifyHandler{} }

func (h *sddVerifyHandler) Slug() PhaseSlug { return PhaseSlug("sdd-verify") }

func (h *sddVerifyHandler) Build(_ context.Context, in Input) (*Output, error) {
	if in.RawText == "" {
		return nil, errors.New("sdd-verify: RawText vacío")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Tarea original del usuario:\n%s\n\n", in.RawText)
	// El output de sdd-apply puede no estar disponible al momento de Build
	// si el dispatcher es Express (pre-arma todo el plan up-front). En ese
	// caso, el cliente IDE tiene el resultado del apply en su contexto
	// inmediato — sólo necesita la consigna. Cuando sí hay output disponible
	// (modo Full, donde verify se construye recién después de que apply
	// termina), lo incluimos para que el prompt sea más concreto.
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
	fmt.Fprintln(&b, "Al terminar, llama a domain_orchestrate_phase_result con el JSON descrito.")
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
		// "re-emit": si el verify queda colgado, el saga puede reanudarlo
		// porque no toca disco — es read-only sobre el output de apply.
		RetryPolicy: RetryReemit,
	}, nil
}

func (h *sddVerifyHandler) Validate(_ context.Context, _ *Output, result ClientResult) error {
	if result.Output == nil {
		return errors.New("sdd-verify: cliente devolvió Output nulo")
	}
	// Shape básico — el orquestador usa esto para decidir si encadena
	// a sdd-judge (scenarios_failed vacío) o vuelve a sdd-apply (algún
	// escenario rojo).
	failed, _ := result.Output["scenarios_failed"].([]any)
	if blockers, ok := result.Output["blockers"].([]any); ok && len(blockers) > 0 {
		// Un blocker convierte la fase en "necesita humano" sin importar
		// los scenarios.
		return ErrPhaseBlockedByClient
	}
	if len(failed) > 0 {
		// No es error duro: el service routear de vuelta a sdd-apply.
		// Devolvemos un error tipado para que el dispatcher distinga.
		return ErrVerificationFailed
	}
	return nil
}

// ErrVerificationFailed señala que scenarios_failed tiene contenido.
// El dispatcher de Full mode debe reagendar sdd-apply; en Express
// el flow termina marcando el run failed (el caller debe iterar).
var ErrVerificationFailed = errors.New("verify reported failed scenarios — orchestrator must re-loop apply")
