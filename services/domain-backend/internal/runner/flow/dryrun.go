package flowrunner

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/api/ctxkeys"
	agentsvc "nunezlagos/domain/internal/service/agent"
	"nunezlagos/domain/internal/service/flow"
)

// PlanStep describe una entrada del dry-run plan.
type PlanStep struct {
	StepID           string   `json:"step_id"`
	Type             string   `json:"type"`
	WillExecute      string   `json:"will_execute"` // "yes" | "no" | "depends_on_runtime"
	Reason           string   `json:"reason,omitempty"`
	EstimatedTokIn   int      `json:"estimated_tokens_in,omitempty"`
	EstimatedTokOut  int      `json:"estimated_tokens_out,omitempty"`
	EstimatedCostUSD float64  `json:"estimated_cost_usd,omitempty"`
	Warnings         []string `json:"warnings,omitempty"`
}

// DryRunResult es el output del plan.
type DryRunResult struct {
	FlowID         uuid.UUID  `json:"flow_id"`
	FlowSlug       string     `json:"flow_slug"`
	Plan           []PlanStep `json:"plan"`
	TotalCostUSD   float64    `json:"total_estimated_cost_usd"`
	TotalTokensIn  int        `json:"total_tokens_in"`
	TotalTokensOut int        `json:"total_tokens_out"`
}

// DryRun analiza estáticamente un flow + inputs y retorna el plan SIN ejecutar steps.
//
// Limitaciones:
//   - Conditional steps cuya expresión depende de output runtime → "depends_on_runtime"
//   - LLM cost estimation usa max_tokens del config o default 1000
//   - Side-effects de http_request marcados como warning
func (r *Runner) DryRun(ctx context.Context, flowID uuid.UUID, inputs map[string]any) (*DryRunResult, error) {
	f, err := r.Flows.GetByID(ctx, flowID)
	if err != nil {
		return nil, fmt.Errorf("flow not found: %w", err)
	}
	out := &DryRunResult{
		FlowID:   f.ID,
		FlowSlug: f.Slug,
		Plan:     make([]PlanStep, 0, len(f.Spec.Steps)),
	}
	for i := range f.Spec.Steps {
		step := &f.Spec.Steps[i]
		ps := analyzeStep(ctx, step, inputs, r.Agents, ctxkeys.OrgID(ctx))
		out.Plan = append(out.Plan, ps)
		out.TotalTokensIn += ps.EstimatedTokIn
		out.TotalTokensOut += ps.EstimatedTokOut
		out.TotalCostUSD += ps.EstimatedCostUSD
	}
	return out, nil
}

// analyzeStep hace análisis estático de un step.
func analyzeStep(ctx context.Context, step *flow.Step, inputs map[string]any,
	agentSvc *agentsvc.Service, orgID uuid.UUID) PlanStep {

	ps := PlanStep{
		StepID:      step.ID,
		Type:        step.Type,
		WillExecute: "yes",
	}
	switch step.Type {
	case flow.StepTypeAgentRun:
		agentSlug, _ := step.Config["agent_slug"].(string)
		input, _ := step.Config["input"].(string)
		// Estimate input tokens: chars/4 (latin) + chars/2 (CJK approximation).
		ps.EstimatedTokIn = estimateTokens(resolveTemplateSimple(input, inputs))
		ps.EstimatedTokOut = 500 // default. Mejor: leer agent.max_tokens cuando exista.
		if agentSvc != nil && agentSlug != "" {
			if ag, err := agentSvc.GetBySlug(ctx, orgID, agentSlug); err == nil {
				if ag.MaxIterations > 0 {
					ps.EstimatedTokOut = 500 * ag.MaxIterations
				}
				ps.Reason = fmt.Sprintf("provider=%s model=%s", ag.Provider, ag.Model)
			} else {
				ps.WillExecute = "no"
				ps.Reason = "agent not found"
			}
		}
		// Cost USD estimación. Para evitar acoplamiento, usamos rate constante
		// promedio: $0.000005/token (mid-tier). Caller con el catálogo de pricing
		// (internal/llm/registry) puede recalcular preciso.
		ps.EstimatedCostUSD = float64(ps.EstimatedTokIn+ps.EstimatedTokOut) * 0.000005

	case flow.StepTypeSkillRun:
		ps.Reason = "skill execution (cost depends on skill type)"
		ps.Warnings = append(ps.Warnings, "side-effects: skill may write to DB or external API")

	case flow.StepTypeMemSave:
		ps.Reason = "mem_save inserts observation"
		ps.Warnings = append(ps.Warnings, "side-effects: persists row + embedding")

	case flow.StepTypeCondition:
		// Análisis estático: si la condición es {{inputs.X}} ==/!= literal,
		// podemos evaluarla con los inputs provistos.
		if isStaticallyEvaluable(step.Config, inputs) {
			ps.Reason = "condition statically evaluable"
		} else {
			ps.WillExecute = "depends_on_runtime"
			ps.Reason = "condition depends on previous step output"
		}

	case flow.StepTypeSubFlow:
		flowSlug, _ := step.Config["flow_slug"].(string)
		ps.Reason = fmt.Sprintf("delegates to flow %s (dry-run not recursive)", flowSlug)
		ps.Warnings = append(ps.Warnings, "nested flow not analyzed (re-run dry-run on child to estimate)")

	case flow.StepTypeParallel:
		branches, _ := step.Config["branches"].([]any)
		ps.Reason = fmt.Sprintf("%d branches in parallel (sum of children)", len(branches))

	case flow.StepTypeHTTPRequest:
		ps.Warnings = append(ps.Warnings, "side-effects: external HTTP call")

	case flow.StepTypeWaitSignal:
		ps.Reason = "waits for external signal (no cost while paused)"

	default:
		ps.WillExecute = "no"
		ps.Reason = "unknown step type"
	}
	return ps
}

// estimateTokens heurística simple: ~1 token cada 4 chars latin.
// Mejor: usar tiktoken-go pero requiere dep.
func estimateTokens(s string) int {
	if s == "" {
		return 0
	}
	return len(s) / 4
}

// resolveTemplateSimple hace una substitución básica de {{inputs.X}} con inputs[X].
// No es exhaustivo — el runner real tiene su propio templater. Esto es
// solo para estimación de tokens.
func resolveTemplateSimple(tpl string, inputs map[string]any) string {
	if !strings.Contains(tpl, "{{") {
		return tpl
	}
	out := tpl
	for k, v := range inputs {
		marker := "{{inputs." + k + "}}"
		out = strings.ReplaceAll(out, marker, fmt.Sprint(v))
	}
	return out
}

// isStaticallyEvaluable retorna true si todos los operandos de la condición
// vienen de inputs (no de outputs.X que es runtime).
func isStaticallyEvaluable(cfg map[string]any, _ map[string]any) bool {
	left, _ := cfg["left"].(string)
	right, _ := cfg["right"].(string)
	return !strings.Contains(left, "{{outputs.") && !strings.Contains(right, "{{outputs.")
}
