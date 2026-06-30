package phases

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type sddDesignHandler struct{}

func NewSDDDesignHandler() Handler { return &sddDesignHandler{} }

func (h *sddDesignHandler) Slug() PhaseSlug { return PhaseSlug("sdd-design") }

func (h *sddDesignHandler) Build(_ context.Context, in Input) (*Output, error) {
	if in.RawText == "" {
		return nil, errors.New("sdd-design: RawText vacío")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Tarea original:\n%s\n\n", in.RawText)
	if spec, ok := in.PriorOutputs[PhaseSlug("sdd-spec")]; ok {
		if md, ok := spec["issue_md"].(string); ok && md != "" {
			fmt.Fprintf(&b, "Spec aprobada:\n%s\n\n", md)
		}
	}
	if propose, ok := in.PriorOutputs[PhaseSlug("sdd-propose")]; ok {
		if md, ok := propose["proposal_md"].(string); ok && md != "" {
			fmt.Fprintf(&b, "Proposal (draft):\n%s\n\n", md)
		}
	}
	fmt.Fprintln(&b, "Produce design.md con ADRs + test_plan + sabotage_plan.")
	fmt.Fprintln(&b, "Recuerda: D5 REQUIERE guardar un mem_save type='adr' por cada ADR.")
	return &Output{
		AgentTemplateSlug: "sdd-design",
		SystemPrompt:      "",
		UserPrompt:        b.String(),
		// RFC 0006 D5: ADRs son REQUIRED. La fase no avanza hasta que el
		// cliente reporte al menos un memory_ref type='adr' en MemoryRefsSaved.
		// Feature B: el design.md es además un DOCUMENTO de primera clase;
		// Required=true sobre knowledge_doc garantiza su registro en BD.
		SuggestedSaves: []SuggestedSave{
			{Type: "adr", Required: true,
				Hint: "guardar al menos 1 ADR con type='adr' por decisión arquitectónica"},
			{Type: "knowledge_doc", Required: true,
				Hint: "persistí el design como knowledge_doc para que quede registro del change en BD"},
		},
		SkillThreshold: 0,
		RetryPolicy:    RetryReemit,
	}, nil
}

func (h *sddDesignHandler) Validate(_ context.Context, _ *Output, result ClientResult) error {
	if result.Output == nil {
		return errors.New("sdd-design: cliente devolvió Output nulo")
	}
	if md, _ := result.Output["design_md"].(string); md == "" {
		return errors.New("sdd-design: campo 'design_md' requerido")
	}

	if adrs, _ := result.Output["adrs"].([]any); len(adrs) == 0 {
		return errors.New("sdd-design: array 'adrs' requerido (al menos 1 entry)")
	}
	return nil
}
