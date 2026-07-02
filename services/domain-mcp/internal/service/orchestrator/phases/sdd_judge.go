package phases

import (
	"context"
	"errors"
	"fmt"
	"strings"
)




type sddJudgeHandler struct{}

func NewSDDJudgeHandler() Handler { return &sddJudgeHandler{} }

func (h *sddJudgeHandler) Slug() PhaseSlug { return PhaseSlug("sdd-judge") }

func (h *sddJudgeHandler) Build(_ context.Context, in Input) (*Output, error) {
	if in.RawText == "" {
		return nil, errors.New("sdd-judge: RawText vacío")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Tarea original:\n%s\n\n", in.RawText)
	if verify, ok := in.PriorOutputs[PhaseSlug("sdd-verify")]; ok {
		if summary, ok := verify["summary"].(string); ok && summary != "" {
			fmt.Fprintf(&b, "Resumen de verify:\n%s\n\n", summary)
		}
	}
	if design, ok := in.PriorOutputs[PhaseSlug("sdd-design")]; ok {
		if sab, ok := design["sabotage_plan"].([]any); ok && len(sab) > 0 {
			fmt.Fprintln(&b, "Sabotage plan declarado en design:")
			for _, s := range sab {
				if m, ok := s.(map[string]any); ok {
					fmt.Fprintf(&b, "  - %v::%v (invariante: %v)\n", m["file"], m["func"], m["invariant"])
				}
			}
			fmt.Fprintln(&b)
		}
	}
	fmt.Fprintln(&b, "Ejecuta cada test de sabotaje: rompe la invariante, valida que el test rojo")
	fmt.Fprintln(&b, "atrapa la regresión, restaura. Reporta los resultados.")
	fmt.Fprintln(&b, "D5 REQUIERE guardar mem_save type='sabotage_record' por cada test ejecutado.")
	return &Output{
		AgentTemplateSlug: "sdd-judge",
		SystemPrompt:      "",
		UserPrompt:        b.String(),


		SuggestedSaves: []SuggestedSave{
			{Type: "sabotage_record", Required: true,
				Hint: "guardar 1 sabotage_record por test de sabotaje ejecutado (invariante + verdict)"},
		},
		SkillThreshold: 0,
		// REQ-54 issue-54.5: panel adversarial. El shape del Output exige >=2
		// judge_verdicts (Validate) — un solo juez no puede cerrar la fase.
		SubagentPlan: "Además del sabotaje, lanzá DOS jueces adversariales CIEGOS en paralelo (subagentes que NO ven el veredicto del otro): juez A evalúa correctness (¿el cambio hace lo que el spec pide?), juez B evalúa robustez/seguridad (¿qué rompe, qué caso borde falta?). Cada juez devuelve {role, verdict: approve|reject, findings[]}. Si divergen, lanzá un tercer juez de desempate. Reportá TODOS en judge_verdicts (mínimo 2).",
		RetryPolicy:    RetryReemit,
	}, nil
}

func (h *sddJudgeHandler) Validate(_ context.Context, _ *Output, result ClientResult) error {
	if result.Output == nil {
		return errors.New("sdd-judge: cliente devolvió Output nulo")
	}
	records, _ := result.Output["sabotage_records"].([]any)
	if len(records) == 0 {
		return errors.New("sdd-judge: array 'sabotage_records' requerido (al menos 1 entry)")
	}
	// REQ-54 issue-54.5: teeth del panel adversarial. El paralelismo no se
	// puede fingir sin mentir explícitamente: mínimo 2 veredictos de jueces
	// distintos (roles correctness + robustez).
	verdicts, _ := result.Output["judge_verdicts"].([]any)
	if len(verdicts) < 2 {
		return errors.New("sdd-judge: 'judge_verdicts' requiere >= 2 veredictos del panel adversarial (issue-54.5)")
	}
	return nil
}
