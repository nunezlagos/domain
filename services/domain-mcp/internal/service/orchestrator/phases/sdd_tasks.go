package phases

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type sddTasksHandler struct{}

func NewSDDTasksHandler() Handler { return &sddTasksHandler{} }

func (h *sddTasksHandler) Slug() PhaseSlug { return PhaseSlug("sdd-tasks") }

func (h *sddTasksHandler) Build(_ context.Context, in Input) (*Output, error) {
	if in.RawText == "" {
		return nil, errors.New("sdd-tasks: RawText vacío")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Tarea original:\n%s\n\n", in.RawText)
	if design, ok := in.PriorOutputs[PhaseSlug("sdd-design")]; ok {
		if md, ok := design["design_md"].(string); ok && md != "" {
			fmt.Fprintf(&b, "Design aprobado:\n%s\n\n", md)
		}
		if tp, ok := design["test_plan"].([]any); ok && len(tp) > 0 {
			fmt.Fprintln(&b, "Test plan derivado:")
			for _, t := range tp {
				if m, ok := t.(map[string]any); ok {
					fmt.Fprintf(&b, "  - %v::%v (%v)\n", m["file"], m["func"], m["scenario"])
				}
			}
			fmt.Fprintln(&b)
		}
	}
	fmt.Fprintln(&b, "Descompon en tasks atómicas con id + descripción + effort + depends_on.")
	fmt.Fprintln(&b, "SYNC OBLIGATORIO (REQ-55): al terminar, corré domain_openspec_export, "+
		"escribí los .md en openspec/changes/<change>/ con tu Write tool, y domain_openspec_apply. "+
		"Reportá ambas en tool_calls o el server RECHAZA el cierre de la fase.")
	return &Output{
		AgentTemplateSlug: "sdd-tasks",
		SystemPrompt:      "",
		UserPrompt:        b.String(),
		// RFC 0006 D5 + Feature B: la descomposición en tasks es un
		// DOCUMENTO de primera clase. Required=true obliga al cliente a
		// persistirla como knowledge_doc antes de avanzar (registro en BD).
		SuggestedSaves: []SuggestedSave{
			{Type: "knowledge_doc", Required: true,
				Hint: "persistí la descomposición de tasks como knowledge_doc para que quede registro en BD"},
		},
		SkillThreshold: 0,
		// REQ-55 issue-55.3: el sync openspec BD->repo es CONTRATO de esta fase,
		// no una nota en el protocolo global. Sin export+apply el server rechaza
		// el cierre (missing_tool_calls). Asi el openspec (tasks.md) de este change queda en openspec/.
		RequiredToolCalls: []string{"domain_openspec_export", "domain_openspec_apply"},
		RetryPolicy:    RetryReemit,
	}, nil
}

func (h *sddTasksHandler) Validate(_ context.Context, _ *Output, result ClientResult) error {
	if result.Output == nil {
		return errors.New("sdd-tasks: cliente devolvió Output nulo")
	}
	tasks, ok := result.Output["tasks"].([]any)
	if !ok || len(tasks) == 0 {
		return errors.New("sdd-tasks: array 'tasks' requerido (al menos 1 task)")
	}

	for i, raw := range tasks {
		m, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("sdd-tasks: task[%d] no es objeto", i)
		}
		if id, _ := m["id"].(string); id == "" {
			return fmt.Errorf("sdd-tasks: task[%d].id requerido", i)
		}
		if desc, _ := m["description"].(string); desc == "" {
			return fmt.Errorf("sdd-tasks: task[%d].description requerido", i)
		}
	}
	return nil
}
