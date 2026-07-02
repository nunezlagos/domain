package modes

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/service/orchestrator/phases"
)

// LitePhases es el subconjunto que el modo Lite ejecuta para cambios
// triviales (fix de 1 línea, doc, refactor chico): entender → implementar
// con TDD → validar tests.
//
//	sdd-explore → sdd-apply → sdd-verify
//
// Rationale del set (RFC 0006, camino LITE):
//
//   - sdd-explore: se conserva (a diferencia de Express) porque incluso un
//     cambio trivial necesita ubicar el archivo/función correcta y confirmar
//     que el scope es realmente trivial. Es read-only y barato.
//   - sdd-apply: la implementación real (con TDD).
//   - sdd-verify: corre los tests para validar la implementación.
//
// Se SALTAN deliberadamente las fases pesadas: propose (alternativas +
// tradeoffs), design (diseño técnico), tasks (descomposición), judge
// (review adversarial), archive (cierre formal) y onboard (documentación
// para terceros). Para un cambio trivial el costo de esas fases supera su
// valor.
//
// spec NO se incluye por default: crear un issue/tracking formal para un
// fix de 1 línea es exactamente la ceremonia que Lite busca evitar. Si un
// equipo quiere tracking en Lite, basta con anteponer phases.PhaseSlug
// ("sdd-spec") a este slice (sdd-spec sólo depende de sdd-explore, que ya
// está presente, así que el orden sigue siendo coherente). De ahí que el
// set sea una var exportada y NO una const: es el punto de tuneo.
//
// IMPORTANTE — al igual que ExpressPhases, Lite NO pasa por selectPhases /
// ValidateDAG. Esa validación está pensada para el catálogo Full (donde
// apply depende de tasks→design→propose→spec). Lite es un atajo curado e
// intencional: declara su propia secuencia y la respeta tal cual, sin
// chequeo de dependencias del DAG completo. Cualquier reordenamiento de
// LitePhases es responsabilidad de quien lo edite.
var LitePhases = []phases.PhaseSlug{
	phases.PhaseSlug("sdd-explore"),
	phases.PhaseSlug("sdd-apply"),
	phases.PhaseSlug("sdd-verify"),
}

// BuildLitePlan resuelve cada fase de LitePhases contra el registry, llama
// Build() de cada handler con el contexto compartido, y arma el PhasePlan
// en orden. Espeja BuildExpressPlan: NO ejecuta las fases (eso lo hace el
// cliente IDE) y aborta si alguna falla al construir — Lite tampoco tiene
// puntos de recuperación intra-plan.
//
// A diferencia de Express (apply+verify, ambas hidratan UserPrompt eager),
// Lite incluye explore como primera fase, también hidratada eager: el
// plan completo es chico y los prompts no dependen de outputs previos al
// momento de armarlo (el rebuild lazy de Full no aplica acá — Lite es un
// plan corto que el cliente ejecuta secuencialmente reportando cada fase).
func BuildLitePlan(ctx context.Context, reg *phases.Registry, in phases.Input, now time.Time) (*PhasePlan, error) {
	if reg == nil {
		return nil, fmt.Errorf("modes.lite: registry nil")
	}
	if len(LitePhases) == 0 {
		return nil, fmt.Errorf("modes.lite: LitePhases vacío")
	}
	plan := &PhasePlan{Mode: "lite", StartedAt: now}
	priorOutputs := in.PriorOutputs
	if priorOutputs == nil {
		priorOutputs = map[phases.PhaseSlug]map[string]any{}
	}
	for _, slug := range LitePhases {
		h, err := reg.Lookup(slug)
		if err != nil {
			return nil, fmt.Errorf("modes.lite: lookup %s: %w", slug, err)
		}
		stepInput := in
		stepInput.PhaseSlug = slug
		stepInput.PriorOutputs = priorOutputs
		out, err := h.Build(ctx, stepInput)
		if err != nil {
			return nil, fmt.Errorf("modes.lite: build %s: %w", slug, err)
		}
		plan.Steps = append(plan.Steps, PhaseStep{
			ID:                uuid.New(),
			Slug:              slug,
			AgentTemplateSlug: out.AgentTemplateSlug,
			SystemPrompt:      out.SystemPrompt,
			UserPrompt:        out.UserPrompt,
			SuggestedSaves:    out.SuggestedSaves,
			RetryPolicy:       out.RetryPolicy,
			SkillThreshold:    out.SkillThreshold,
			RequiredToolCalls: out.RequiredToolCalls,
		})
	}
	return plan, nil
}
