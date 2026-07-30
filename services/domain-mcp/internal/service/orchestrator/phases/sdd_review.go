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

	fmt.Fprintln(&b, "Revisa que la solución IMPLEMENTADA cumpla las políticas y skills del proyecto:")
	fmt.Fprintln(&b, "1. Lista las reglas aplicables: domain_project_policy_list(project_slug) +")
	fmt.Fprintln(&b, "   domain_policy_list. Para skills: domain_project_skill_list(project_slug,")
	fmt.Fprintln(&b, "   include_globals=true). El resolver jerárquico (project → platform) ya")
	fmt.Fprintln(&b, "   resuelve override_platform: respeta la regla efectiva.")
	fmt.Fprintln(&b, "   Esos listados devuelven slug, name, kind y override_platform, pero NO el")
	fmt.Fprintln(&b, "   cuerpo de la regla (DOMAINSERV-161). Obtené el body_md de CADA slug aplicable")
	fmt.Fprintln(&b, "   con domain_policy_get — en paralelo, un fan-out por slug. Sin ese cuerpo no")
	fmt.Fprintln(&b, "   estás revisando nada: estás leyendo nombres.")
	fmt.Fprintln(&b, "   Para las policies DE PROYECTO hay que pasarle project_slug a domain_policy_get:")
	fmt.Fprintln(&b, "   sin ese argumento solo resuelve las de plataforma y devuelve 'not found'.")
	fmt.Fprintln(&b, "2. Abre un checkpoint: domain_verify_start(project_slug, kind='policy_review',")
	fmt.Fprintln(&b, "   context=<issue>, items=[1 item por policy/skill evaluada, label=slug]).")
	fmt.Fprintln(&b, "3. Contrasta CADA regla contra el diff de los archivos modificados. Reporta")
	fmt.Fprintln(&b, "   cada item con domain_verify_update_item(status=pass|fail|skipped, output=evidencia).")
	fmt.Fprintln(&b, "   NO modifiques código — esta fase es read-only.")
	fmt.Fprintln(&b, "4. Cierra con domain_verify_complete(verification_id).")
	fmt.Fprintln(&b, "5. Reporta vía domain_orchestrate_phase_result el JSON con verdict + violations.")
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
		RequiredToolCalls: []string{"domain_project_policy_list", "domain_policy_get", "domain_verify_start", "domain_verify_update_item", "domain_verify_complete"},
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
		// Un 'compliant' con cero policies evaluadas no es un review: es un sello de goma.
		// Antes de DOMAINSERV-161 esto pasaba, y con los listados proyectados sin body_md
		// habría pasado siempre — el modo de falla es indistinguible del éxito.
		if contadas := contarEnteroDelOutput(result.Output, "policies_checked"); contadas <= 0 {
			return errors.New("sdd-review: verdict 'compliant' con policies_checked en cero o ausente; el review tiene que traer el body_md de cada slug con domain_policy_get y evaluarlo")
		}
		return nil
	case "violations_found":
		return ErrPolicyReviewFailed
	default:
		return errors.New("sdd-review: campo 'verdict' requerido (compliant | violations_found)")
	}
}

// json.Unmarshal entrega los números como float64, pero un output construido en Go llega
// como int: aceptar solo uno de los dos rechazaría reviews legítimos
func contarEnteroDelOutput(out map[string]any, clave string) int {
	switch v := out[clave].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

// ErrPolicyReviewFailed señala que el review encontró incumplimientos
// que bloquean el cierre. El service marca el step failed y propaga el
// fallo del flow; el humano debe resolver las violaciones y re-loopear
// apply antes de archivar.
var ErrPolicyReviewFailed = errors.New("review reported policy/skill violations — orchestrator must re-loop apply before archive")
