package phases

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// sddReviewHandler implementa la fase sdd-review: el revisor de
// implementación que corre al cierre del ciclo SDD (entre judge y
// archive). A diferencia de sdd-verify (valida escenarios Gherkin) y
// sdd-judge (sabotage tests sobre invariantes), esta fase contrasta la
// solución IMPLEMENTADA contra las políticas y skills aplicables del
// proyecto (resolver jerárquico project → platform).
//
// Es read-only sobre el workspace. Persiste un checkpoint en
// tdd_verifications con kind='policy_review' (un item por policy/skill
// evaluada) vía los tools domain_verify_*. Actúa como GATE: si el
// veredicto es violations_found, Validate bloquea el flow y archive no
// procede hasta que el humano resuelva.
type sddReviewHandler struct{}

func NewSDDReviewHandler() Handler { return &sddReviewHandler{} }

func (h *sddReviewHandler) Slug() PhaseSlug { return PhaseSlug("sdd-review") }

func (h *sddReviewHandler) Build(_ context.Context, in Input) (*Output, error) {
	if in.RawText == "" {
		return nil, errors.New("sdd-review: RawText vacío")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Tarea original del usuario:\n%s\n\n", in.RawText)

	if apply, ok := in.PriorOutputs[PhaseSlug("sdd-apply")]; ok {
		if summary, ok := apply["summary"].(string); ok && summary != "" {
			fmt.Fprintf(&b, "Resumen de lo implementado (sdd-apply):\n%s\n\n", summary)
		}
		if files, ok := apply["files_changed"].([]any); ok && len(files) > 0 {
			fmt.Fprintln(&b, "Archivos modificados a revisar:")
			for _, f := range files {
				fmt.Fprintf(&b, "  - %v\n", f)
			}
			fmt.Fprintln(&b)
		}
	}

	fmt.Fprintln(&b, "Revisá que la solución IMPLEMENTADA cumpla las políticas y skills del proyecto:")
	fmt.Fprintln(&b, "1. Listá las reglas aplicables: domain_project_policy_list(project_slug) +")
	fmt.Fprintln(&b, "   domain_policy_list. Para skills: domain_project_skill_list(project_slug,")
	fmt.Fprintln(&b, "   include_globals=true). El resolver jerárquico (project → platform) ya")
	fmt.Fprintln(&b, "   resuelve override_platform: respetá la regla efectiva.")
	fmt.Fprintln(&b, "2. Abrí un checkpoint: domain_verify_start(project_slug, kind='policy_review',")
	fmt.Fprintln(&b, "   context=<issue>, items=[1 item por policy/skill evaluada, label=slug]).")
	fmt.Fprintln(&b, "3. Contrastá CADA regla contra el diff de los archivos modificados. Reportá")
	fmt.Fprintln(&b, "   cada item con domain_verify_update_item(status=pass|fail|skipped, output=evidencia).")
	fmt.Fprintln(&b, "   NO modifiques código — esta fase es read-only.")
	fmt.Fprintln(&b, "4. Cerrá con domain_verify_complete(verification_id).")
	fmt.Fprintln(&b, "5. Reportá vía domain_orchestrate_phase_result el JSON con verdict + violations.")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "verdict='violations_found' SOLO si hay incumplimientos que bloquean el cierre")
	fmt.Fprintln(&b, "(ej: secret hardcodeado, RLS ausente, N+1, archivo >150 líneas). Nits menores")
	fmt.Fprintln(&b, "van en 'warnings' con verdict='compliant'.")

	return &Output{
		AgentTemplateSlug: "sdd-review",
		SystemPrompt:      "",
		UserPrompt:        b.String(),
		SuggestedSaves: []SuggestedSave{
			{
				Type:     "knowledge_doc",
				Required: false, // el checkpoint vive en tdd_verifications, no en mem_save
				Hint:     "opcional: knowledge_doc con el resumen del review si hubo hallazgos relevantes",
			},
		},
		SkillThreshold: 0,
		// REQ-54 issue-54.6: el prompt de review YA instruía estas tools en prosa
		// (causa raíz de tools huérfanas); ahora son contrato verificable.
		RequiredToolCalls: []string{"domain_project_policy_list", "domain_verify_start", "domain_verify_update_item", "domain_verify_complete"},
		RetryPolicy:    RetryReemit, // read-only, idempotente
	}, nil
}

func (h *sddReviewHandler) Validate(_ context.Context, _ *Output, result ClientResult) error {
	if result.Output == nil {
		return errors.New("sdd-review: cliente devolvió Output nulo")
	}
	verdict, _ := result.Output["verdict"].(string)
	switch verdict {
	case "compliant":
		return nil
	case "violations_found":
		return ErrPolicyReviewFailed
	default:
		return errors.New("sdd-review: campo 'verdict' requerido (compliant | violations_found)")
	}
}

// ErrPolicyReviewFailed señala que el review encontró incumplimientos
// que bloquean el cierre. El service marca el step failed y propaga el
// fallo del flow; el humano debe resolver las violaciones y re-loopear
// apply antes de archivar.
var ErrPolicyReviewFailed = errors.New("review reported policy/skill violations — orchestrator must re-loop apply before archive")
