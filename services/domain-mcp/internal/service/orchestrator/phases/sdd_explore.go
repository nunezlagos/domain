package phases

import (
	"context"
	"errors"
	"fmt"
	"strings"
)



// exploreSubagentPlan reparte las tareas del system_prompt por AGENTE, no por
// área: las tareas 4 y 5 buscan cosas distintas (memoria vs repo) y las cubren
// agentes distintos del catálogo. El mapa de diseño se queda en un subagente de
// exploración amplia porque repo-scout se autoexcluye del análisis cross-file.
const exploreSubagentPlan = `Reparte la exploración EN PARALELO y combina los resultados en un único mapa sin duplicados; marca las contradicciones como hallazgos.

1. Issues/HUs similares ya implementadas (tarea 4): delega en el agente domain-memory (` + MarcaFallback + ` hazlo en el hilo principal con domain_mem_search y domain_knowledge_search).
2. Handlers, servicios y paths reales afectados (tarea 5): delega en el agente repo-scout (` + MarcaFallback + ` usa un subagente read-only de búsqueda, o Read/Grep/Glob en el hilo principal).
3. Mapa de diseño del área cuando el cambio cruza módulos: NO es trabajo de repo-scout. Usa un subagente de exploración amplia con Read/Grep/Glob/Bash, uno por área (máx 4: rutas/handlers, servicios/lógica, esquema/migraciones, tests).

Cada subagente devuelve referencias file:line concretas, nunca salida cruda de tools. Los agentes nombrados son una preferencia: si el cliente no los tiene instalados, la fase se ejecuta igual por el camino alternativo.`

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
		// REQ-54 issue-54.5: exploración paralela. DOMAINSERV-180: el reparto es
		// por agente del catálogo, no por área.
		SubagentPlan: exploreSubagentPlan,
		RetryPolicy:  RetryReemit,
	}, nil
}

func (h *sddExploreHandler) Validate(_ context.Context, _ *Output, result ClientResult) error {
	if result.Output == nil {
		return errors.New("sdd-explore: cliente devolvió Output nulo")
	}
	if intent, _ := result.Output["intent"].(string); intent == "" {
		return errors.New("sdd-explore: campo 'intent' (string) requerido en output — describe en 1-2 líneas qué hay que resolver, derivado de la exploración")
	}


	return nil
}
