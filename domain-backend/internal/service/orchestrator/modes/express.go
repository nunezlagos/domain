// Package modes implementa los dispatchers de los 5 modos del orquestador
// (RFC 0006). Cada modo decide qué subconjunto de las 10 fases ejecutar
// y en qué orden.
//
// Esta primera versión es in-memory: el dispatcher arma un PhasePlan
// (lista ordenada de prompts y suggested_saves) y se lo entrega al
// caller. La persistencia en flow_runs + flow_run_steps + el bucle
// dispatch ↔ phase_result vendrá en el chunk siguiente, junto con la
// integración MCP.
package modes

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/service/orchestrator/phases"
)

// PhasePlan es el contrato que el dispatcher devuelve al service.
// Cada PhaseStep contiene todo lo que el cliente IDE necesita para
// ejecutar una fase: prompts, suggested_saves, template_slug.
type PhasePlan struct {
	Mode      string
	Steps     []PhaseStep
	StartedAt time.Time
}

// PhaseStep es una fase concreta a despachar. ID es un UUID local del
// plan (no de DB) — sirve para que el cliente correlacione phase_result
// con el step correcto cuando la persistencia llegue, sin cambiar el
// API público.
type PhaseStep struct {
	ID                uuid.UUID
	Slug              phases.PhaseSlug
	AgentTemplateSlug string
	SystemPrompt      string
	UserPrompt        string
	SuggestedSaves    []phases.SuggestedSave
	RetryPolicy       phases.RetryPolicy
	SkillThreshold    float64
}

// ExpressPhases es el subconjunto que el modo Express ejecuta: sólo
// apply + verify (RFC 0006). Diseño cerrado en RFC; cualquier cambio
// requiere ADR nueva — no se hot-patchea acá.
var ExpressPhases = []phases.PhaseSlug{
	phases.PhaseSlug("sdd-apply"),
	phases.PhaseSlug("sdd-verify"),
}

// BuildExpressPlan resuelve cada fase del slice ExpressPhases contra el
// registry, llama Build() de cada handler con el contexto compartido,
// y arma el PhasePlan en orden. Si alguna fase falla al construir, el
// dispatcher aborta — Express no tiene puntos de recuperación intra-plan.
//
// Notar que NO ejecuta las fases — sólo prepara los prompts. La
// ejecución la hace el cliente IDE (RFC 0006 separación state vs exec).
func BuildExpressPlan(ctx context.Context, reg *phases.Registry, in phases.Input, now time.Time) (*PhasePlan, error) {
	if reg == nil {
		return nil, fmt.Errorf("modes.express: registry nil")
	}
	plan := &PhasePlan{Mode: "express", StartedAt: now}
	priorOutputs := in.PriorOutputs
	if priorOutputs == nil {
		priorOutputs = map[phases.PhaseSlug]map[string]any{}
	}
	for _, slug := range ExpressPhases {
		h, err := reg.Lookup(slug)
		if err != nil {
			return nil, fmt.Errorf("modes.express: lookup %s: %w", slug, err)
		}
		stepInput := in
		stepInput.PhaseSlug = slug
		stepInput.PriorOutputs = priorOutputs
		out, err := h.Build(ctx, stepInput)
		if err != nil {
			return nil, fmt.Errorf("modes.express: build %s: %w", slug, err)
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
		})
	}
	return plan, nil
}
