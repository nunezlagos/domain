package phases

import (
	"context"
	"errors"
	"fmt"
	"strings"
)



type sddSpecHandler struct{}

func NewSDDSpecHandler() Handler { return &sddSpecHandler{} }

func (h *sddSpecHandler) Slug() PhaseSlug { return PhaseSlug("sdd-spec") }

func (h *sddSpecHandler) Build(_ context.Context, in Input) (*Output, error) {
	if in.RawText == "" {
		return nil, errors.New("sdd-spec: RawText vacío")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Prompt del usuario:\n%s\n\n", in.RawText)
	if explore, ok := in.PriorOutputs[PhaseSlug("sdd-explore")]; ok {
		if intent, ok := explore["intent"].(string); ok {
			fmt.Fprintf(&b, "Intent detectado: %s\n", intent)
		}
		if scope, ok := explore["scope"].(string); ok {
			fmt.Fprintf(&b, "Scope: %s\n", scope)
		}
		if mods, ok := explore["modules_affected"].([]any); ok && len(mods) > 0 {
			fmt.Fprintln(&b, "Módulos afectados:")
			for _, m := range mods {
				fmt.Fprintf(&b, "  - %v\n", m)
			}
		}
		fmt.Fprintln(&b)
	}
	fmt.Fprintln(&b, "Construye la spec issue.md siguiendo .claude/rules/sdd.md.")
	return &Output{
		AgentTemplateSlug: "sdd-spec",
		SystemPrompt:      "",
		UserPrompt:        b.String(),


		SuggestedSaves: []SuggestedSave{
			{Type: "knowledge_doc", Required: false,
				Hint: "guardar knowledge_doc apuntando al draft_id del issuebuilder si aplica"},
		},
		SkillThreshold: 0,
		RetryPolicy:    RetryReemit,
	}, nil
}

func (h *sddSpecHandler) Validate(_ context.Context, _ *Output, result ClientResult) error {
	if result.Output == nil {
		return errors.New("sdd-spec: cliente devolvió Output nulo")
	}
	if slug, _ := result.Output["issue_slug"].(string); slug == "" {
		return errors.New("sdd-spec: campo 'issue_slug' requerido")
	}
	if md, _ := result.Output["issue_md"].(string); md == "" {
		return errors.New("sdd-spec: campo 'issue_md' requerido (contenido de la spec)")
	}
	return nil
}
