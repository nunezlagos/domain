package phases

import (
	"context"
	"errors"
	"fmt"
	"strings"
)





// onboardSubagentPlan delega el recall en domain-memory. Criterio de DOMAINSERV-180 que quedó
// sin entregar: la fase pedía "buscar via mem_search" desde el hilo principal, o sea traerse el
// recall completo al contexto que justo esta fase intenta no ensuciar.
const onboardSubagentPlan = `Antes de decidir si el descubrimiento amerita un knowledge_doc, determinar si ya está documentado: delegar esa consulta en el agente domain-memory (si no está disponible: hacerlo en el hilo principal con domain_mem_search y domain_knowledge_search).

Lo que se delega es la BÚSQUEDA, no la decisión: el agente devuelve qué hay ya persistido y con qué ids, y este hilo decide si el descubrimiento es nuevo, si funde con algo existente, o si no amerita nada. Un agente que no vio la implementación no puede juzgar si lo aprendido es novedoso.

Si el retorno viene degradado —MCP sin responder, búsqueda sin project_slug— tratarlo como "no se pudo consultar", NO como "no hay nada documentado": crear un doc duplicado es peor que no crearlo.`

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
		SubagentPlan:      onboardSubagentPlan,
		RetryPolicy:       RetryReemit,
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
