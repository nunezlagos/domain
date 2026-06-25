// Package agentrunner — issue-08.2 motor de ejecución de agents.
//
// Flow:
//  1. Crear agent_run con status=pending
//  2. Cargar agent + sus skills + system_prompt
//  3. Loop:
//     a. status=running
//     b. Llamar Provider.Complete con messages + tools (skills como function defs)
//     c. Si finish_reason=tool_use → ejecutar skill correspondiente → append result
//     como tool message → iterar
//     d. Si finish_reason=stop → finalizar
//     e. Si iterations > max_iterations → finalizar con error
//  4. Persistir tokens + cost + outputs
//  5. status=completed | failed
//
// Skill execution: actualmente solo skill_type "prompt" se ejecuta sustituyendo
// variables en content. Otros types (code/api/mcp_tool) requieren issue-05.5.
package agentrunner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/api/ctxkeys"
	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/llm"
	budgetmw "nunezlagos/domain/internal/llm/budget"
	"nunezlagos/domain/internal/llm/registry"
	"nunezlagos/domain/internal/llm/tokens"
	"nunezlagos/domain/internal/metrics"
	skillrunner "nunezlagos/domain/internal/runner/skill"
	agentsvc "nunezlagos/domain/internal/service/agent"
	skillsvc "nunezlagos/domain/internal/service/skill"
)

var (
	ErrAgentNotFound   = errors.New("agent not found")
	ErrProviderMissing = errors.New("LLM provider not registered for agent.provider")
	ErrMaxIterations   = errors.New("max iterations reached")



	ErrOrphanRunNotAllowed = errors.New("agent_run without flow_run_id requires WithStandalone in this environment")
)

// Status del agent_run.
const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

// EventEmitter dispara eventos de dominio post-run (issue-10.4 outbound webhooks + issue-15.3 alerts).
type EventEmitter interface {
	EmitAgentRunFinished(ctx context.Context, orgID uuid.UUID, runID uuid.UUID, agentSlug, status string, costUSD float64, tokensTotal int64)
}

// Runner orquesta ejecución de agents.
type Runner struct {
	Pool        *pgxpool.Pool
	Audit       audit.Recorder
	Factory     *llm.Factory
	Agents      *agentsvc.Service
	Skills      *skillsvc.Service
	SkillRunner *skillrunner.Runner // si nil, se crea uno default por Run()
	Models      *registry.Registry  // si nil, costo siempre 0
	Emitter     EventEmitter        // si nil, no emite eventos outbound
	Metrics     *metrics.Registry   // opcional, si nil no genera métricas



	Env string
}

type RunInput struct {
	AgentID    uuid.UUID
	UserID     *uuid.UUID
	UserPrompt string
	Variables  map[string]any
}

type RunResult struct {
	RunID        uuid.UUID
	Status       string
	Output       string
	Error        string
	TokensInput  int
	TokensOutput int
	Iterations   int
	StartedAt    time.Time
	FinishedAt   time.Time
}

// Run ejecuta el agente con el prompt del usuario y devuelve resultado.
// Es síncrono — bloquea hasta finalizar. Streaming versión en issue-08.2.1.
//
// Las opciones variadic permiten al orquestador SDD (issue-08.10) atar el
// run a un flow_run. Sin opciones, el comportamiento es el legacy:
// standalone=true, flow_run_id NULL.
func (r *Runner) Run(ctx context.Context, in RunInput, opts ...RunOption) (*RunResult, error) {
	ro := resolveRunOpts(opts)
	if err := r.checkOrphanPolicy(ro); err != nil {
		return nil, err
	}
	agent, err := r.Agents.GetByID(ctx, in.AgentID)
	if err != nil {
		return nil, fmt.Errorf("get agent: %w", err)
	}



	orgID := ctxkeys.OrgID(ctx)

	provider, err := r.Factory.Get(agent.Provider)
	if err != nil {
		return r.failedRun(ctx, orgID, in, ro, "provider_missing",
			fmt.Errorf("%w: %v", ErrProviderMissing, err))
	}


	tools, skillBySlug, err := r.loadSkillTools(ctx, agent)
	if err != nil {
		return r.failedRun(ctx, orgID, in, ro, "load_skills", err)
	}


	inputsJSON, _ := json.Marshal(map[string]any{
		"user_prompt": in.UserPrompt,
		"variables":   in.Variables,
	})
	var runID uuid.UUID
	now := time.Now().UTC()
	metadataJSON := buildRunMetadata(ro, "direct_invocation")
	err = r.Pool.QueryRow(ctx,
		`INSERT INTO agent_runs (agent_id, user_id, flow_run_id, status, inputs, metadata, started_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id`,
		agent.ID, in.UserID, ro.flowRunID, StatusRunning, inputsJSON, metadataJSON, now,
	).Scan(&runID)
	if err != nil {
		return nil, fmt.Errorf("create run: %w", err)
	}



	if agent.TokenBudget != nil && *agent.TokenBudget > 0 {
		modelMax := 0
		if r.Models != nil {
			if m, merr := r.Models.Get(ctx, agent.Provider, agent.Model); merr == nil && m.ContextSize != nil {
				modelMax = *m.ContextSize
			}
		}
		hard := int(*agent.TokenBudget)
		if modelMax > 0 && hard > modelMax {
			hard = modelMax
		}
		if mgr, berr := tokens.NewTokenBudget(hard*80/100, hard, modelMax, tokens.ModeError); berr == nil {
			mgr.OnSoftLimit = func(used, soft int) {
				r.appendLog(ctx, runID, 0, "warning", map[string]any{
					"stage": "token_budget", "message": "soft limit alcanzado",
					"used": used, "soft": soft,
				}, 0, 0, 0)
			}
			provider = budgetmw.New(provider, mgr)
		}
	}


	messages := []llm.Message{{Role: "user", Content: in.UserPrompt}}
	totalIn, totalOut, iterations := 0, 0, 0
	var finalText string
	var finalErr error

LOOP:
	for iterations < agent.MaxIterations {
		iterations++
		opts := llm.CompletionOptions{
			Model:        agent.Model,
			SystemPrompt: agent.SystemPrompt,
			Messages:     messages,
			Tools:        tools,
		}
		if agent.Temperature != nil {
			opts.Temperature = *agent.Temperature
		}
		if agent.TokenBudget != nil && *agent.TokenBudget > 0 {
			remaining := int(*agent.TokenBudget) - totalIn - totalOut
			if remaining > 0 {
				opts.MaxTokens = remaining
			}
		}




		r.appendPromptSnapshot(ctx, runID, iterations, opts, tools)

		callStart := time.Now()
		resp, err := provider.Complete(ctx, opts)
		latencyMS := int(time.Since(callStart).Milliseconds())
		if err != nil {
			r.appendLog(ctx, runID, iterations, "error",
				map[string]any{"stage": "llm_call", "error": err.Error()},
				0, 0, latencyMS)
			finalErr = fmt.Errorf("complete iter=%d: %w", iterations, err)
			break LOOP
		}
		totalIn += resp.Usage.PromptTokens
		totalOut += resp.Usage.CompletionTokens
		r.appendLog(ctx, runID, iterations, "llm_call", map[string]any{
			"model":         agent.Model,
			"content":       resp.Content,
			"finish_reason": resp.FinishReason,
			"tool_calls":    resp.ToolCalls,
		}, resp.Usage.PromptTokens, resp.Usage.CompletionTokens, latencyMS)


		if len(resp.ToolCalls) == 0 {
			finalText = resp.Content
			break LOOP
		}


		messages = append(messages, llm.Message{
			Role: "assistant", Content: resp.Content, ToolCalls: resp.ToolCalls,
		})
		for _, tc := range resp.ToolCalls {
			toolStart := time.Now()
			r.appendLog(ctx, runID, iterations, "tool_call",
				map[string]any{"tool_name": tc.Name, "arguments": tc.Arguments},
				0, 0, 0)
			result, terr := r.executeTool(ctx, skillBySlug[tc.Name], tc.Arguments)
			toolLatency := int(time.Since(toolStart).Milliseconds())
			if terr != nil {
				result = fmt.Sprintf("tool error: %v", terr)
				r.appendLog(ctx, runID, iterations, "error",
					map[string]any{"stage": "tool_execute", "tool_name": tc.Name, "error": terr.Error()},
					0, 0, toolLatency)
			} else {
				r.appendLog(ctx, runID, iterations, "tool_result",
					map[string]any{"tool_name": tc.Name, "result_preview": truncate(result, 500)},
					0, 0, toolLatency)
			}
			messages = append(messages, llm.Message{
				Role: "tool", ToolCallID: tc.ID, Content: result,
			})
		}
	}
	if iterations >= agent.MaxIterations && finalText == "" && finalErr == nil {
		finalErr = ErrMaxIterations
	}


	status := StatusCompleted
	errStr := ""
	if finalErr != nil {
		status = StatusFailed
		errStr = finalErr.Error()
	}
	finishedAt := time.Now().UTC()
	outputJSON, _ := json.Marshal(map[string]any{"text": finalText})


	var costUSD float64
	if r.Models != nil {
		if c, cerr := r.Models.CostUSD(ctx, agent.Provider, agent.Model, llm.Usage{
			PromptTokens: totalIn, CompletionTokens: totalOut,
		}); cerr == nil {
			costUSD = c
		}
	}

	_, _ = r.Pool.Exec(ctx,
		`UPDATE agent_runs SET status = $1, outputs = $2, error = $3,
		    tokens_input = $4, tokens_output = $5, cost_usd = $6,
		    iterations = $7, finished_at = $8
		 WHERE id = $9`,
		status, outputJSON, nullStr(errStr), totalIn, totalOut, costUSD,
		iterations, finishedAt, runID)


	r.appendLog(ctx, runID, iterations, "final", map[string]any{
		"status":   status,
		"output":   finalText,
		"error":    errStr,
		"cost_usd": costUSD,
	}, totalIn, totalOut, 0)

	if r.Emitter != nil {
		r.Emitter.EmitAgentRunFinished(ctx, orgID, runID, agent.Slug, status, costUSD, int64(totalIn+totalOut))
	}

	if r.Audit != nil {
		_ = r.Audit.Record(ctx, audit.Event{
			OrganizationID: &orgID,
			ActorID:        in.UserID,
			ActorType:      audit.ActorUser,
			Action:         "agent.run_" + status,
			EntityType:     "agent_run",
			EntityID:       &runID,
			NewValues: map[string]any{
				"agent_slug": agent.Slug, "iterations": iterations,
				"tokens_total": totalIn + totalOut,
			},
		})
	}

	if r.Metrics != nil && status == StatusCompleted {
		r.Metrics.LLMTokensTotal.WithLabelValues(agent.Provider, agent.Model, "input").Add(float64(totalIn))
		r.Metrics.LLMTokensTotal.WithLabelValues(agent.Provider, agent.Model, "output").Add(float64(totalOut))
		r.Metrics.CostUSDTotal.WithLabelValues(agent.Provider, agent.Model).Add(costUSD)
	}

	return &RunResult{
		RunID: runID, Status: status, Output: finalText, Error: errStr,
		TokensInput: totalIn, TokensOutput: totalOut, Iterations: iterations,
		StartedAt: now, FinishedAt: finishedAt,
	}, nil
}

// checkOrphanPolicy decide si un run sin flow_run_id está permitido.
//
// Regla (issue-08.10):
//   - Env != "prod"           → permitido siempre (dev/staging libres)
//   - flow_run_id presente     → permitido (no es orphan)
//   - standalone explícito true → permitido (caller declaró intent)
//   - resto                    → ErrOrphanRunNotAllowed
//
// Esta es la barrera proactiva. El cron orphan-runs-audit (issue-08.12)
// es la red de seguridad reactiva que cuenta los que pasaron de todos modos.
func (r *Runner) checkOrphanPolicy(o runOpts) error {
	if r.Env != "prod" {
		return nil
	}
	if o.flowRunID != nil {
		return nil
	}
	if o.standalone {
		return nil
	}
	return ErrOrphanRunNotAllowed
}

// loadSkillTools resuelve los skills asignados a ToolDef LLM-format.
func (r *Runner) loadSkillTools(ctx context.Context, agent *agentsvc.Agent) ([]llm.ToolDef, map[string]*skillsvc.Skill, error) {
	out := make([]llm.ToolDef, 0, len(agent.SkillsSlugs))
	bySlug := make(map[string]*skillsvc.Skill, len(agent.SkillsSlugs))
	for _, slug := range agent.SkillsSlugs {
		sk, err := r.Skills.GetBySlug(ctx, ctxkeys.OrgID(ctx), slug)
		if err != nil {
			return nil, nil, fmt.Errorf("load skill %s: %w", slug, err)
		}
		bySlug[slug] = sk
		out = append(out, llm.ToolDef{
			Name:        sk.Slug,
			Description: sk.Description,
			Schema:      sk.InputSchema,
		})
	}
	return out, bySlug, nil
}

// executeTool delega al SkillRunner (issue-05.5).
func (r *Runner) executeTool(ctx context.Context, sk *skillsvc.Skill, args map[string]any) (string, error) {
	if sk == nil {
		return "", errors.New("skill not loaded")
	}
	if err := r.Skills.ValidateInput(ctx, sk.ID, args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	sr := r.SkillRunner
	if sr == nil {
		sr = skillrunner.New()
	}
	return sr.Execute(ctx, sk, args)
}

// failedRun crea un agent_run con status=failed sin invocar LLM.
func (r *Runner) failedRun(ctx context.Context, orgID uuid.UUID, in RunInput, ro runOpts, reason string, err error) (*RunResult, error) {
	inputsJSON, _ := json.Marshal(map[string]any{
		"user_prompt": in.UserPrompt,
		"variables":   in.Variables,
	})
	now := time.Now().UTC()
	var runID uuid.UUID
	metadataJSON := buildRunMetadata(ro, "direct_invocation_failed")
	dbErr := r.Pool.QueryRow(ctx,
		`INSERT INTO agent_runs (agent_id, user_id, flow_run_id, status, inputs, metadata, error, started_at, finished_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
		 RETURNING id`,
		in.AgentID, in.UserID, ro.flowRunID, StatusFailed, inputsJSON, metadataJSON, err.Error(), now,
	).Scan(&runID)
	if dbErr != nil {
		return nil, fmt.Errorf("create failed run: %w", dbErr)
	}
	return &RunResult{
		RunID: runID, Status: StatusFailed, Error: err.Error(),
		StartedAt: now, FinishedAt: now,
	}, err
}



// appendLog persiste una entry en agent_run_logs (issue-08.3). Best-effort:
// errores de logging NO interrumpen el run principal.
func (r *Runner) appendLog(ctx context.Context, runID uuid.UUID, iteration int,
	eventType string, payload map[string]any, tokensIn, tokensOut, latencyMS int) {
	if r.Pool == nil {
		return
	}
	raw, _ := json.Marshal(payload)
	_, _ = r.Pool.Exec(ctx,
		`INSERT INTO agent_run_logs (agent_run_id, iteration, event_type, payload,
		    tokens_input, tokens_output, latency_ms)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		runID, iteration, eventType, raw, tokensIn, tokensOut, latencyMS)
}

// appendPromptSnapshot persiste el prompt efectivo enviado al LLM en una
// iteracion (issue-42.4: agent_run_prompts). Captura el lado ENTRADA del
// modelo (system_prompt resuelto + messages serializados + tools expuestas),
// distinto de agent_run_logs que registra lo que SALIO del LLM. Best-effort:
// errores de persistencia silenciados, igual que appendLog.
func (r *Runner) appendPromptSnapshot(ctx context.Context, runID uuid.UUID,
	iteration int, opts llm.CompletionOptions, tools []llm.ToolDef) {
	if r.Pool == nil {
		return
	}
	msgsJSON, _ := json.Marshal(opts.Messages)
	slugs := make([]string, 0, len(tools))
	for _, t := range tools {
		slugs = append(slugs, t.Name)
	}
	est := tokens.EstimateMessages(opts.SystemPrompt, opts.Messages)
	cc := len(opts.SystemPrompt)
	for _, m := range opts.Messages {
		cc += len(m.Content)
	}
	_, _ = r.Pool.Exec(ctx,
		`INSERT INTO agent_run_prompts
		   (agent_run_id, iteration, model, system_prompt, messages, tool_slugs, char_count, estimated_tokens)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (agent_run_id, iteration) DO UPDATE
		   SET messages = EXCLUDED.messages,
		       char_count = EXCLUDED.char_count,
		       estimated_tokens = EXCLUDED.estimated_tokens,
		       updated_at = NOW()`,
		runID, iteration, opts.Model, opts.SystemPrompt, msgsJSON,
		slugs, cc, est)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// ensure pgx import (para future uso de tx en runs)
var _ = pgx.ErrNoRows
