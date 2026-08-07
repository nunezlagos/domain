// sdd_compliance.go — issue-56.5. Valida las obligaciones de los marcos normativos que el
// proyecto declaró y puede DETENER el flow.
//
// Va entre sdd-design y sdd-tasks: el design ya declara qué datos toca y qué controles piensa
// implementar —o sea hay sustrato real que evaluar— y todavía no hay tasks ni una línea de código
// escrita, así que corregir es barato. Bloquear después de sdd-apply sería la peor experiencia
// posible, y es una de las razones por las que esto no vive dentro de sdd-4r.
package phases

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const complianceSubagentPlan = `Evalúa las obligaciones de los marcos declarados por el proyecto contra lo que el design dice que se va a construir. NO revises el diff — eso es de R1 en sdd-4r, que recibe tus controles_exigidos por PriorOutputs.
- Trae los marcos y sus controles con domain_compliance_report(project_slug).
- Si el reporte trae aplica=false, cierra con veredicto not_applicable y NADA más: el proyecto no declaró marcos y no hay obligaciones que evaluar.
- Por cada control exigido, decide si el design lo satisface, lo satisface a medias o no lo menciona. Cita el marco y su referencia de artículo cuando exista.
- La severidad NO la eliges tú: sale del catálogo. Marco obligatorio y vigente = BLOCKER; obligatorio pero aún no vigente = WARNING; no obligatorio = SUGGESTION. El reporte te dice cuál es cada uno.
- Un BLOCKER detiene el flow. Si el humano decide seguir igual, necesita un waiver con razón escrita, que queda auditado.`

type sddComplianceHandler struct{}

// NewSDDComplianceHandler crea el handler de la fase sdd-compliance.
func NewSDDComplianceHandler() Handler { return &sddComplianceHandler{} }

func (h *sddComplianceHandler) Slug() PhaseSlug { return PhaseSlug("sdd-compliance") }

func (h *sddComplianceHandler) Build(_ context.Context, in Input) (*Output, error) {
	if in.RawText == "" {
		return nil, errors.New("sdd-compliance: RawText vacío")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Tarea original del usuario:\n%s\n\n", in.RawText)
	b.WriteString("## Qué se va a construir (sdd-design)\n")
	escribirDesign(&b, in)
	b.WriteString("\n## Qué evaluar\n")
	b.WriteString("Las obligaciones de los marcos que ESTE proyecto declaró. Si no declaró " +
		"ninguno, la fase cierra en not_applicable sin evaluar nada.\n")

	return &Output{
		AgentTemplateSlug: "sdd-compliance",
		UserPrompt:        b.String(),
		SuggestedSaves: []SuggestedSave{
			{Type: "knowledge_doc", Required: false,
				Hint: "guardar la decisión si se otorgó un waiver: la razón importa más que el hecho"},
		},
		SubagentPlan:      complianceSubagentPlan,
		RequiredToolCalls: []string{"domain_compliance_report"},
		RetryPolicy:       RetryReemit,
	}, nil
}

func escribirDesign(b *strings.Builder, in Input) {
	design, ok := in.PriorOutputs[PhaseSlug("sdd-design")]
	if !ok {
		b.WriteString("(sin output de sdd-design)\n")
		return
	}
	if md, ok := design["design_md"].(string); ok && md != "" {
		b.WriteString(md + "\n")
		return
	}
	if adrs, ok := design["adrs"].([]any); ok && len(adrs) > 0 {
		fmt.Fprintf(b, "%d ADR(s) declarados en el diseño\n", len(adrs))
	}
}

// Validate aplica el contrato de la fase.
//
// El veredicto not_applicable se acepta SIN hallazgos ni controles: es el no-op de un proyecto que
// no declaró marcos, y exigirle evidencia lo volvería caro justo en el caso que tiene que ser
// gratis. Los demás veredictos sí exigen que la fase declare qué evaluó — un "ok" sin marcos
// evaluados sería una afirmación de cumplimiento sin sustento.
func (h *sddComplianceHandler) Validate(_ context.Context, _ *Output, result ClientResult) error {
	if result.Output == nil {
		return errors.New("sdd-compliance: cliente devolvió Output nulo")
	}
	veredicto, _ := result.Output["veredicto"].(string)
	switch veredicto {
	case "not_applicable":
		return nil
	case "ok", "con_hallazgos", "bloqueado":
	default:
		return fmt.Errorf("sdd-compliance: veredicto %q inválido "+
			"(not_applicable|ok|con_hallazgos|bloqueado)", veredicto)
	}
	if marcos, _ := result.Output["marcos_evaluados"].([]any); len(marcos) == 0 {
		return fmt.Errorf("sdd-compliance: veredicto %q exige marcos_evaluados no vacío — "+
			"sin marcos el veredicto es not_applicable", veredicto)
	}
	return validarHallazgos(result.Output)
}

// validarHallazgos exige que un veredicto bloqueado traiga al menos un BLOCKER sin waiver, y que
// todo hallazgo cite el marco que lo motiva. Un hallazgo sin marco no es accionable: nadie sabría
// contra qué norma corregir.
func validarHallazgos(out map[string]any) error {
	hallazgos, _ := out["hallazgos"].([]any)
	bloqueantesSinWaiver := 0
	for i, h := range hallazgos {
		item, _ := h.(map[string]any)
		if fw, _ := item["framework_slug"].(string); fw == "" {
			return fmt.Errorf("sdd-compliance: hallazgos[%d] sin framework_slug: "+
				"un hallazgo que no cita el marco no es accionable", i)
		}
		if sev, _ := item["severidad"].(string); sev == "BLOCKER" {
			if w, _ := item["waiver_id"].(string); w == "" {
				bloqueantesSinWaiver++
			}
		}
	}
	if v, _ := out["veredicto"].(string); v == "bloqueado" && bloqueantesSinWaiver == 0 {
		return errors.New("sdd-compliance: veredicto 'bloqueado' sin ningún BLOCKER sin waiver")
	}
	return nil
}
