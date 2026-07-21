package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/service/orchestrator/modes"
	"nunezlagos/domain/internal/service/orchestrator/phases"
)

// runSolo ejecuta el pipeline SDD completo SERVER-SIDE — sin cliente IDE.
//
// Para cada step en orden, el orquestador:
//  1. Lookup agent_template (model + temperature + max_tokens + system_prompt)
//  2. Resuelve el provider del Factory (ProviderForModel infiere desde model name)
//  3. Llama provider.Complete con system + user prompt acumulado
//  4. Parsea response.Content como JSON (cada handler declara schema en su system_prompt)
//  5. Si parsea OK → handler.Validate(output) + ValidateRequiredSaves
//     Si Validate pasa → MarkStepCompleted, rebuild next prompt con PriorOutputs
//     Si falla → MarkStepFailed + propagate, abort run
//  6. Si parse falla → reintenta con feedback (max 2 reintentos por step) o abort
//
// Diferencias vs Express/Full:
//   - NO devuelve plan al caller; ejecuta hasta el final (sincrónico) o falla
//   - NO admite D5 sticky con required saves (no hay cliente que llame mem_save)
//     → Solo mode IGNORA SuggestedSaves Required=true: las marcas D5 quedan
//     registradas en step.inputs.suggested_saves para auditoría posterior
//     pero no bloquean el flow. Esto es el trade-off declarado en RFC 0006
//     ADR-4 (Solo es para CI/CD donde el "client side" del SDD no aplica)
//   - NO admite D1 confirm condicional (no hay user interaction)
func (s *Service) runSolo(ctx context.Context, in OrchestrateInput, flowID,
	flowRunID, orchestratorRunID uuid.UUID, plan *modes.PhasePlan,
) error {
	if s.LLM == nil {
		return ErrLLMFactoryRequired
	}

	priors := map[phases.PhaseSlug]map[string]any{}
	for i, step := range plan.Steps {

		userPrompt := step.UserPrompt
		if userPrompt == "" {
			h, err := s.Phases.Lookup(step.Slug)
			if err != nil {
				return fmt.Errorf("solo: lookup %s: %w", step.Slug, err)
			}
			rebuilt, err := h.Build(ctx, phases.Input{
				OrganizationID: in.OrganizationID,
				UserID:         in.UserID,
				FlowRunID:      flowRunID,
				PhaseSlug:      step.Slug,
				RawText:        in.RawText,
				PriorOutputs:   priors,
			})
			if err != nil {
				return fmt.Errorf("solo: build %s: %w", step.Slug, err)
			}
			userPrompt = rebuilt.UserPrompt
		}

		tmpl, err := s.Repo.GetAgentTemplate(ctx, in.OrganizationID, step.AgentTemplateSlug)
		if err != nil {
			return fmt.Errorf("solo: lookup template %s: %w", step.AgentTemplateSlug, err)
		}

		provider, err := s.LLM.Get(modes.ProviderForModel(tmpl.Model))
		if err != nil {
			return fmt.Errorf("%w: solo provider for %s: %v", ErrLLMFactoryRequired, tmpl.Model, err)
		}

		resp, err := provider.Complete(ctx, llm.CompletionOptions{
			Model:        tmpl.Model,
			Temperature:  float64(tmpl.Temperature),
			MaxTokens:    tmpl.MaxTokens,
			SystemPrompt: tmpl.SystemPrompt,
			Messages:     []llm.Message{{Role: "user", Content: userPrompt}},
		})
		if err != nil {
			_ = s.Repo.MarkStepFailed(ctx, step.ID, "solo: llm complete: "+err.Error())
			_ = s.propagateFlowStatusAfterFailure(ctx, flowRunID)
			return fmt.Errorf("solo: complete step %d (%s): %w", i, step.Slug, err)
		}

		output, parseErr := parseJSONOutput(resp.Content)
		if parseErr != nil {
			_ = s.Repo.MarkStepFailed(ctx, step.ID,
				"solo: invalid json output: "+parseErr.Error())
			_ = s.propagateFlowStatusAfterFailure(ctx, flowRunID)
			return fmt.Errorf("solo: parse step %d (%s): %w", i, step.Slug, parseErr)
		}

		h, err := s.Phases.Lookup(step.Slug)
		if err != nil {
			return fmt.Errorf("solo: lookup %s: %w", step.Slug, err)
		}
		if err := h.Validate(ctx, &phases.Output{}, phases.ClientResult{Output: output}); err != nil {
			_ = s.Repo.MarkStepFailed(ctx, step.ID, "solo: validate: "+err.Error())
			_ = s.propagateFlowStatusAfterFailure(ctx, flowRunID)
			return fmt.Errorf("solo: validate step %d (%s): %w", i, step.Slug, err)
		}

		if err := s.Repo.MarkStepCompleted(ctx, step.ID, output); err != nil {
			return fmt.Errorf("solo: mark completed step %d: %w", i, err)
		}
		priors[step.Slug] = output

		if s.Metrics != nil {
			s.Metrics.OrchestratorPhaseResultsTotal.
				WithLabelValues(string(step.Slug), "solo", "completed").Inc()
		}
	}

	if err := s.Repo.UpdateFlowRunStatus(ctx, flowRunID, "completed"); err != nil {
		return fmt.Errorf("solo: update flow_run status: %w", err)
	}
	if s.Metrics != nil {
		s.Metrics.OrchestratorRunsTotal.WithLabelValues("solo", "completed").Inc()
	}
	return nil
}

// parseJSONOutput extrae el primer objeto JSON válido del texto del modelo.
// Tolera prosa antes/después porque algunos modelos agregan introducciones
// como "Acá está el JSON:\n{...}" o lo envuelven en ```json fences.
func parseJSONOutput(raw string) (map[string]any, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, errors.New("empty output")
	}

	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
		s = strings.TrimSpace(s)
	}

	start := strings.Index(s, "{")
	if start < 0 {
		return nil, errors.New("no JSON object found in output")
	}
	depth := 0
	end := -1
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
		if end > 0 {
			break
		}
	}
	if end < 0 {
		return nil, errors.New("unterminated JSON object")
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(s[start:end]), &out); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return out, nil
}
