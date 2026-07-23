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
	hasIssue := false
	if spec, ok := in.PriorOutputs[PhaseSlug("sdd-spec")]; ok {
		if slug, ok := spec["issue_slug"].(string); ok && slug != "" {
			fmt.Fprintf(&b, "Issue a archivar: %s\n\n", slug)
			hasIssue = true
		}
	}
	fmt.Fprintln(&b, "Marca el issue como implemented y archivado (issue-13.x).")
	fmt.Fprintln(&b, "Actualiza el CHANGELOG.md Unreleased con entrada del issue.")
	if !hasIssue {
		// DOMAINSERV-89: en Lite (sin sdd-spec) puede no haber issue/change que
		// archivar; en ese caso reportá nothing_to_archive=true en vez de forzar.
		fmt.Fprintln(&b, "Si NO hay issue/change openspec asociado (modo liviano sin spec), "+
			"reportá nothing_to_archive=true (no inventes un issue).")
	}
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
	// DOMAINSERV-89: en Lite sin issue/change, nothing_to_archive=true cierra la
	// fase sin exigir archived (evita romper changes livianos sin spec).
	if nothing, _ := result.Output["nothing_to_archive"].(bool); nothing {
		return nil
	}
	if archived, _ := result.Output["archived"].(bool); !archived {
		return errors.New("sdd-archive: 'archived'=true o 'nothing_to_archive'=true requerido")
	}
	return nil
}
