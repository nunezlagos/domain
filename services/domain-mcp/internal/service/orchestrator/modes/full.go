package modes

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/service/orchestrator/phases"
)

// FullPhases — orden canónico de las 12 fases SDD que el modo Full
// ejecuta (RFC 0006). El catálogo de seeds.SDDPipelinePhaseSlugs vive
// en el package seeds; replicado aquí para evitar el cycle import.
//
// Esta slice es la VERSIÓN DEFAULT. SkipPhases y StartingPhase del
// OrchestrateInput la modifican en BuildFullPlan.
var FullPhases = []phases.PhaseSlug{
	phases.PhaseSlug("sdd-explore"),
	phases.PhaseSlug("sdd-spec"),
	phases.PhaseSlug("sdd-propose"),
	phases.PhaseSlug("sdd-design"),
	phases.PhaseSlug("sdd-tasks"),
	phases.PhaseSlug("sdd-apply"),
	phases.PhaseSlug("sdd-verify"),
	phases.PhaseSlug("sdd-judge"),
	phases.PhaseSlug("sdd-4r"),
	phases.PhaseSlug("sdd-review"),
	phases.PhaseSlug("sdd-archive"),
	phases.PhaseSlug("sdd-onboard"),
}

// BuildFullPlan construye el plan para el modo Full: las 12 fases
// (menos las skipped, opcionalmente arrancando en StartingPhase).
//
// IMPORTANTE — lazy build: a diferencia de Express, sólo el PRIMER step
// del plan recibe UserPrompt hidratado. Los demás llegan con UserPrompt
// vacío. La razón: el prompt de cada fase depende de los outputs
// reales de las fases anteriores (ej: sdd-design necesita
// sdd-spec.issue_md). Construir el prompt al momento de despachar
// produce salidas relevantes.
//
// El service hace el rebuild de los user_prompts vía Service.RecordPhaseResult
// cuando cada step completa, llamando handler.Build con los PriorOutputs
// acumulados de los steps completados hasta ese momento.
func BuildFullPlan(ctx context.Context, reg *phases.Registry, in phases.Input,
	startingPhase phases.PhaseSlug, skipPhases []phases.PhaseSlug, now time.Time,
) (*PhasePlan, error) {
	if reg == nil {
		return nil, errors.New("modes.full: registry nil")
	}
	slugs, err := selectPhases(FullPhases, startingPhase, skipPhases)
	if err != nil {
		return nil, err
	}
	if len(slugs) == 0 {
		return nil, errors.New("modes.full: el filtrado dejó el plan vacío — revisar SkipPhases/StartingPhase")
	}
	plan := &PhasePlan{Mode: "full", StartedAt: now}
	priorOutputs := in.PriorOutputs
	if priorOutputs == nil {
		priorOutputs = map[phases.PhaseSlug]map[string]any{}
	}
	for i, slug := range slugs {
		h, err := reg.Lookup(slug)
		if err != nil {
			return nil, fmt.Errorf("modes.full: lookup %s: %w", slug, err)
		}
		step := PhaseStep{
			ID:                uuid.New(),
			Slug:              slug,
			AgentTemplateSlug: string(slug),
		}

		if i == 0 {
			stepInput := in
			stepInput.PhaseSlug = slug
			stepInput.PriorOutputs = priorOutputs
			out, err := h.Build(ctx, stepInput)
			if err != nil {
				return nil, fmt.Errorf("modes.full: build %s: %w", slug, err)
			}
			step.AgentTemplateSlug = out.AgentTemplateSlug
			step.UserPrompt = out.UserPrompt
			step.SuggestedSaves = out.SuggestedSaves
			step.RetryPolicy = out.RetryPolicy
			step.SkillThreshold = out.SkillThreshold
			step.RequiredToolCalls = out.RequiredToolCalls
			step.OutputSchema = out.OutputSchema
			step.SubagentPlan = out.SubagentPlan
		} else {

			out, err := h.Build(ctx, phases.Input{
				OrganizationID: in.OrganizationID,
				UserID:         in.UserID,
				FlowRunID:      in.FlowRunID,
				PhaseSlug:      slug,
				RawText:        in.RawText,
				PriorOutputs:   map[phases.PhaseSlug]map[string]any{},
			})
			if err != nil {

				step.RetryPolicy = phases.RetryAutoEligible
			} else {
				step.SuggestedSaves = out.SuggestedSaves
				step.RetryPolicy = out.RetryPolicy
				step.SkillThreshold = out.SkillThreshold

			}
		}
		plan.Steps = append(plan.Steps, step)
	}
	return plan, nil
}

// selectPhases aplica StartingPhase + SkipPhases sobre el slice base.
//
// Reglas:
//   - StartingPhase vacío → arranca desde el primer elemento del base
//   - StartingPhase no vacío → debe estar en base; arranca desde ahí
//   - SkipPhases → se omiten esos slugs del resultado
//   - El resultado preserva el orden de base
func selectPhases(base []phases.PhaseSlug, startingPhase phases.PhaseSlug, skipPhases []phases.PhaseSlug) ([]phases.PhaseSlug, error) {
	if err := ValidateDAG(base, skipPhases, startingPhase); err != nil {
		return nil, fmt.Errorf("modes.full: %w", err)
	}
	skip := make(map[phases.PhaseSlug]struct{}, len(skipPhases))
	for _, s := range skipPhases {
		skip[s] = struct{}{}
	}
	startIdx := 0
	if startingPhase != "" {
		startIdx = -1
		for i, slug := range base {
			if slug == startingPhase {
				startIdx = i
				break
			}
		}
		if startIdx < 0 {
			return nil, fmt.Errorf("modes.full: starting_phase %s no está en el catálogo Full", startingPhase)
		}
	}
	out := make([]phases.PhaseSlug, 0, len(base)-startIdx)
	for _, slug := range base[startIdx:] {
		if _, skipped := skip[slug]; skipped {
			continue
		}
		out = append(out, slug)
	}
	return out, nil
}
