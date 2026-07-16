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
	fmt.Fprintln(&b, "ANTES de redactar: si hay ambigüedades, decisiones abiertas o supuestos")
	fmt.Fprintln(&b, "no confirmados, CONSULTA al usuario con AskUserQuestion (opciones concretas")
	fmt.Fprintln(&b, "+ 'Other' para que escriba su propia respuesta) y espera su respuesta. Una")
	fmt.Fprintln(&b, "pregunta a la vez. NO uses prosa plana ni ejecutes esta fase en un subagente")
	fmt.Fprintln(&b, "(AskUserQuestion no está disponible en subagentes). REQ-55 issue-55.1.")
	fmt.Fprintln(&b, "No especules requisitos: el spec fija el contrato (REQ-54 issue-54.7).")
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
		// R5-A: contrato de output declarado upfront para que el cliente sepa
		// qué campos son obligatorios antes de reportar la fase.
		OutputSchema: map[string]any{
			"type":     "object",
			"required": []any{"issue_slug", "issue_md"},
			"properties": map[string]any{
				"issue_slug": map[string]any{"type": "string", "description": "slug del issue creado"},
				"issue_md":   map[string]any{"type": "string", "description": "contenido markdown de la spec"},
			},
		},
	}, nil
}

func (h *sddSpecHandler) Validate(_ context.Context, _ *Output, result ClientResult) error {
	if result.Output == nil {
		return errors.New("sdd-spec: cliente devolvió Output nulo")
	}
	// R5-B: acumular TODOS los campos faltantes en una pasada, en vez de
	// retornar al primero. El cliente ve la lista completa en un solo rechazo.
	var missing []string
	if slug, _ := result.Output["issue_slug"].(string); slug == "" {
		missing = append(missing, "issue_slug")
	}
	if md, _ := result.Output["issue_md"].(string); md == "" {
		missing = append(missing, "issue_md (contenido de la spec)")
	}
	if len(missing) > 0 {
		return fmt.Errorf("sdd-spec: campos requeridos faltantes: %s", strings.Join(missing, ", "))
	}
	return nil
}
