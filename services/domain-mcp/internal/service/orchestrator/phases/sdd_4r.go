package phases

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// fourRLensCount son las 4 lenses del review 4R. Validate exige que el
// controller reporte una por lens como teeth de cobertura.
const fourRLensCount = 4

// fourRSubagentPlan instruye el fan-out de las 4 lenses. Es TEXTO: el
// fan-out lo ejecuta el cliente con sus subagentes. Cada lens corre una
// vez contra el initial_review_tree, escribe su reporte a archivo
// (file-only) y el controller mergea con toda la autoridad — esta fase
// no bloquea (sdd-review sigue siendo el gate duro).
const fourRSubagentPlan = `Lanzá las 4 lenses del review 4R EN PARALELO, una vez cada una, contra el initial_review_tree. Cada lens es read-only, escribe su reporte a archivo (file-only) y devuelve candidate rows; si está limpia devuelve lista vacía CON evidencia del scope revisado. El controller tiene toda la autoridad: mergea y decide, esta fase no bloquea.
- R1 Risk: seguridad, límites de privilegio, exposición de datos, dependencias, vulnerabilidades merge-blocking.
- R2 Readability: naming, complejidad, intención, mantenibilidad, tamaño del review, claridad de contexto.
- R3 Reliability: cobertura de tests behavior-first, edge cases, determinismo, contratos, regresiones.
- R4 Resilience: fallbacks, retry/backoff, degradación elegante, observabilidad, carga, rollback, riesgos de SLO.
Reportá los 4 resultados en lens_reports (uno por lens), cada uno {lens, findings[], evidence[]}. Un 'clean' exige findings=[] pero evidence NO vacío.`

type sdd4rHandler struct{}

// NewSDD4RHandler crea el handler de la fase sdd-4r (code review 4R).
func NewSDD4RHandler() Handler { return &sdd4rHandler{} }

func (h *sdd4rHandler) Slug() PhaseSlug { return PhaseSlug("sdd-4r") }

func (h *sdd4rHandler) Build(_ context.Context, in Input) (*Output, error) {
	if in.RawText == "" {
		return nil, errors.New("sdd-4r: RawText vacío")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Tarea original del usuario:\n%s\n\n", in.RawText)
	b.WriteString("## initial_review_tree\n")
	if apply, ok := in.PriorOutputs[PhaseSlug("sdd-apply")]; ok {
		if files, ok := apply["files_changed"].([]any); ok && len(files) > 0 {
			b.WriteString("Archivos cambiados (sdd-apply):\n")
			for _, f := range files {
				fmt.Fprintf(&b, "  - %v\n", f)
			}
		}
	}
	if verify, ok := in.PriorOutputs[PhaseSlug("sdd-verify")]; ok {
		if summary, ok := verify["summary"].(string); ok && summary != "" {
			fmt.Fprintf(&b, "Resumen de verificación (sdd-verify):\n%s\n", summary)
		}
	}
	return &Output{
		AgentTemplateSlug: "sdd-4r",
		SystemPrompt:      "",
		UserPrompt:        b.String(),
		SuggestedSaves: []SuggestedSave{
			{Type: "knowledge_doc", Required: false, Hint: "resumen del review 4R si hubo hallazgos severos"},
		},
		SkillThreshold: 0,
		SubagentPlan:   fourRSubagentPlan,
		RetryPolicy:    RetryReemit,
	}, nil
}

// Validate exige cobertura de las 4 lenses (teeth) pero NO bloquea: un
// error inline es recuperable (re-emit), no un sentinela de gate. El
// controller conserva la autoridad de decidir sobre los findings.
func (h *sdd4rHandler) Validate(_ context.Context, _ *Output, result ClientResult) error {
	if result.Output == nil {
		return errors.New("sdd-4r: cliente devolvió Output nulo")
	}
	reports, _ := result.Output["lens_reports"].([]any)
	if len(reports) < fourRLensCount {
		return fmt.Errorf("sdd-4r: 'lens_reports' requiere las %d lenses (R1..R4), recibí %d", fourRLensCount, len(reports))
	}
	return nil
}
