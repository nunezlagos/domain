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
		// REQ-54 issue-54.6: explore debe partir del grafo de código vivo.
		RequiredToolCalls: []string{"domain_code_graph"},
		RetryPolicy:    RetryReemit,
	}, nil
}

func (h *sddExploreHandler) Validate(_ context.Context, _ *Output, result ClientResult) error {
	if result.Output == nil {
		return errors.New("sdd-explore: cliente devolvió Output nulo")
	}
	if intent, _ := result.Output["intent"].(string); intent == "" {
		return errors.New("sdd-explore: campo 'intent' requerido en output")
	}


	return nil
}
