package phases

import (
	"context"
	"errors"
	"fmt"
	"strings"
)




type sddArchiveHandler struct{}

func NewSDDArchiveHandler() Handler { return &sddArchiveHandler{} }

func (h *sddArchiveHandler) Slug() PhaseSlug { return PhaseSlug("sdd-archive") }

func (h *sddArchiveHandler) Build(_ context.Context, in Input) (*Output, error) {
	if in.RawText == "" {
		return nil, errors.New("sdd-archive: RawText vacío")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Tarea original:\n%s\n\n", in.RawText)
	if spec, ok := in.PriorOutputs[PhaseSlug("sdd-spec")]; ok {
		if slug, ok := spec["issue_slug"].(string); ok {
			fmt.Fprintf(&b, "Issue a archivar: %s\n\n", slug)
		}
	}
	fmt.Fprintln(&b, "Marca el issue como implemented y archivado (issue-13.x).")
	fmt.Fprintln(&b, "Actualiza el CHANGELOG.md Unreleased con entrada del issue.")
	return &Output{
		AgentTemplateSlug: "sdd-archive",
		SystemPrompt:      "",
		UserPrompt:        b.String(),
		SuggestedSaves: []SuggestedSave{
			{Type: "knowledge_doc", Required: false,
				Hint: "guardar knowledge_doc apuntando al CHANGELOG diff + issue archivado"},
		},
		SkillThreshold: 0,
		// REQ-54 issue-54.6: archive verifica el estado openspec antes de cerrar.
		RequiredToolCalls: []string{"domain_openspec_status"},
		RetryPolicy:    RetryReemit,
	}, nil
}

func (h *sddArchiveHandler) Validate(_ context.Context, _ *Output, result ClientResult) error {
	if result.Output == nil {
		return errors.New("sdd-archive: cliente devolvió Output nulo")
	}
	if archived, _ := result.Output["archived"].(bool); !archived {
		return errors.New("sdd-archive: campo 'archived'=true requerido")
	}
	return nil
}
