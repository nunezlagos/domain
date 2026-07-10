package phases

import (
	"context"
	"errors"
	"fmt"
	"strings"
)



type sddExploreHandler struct{}

func NewSDDExploreHandler() Handler { return &sddExploreHandler{} }

func (h *sddExploreHandler) Slug() PhaseSlug { return PhaseSlug("sdd-explore") }

func (h *sddExploreHandler) Build(_ context.Context, in Input) (*Output, error) {
	if in.RawText == "" {
		return nil, errors.New("sdd-explore: RawText vacío")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Prompt del usuario:\n%s\n\n", in.RawText)
	fmt.Fprintln(&b, "Analiza el prompt y devuelve el JSON descrito en el system_prompt.")
	fmt.Fprintln(&b, "Si detectas multi-concern, lista los concerns separables (RFC 0006 D2).")
	return &Output{
		AgentTemplateSlug: "sdd-explore",
		SystemPrompt:      "",
		UserPrompt:        b.String(),


		SuggestedSaves: []SuggestedSave{
			{Type: "knowledge_doc", Required: false,
				Hint: "guardar knowledge_doc si descubriste contexto reusable (módulos, decisiones previas)"},
		},
		SkillThreshold: 0,
		// Code graph retirado (2026-07-07): se eliminó el contrato
		// domain_code_graph — obligaba una llamada burocrática a un grafo
		// sin uso real (auditoría: 45-94% nodos basura, consumo casi 100%
		// automático).
		RequiredToolCalls: nil,
		// REQ-54 issue-54.5: exploración paralela por área.
		SubagentPlan: "Detectá las áreas del codebase relevantes a la tarea (máx 4: ej. rutas/handlers, servicios/lógica, esquema/migraciones, tests) y lanzá UN subagente Explore POR ÁREA, en paralelo. Cada subagente recibe el contexto preparado de su área y devuelve un mapa con referencias file:line. Mergeá los resultados en un único mapa sin duplicados; marcá contradicciones entre áreas como hallazgos.",
		RetryPolicy:    RetryReemit,
	}, nil
}

func (h *sddExploreHandler) Validate(_ context.Context, _ *Output, result ClientResult) error {
	if result.Output == nil {
		return errors.New("sdd-explore: cliente devolvió Output nulo")
	}
	if intent, _ := result.Output["intent"].(string); intent == "" {
		return errors.New("sdd-explore: campo 'intent' (string) requerido en output — describí en 1-2 líneas qué hay que resolver, derivado de la exploración")
	}


	return nil
}
