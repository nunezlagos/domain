package phases

import (
	"context"
	"errors"
	"fmt"
	"strings"
)





type sddOnboardHandler struct{}

func NewSDDOnboardHandler() Handler { return &sddOnboardHandler{} }

func (h *sddOnboardHandler) Slug() PhaseSlug { return PhaseSlug("sdd-onboard") }

func (h *sddOnboardHandler) Build(_ context.Context, in Input) (*Output, error) {
	if in.RawText == "" {
		return nil, errors.New("sdd-onboard: RawText vacío")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Tarea original:\n%s\n\n", in.RawText)
	if spec, ok := in.PriorOutputs[PhaseSlug("sdd-spec")]; ok {
		if slug, ok := spec["issue_slug"].(string); ok {
			fmt.Fprintf(&b, "Issue completado: %s\n", slug)
		}
	}
	if apply, ok := in.PriorOutputs[PhaseSlug("sdd-apply")]; ok {
		if summary, ok := apply["summary"].(string); ok && summary != "" {
			fmt.Fprintf(&b, "Resumen de implementación:\n%s\n\n", summary)
		}
	}
	fmt.Fprintln(&b, "Si la implementación introduce conceptos no obvios (nuevo patrón,")
	fmt.Fprintln(&b, "convención, gotcha), genera un knowledge_doc breve que otros podrán")
	fmt.Fprintln(&b, "buscar via mem_search. Si no aplica, devuelve skipped=true.")
	return &Output{
		AgentTemplateSlug: "sdd-onboard",
		SystemPrompt:      "",
		UserPrompt:        b.String(),
		SuggestedSaves: []SuggestedSave{
			{Type: "knowledge_doc", Required: false,
				Hint: "guardar knowledge_doc con el descubrimiento si aplica (Required=false: skip si no hay nada que documentar)"},
		},
		SkillThreshold: 0,
		// REQ-54 issue-54.6: onboard materializa el conocimiento del cambio.
		RequiredToolCalls: []string{"domain_knowledge_save"},
		RetryPolicy:    RetryReemit,
	}, nil
}

func (h *sddOnboardHandler) Validate(_ context.Context, _ *Output, result ClientResult) error {
	if result.Output == nil {
		return errors.New("sdd-onboard: cliente devolvió Output nulo")
	}


	skipped, _ := result.Output["skipped"].(bool)
	docCreated, _ := result.Output["doc_created"].(bool)
	if !skipped && !docCreated {
		return errors.New("sdd-onboard: debe declarar 'skipped=true' o 'doc_created=true'")
	}
	return nil
}
