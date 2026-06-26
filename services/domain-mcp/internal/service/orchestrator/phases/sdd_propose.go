package phases

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type sddProposeHandler struct{}

func NewSDDProposeHandler() Handler { return &sddProposeHandler{} }

func (h *sddProposeHandler) Slug() PhaseSlug { return PhaseSlug("sdd-propose") }

func (h *sddProposeHandler) Build(_ context.Context, in Input) (*Output, error) {
	if in.RawText == "" {
		return nil, errors.New("sdd-propose: RawText vacío")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Tarea original:\n%s\n\n", in.RawText)
	if spec, ok := in.PriorOutputs[PhaseSlug("sdd-spec")]; ok {
		if slug, ok := spec["issue_slug"].(string); ok {
			fmt.Fprintf(&b, "Issue slug: %s\n", slug)
		}
		if md, ok := spec["issue_md"].(string); ok && md != "" {
			fmt.Fprintf(&b, "\nSpec previa:\n%s\n\n", md)
		}
	}
	fmt.Fprintln(&b, "Genera proposal.md con scope formal del cambio + esfuerzo estimado.")
	return &Output{
		AgentTemplateSlug: "sdd-propose",
		SystemPrompt:      "",
		UserPrompt:        b.String(),
		// RFC 0006 D5 + Feature B: el proposal es un DOCUMENTO de primera
		// clase. Required=true obliga al cliente a persistirlo como
		// knowledge_doc antes de avanzar, garantizando registro en BD.
		SuggestedSaves: []SuggestedSave{
			{Type: "knowledge_doc", Required: true,
				Hint: "persistí el proposal como knowledge_doc para que quede registro del change en BD"},
		},
		SkillThreshold: 0,
		RetryPolicy:    RetryReemit,
	}, nil
}

func (h *sddProposeHandler) Validate(_ context.Context, _ *Output, result ClientResult) error {
	if result.Output == nil {
		return errors.New("sdd-propose: cliente devolvió Output nulo")
	}
	if md, _ := result.Output["proposal_md"].(string); md == "" {
		return errors.New("sdd-propose: campo 'proposal_md' requerido")
	}
	if status, _ := result.Output["status"].(string); status != "draft" {

		return errors.New("sdd-propose: status debe ser 'draft' — promoción requiere paso explícito")
	}
	return nil
}
