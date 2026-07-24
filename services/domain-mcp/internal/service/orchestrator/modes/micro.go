package modes

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/service/orchestrator/phases"
)

// MicroPhases es el subconjunto MÍNIMO del pipeline: sólo sdd-apply.
// A diferencia de Express (apply + verify), Micro NO corre sdd-verify —
// está pensado para ediciones triviales sin lógica testeable (texto de
// front, crear un script, doc/config, 1 archivo). El commit-gate del
// cliente exenta estos flows del requisito de tests (marker mode=micro).
var MicroPhases = []phases.PhaseSlug{
	phases.PhaseSlug("sdd-apply"),
}

// BuildMicroPlan arma un PhasePlan con la única fase sdd-apply. Mismo
// contrato que BuildExpressPlan: NO ejecuta la fase, sólo prepara el
// prompt para que el cliente IDE la corra. Sin puntos de recuperación
// intra-plan (una sola fase).
func BuildMicroPlan(ctx context.Context, reg *phases.Registry, in phases.Input, now time.Time) (*PhasePlan, error) {
	if reg == nil {
		return nil, fmt.Errorf("modes.micro: registry nil")
	}
	plan := &PhasePlan{Mode: "micro", StartedAt: now}
	priorOutputs := in.PriorOutputs
	if priorOutputs == nil {
		priorOutputs = map[phases.PhaseSlug]map[string]any{}
	}
	for _, slug := range MicroPhases {
		h, err := reg.Lookup(slug)
		if err != nil {
			return nil, fmt.Errorf("modes.micro: lookup %s: %w", slug, err)
		}
		stepInput := in
		stepInput.PhaseSlug = slug
		stepInput.PriorOutputs = priorOutputs
		out, err := h.Build(ctx, stepInput)
		if err != nil {
			return nil, fmt.Errorf("modes.micro: build %s: %w", slug, err)
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
			OutputSchema:      out.OutputSchema,
			SubagentPlan:      out.SubagentPlan,
		})
	}
	return plan, nil
}
