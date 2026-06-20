package phases

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// sddExploreHandler — fase sdd-explore. system_prompt en BD.

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
		// D5: explore es read-only, sin required saves. Opcional knowledge_doc
		// si descubrió contexto útil para el archive.
		SuggestedSaves: []SuggestedSave{
			{Type: "knowledge_doc", Required: false,
				Hint: "guardar knowledge_doc si descubriste contexto reusable (módulos, decisiones previas)"},
		},
		SkillThreshold: 0,
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
	// multi_concern=true es válido — no es error; el dispatcher Full lo
	// usa para splitear. Acá sólo validamos el shape mínimo.
	return nil
}
