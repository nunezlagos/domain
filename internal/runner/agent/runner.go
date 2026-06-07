// Package agentrunner — HU-08.2 motor de ejecución de agents.
//
// Flow:
//   1. Crear agent_run con status=pending
//   2. Cargar agent + sus skills + system_prompt
//   3. Loop:
//      a. status=running
//      b. Llamar Provider.Complete con messages + tools (skills como function defs)
//      c. Si finish_reason=tool_use → ejecutar skill correspondiente → append result
//         como tool message → iterar
//      d. Si finish_reason=stop → finalizar
//      e. Si iterations > max_iterations → finalizar con error
//   4. Persistir tokens + cost + outputs
//   5. status=completed | failed
//
// Skill execution: actualmente solo skill_type "prompt" se ejecuta sustituyendo
// variables en content. Otros types (code/api/mcp_tool) requieren HU-05.5.
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

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/llm"
	agentsvc "nunezlagos/domain/internal/service/agent"
	"nunezlagos/domain/internal/service/billing"
	skillsvc "nunezlagos/domain/internal/service/skill"
)

var (
	ErrAgentNotFound   = errors.New("agent not found")
	ErrProviderMissing = errors.New("LLM provider not registered for agent.provider")
	ErrQuotaExceeded   = errors.New("organization quota exceeded")
	ErrMaxIterations   = errors.New("max iterations reached")
)

// Status del agent_run.
const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

// Runner orquesta ejecución de agents.
type Runner struct {
	Pool     *pgxpool.Pool
	Audit    audit.Recorder
	Factory  *llm.Factory
	Agents   *agentsvc.Service
	Skills   *skillsvc.Service
	Billing  *billing.Service
}

type RunInput struct {
	AgentID    uuid.UUID
	UserID     *uuid.UUID
	UserPrompt string
	Variables  map[string]any
}

type RunResult struct {
	RunID         uuid.UUID
	Status        string
	Output        string
	Error         string
	TokensInput   int
	TokensOutput  int
	Iterations    int
	StartedAt     time.Time
	FinishedAt    time.Time
}

// Run ejecuta el agente con el prompt del usuario y devuelve resultado.
// Es síncrono — bloquea hasta finalizar. Streaming versión en HU-08.2.1.
func (r *Runner) Run(ctx context.Context, in RunInput) (*RunResult, error) {
	agent, err := r.Agents.GetByID(ctx, in.AgentID)
	if err != nil {
		return nil, fmt.Errorf("get agent: %w", err)
	}

	// Pre-flight: quota check
	if r.Billing != nil {
		state, qerr := r.Billing.CheckTokens(ctx, agent.OrganizationID, 0)
		if qerr != nil {
			return r.failedRun(ctx, agent.OrganizationID, in, "quota_exceeded",
				fmt.Errorf("%w: %v", ErrQuotaExceeded, qerr))
		}
		_ = state
	}

	provider, err := r.Factory.Get(agent.Provider)
	if err != nil {
		return r.failedRun(ctx, agent.OrganizationID, in, "provider_missing",
			fmt.Errorf("%w: %v", ErrProviderMissing, err))
	}

	// Cargar skills asignadas como ToolDefs
	tools, skillBySlug, err := r.loadSkillTools(ctx, agent)
	if err != nil {
		return r.failedRun(ctx, agent.OrganizationID, in, "load_skills", err)
	}

	// Crear agent_run con status running
	inputsJSON, _ := json.Marshal(map[string]any{
		"user_prompt": in.UserPrompt,
		"variables":   in.Variables,
	})
	var runID uuid.UUID
	now := time.Now().UTC()
	err = r.Pool.QueryRow(ctx,
		`INSERT INTO agent_runs (organization_id, agent_id, user_id, status, inputs, started_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id`,
		agent.OrganizationID, agent.ID, in.UserID, StatusRunning, inputsJSON, now,
	).Scan(&runID)
	if err != nil {
		return nil, fmt.Errorf("create run: %w", err)
	}

	// Loop completion + tool calls
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

		resp, err := provider.Complete(ctx, opts)
		if err != nil {
			finalErr = fmt.Errorf("complete iter=%d: %w", iterations, err)
			break LOOP
		}
		totalIn += resp.Usage.PromptTokens
		totalOut += resp.Usage.CompletionTokens

		// Si no hay tool calls, terminamos
		if len(resp.ToolCalls) == 0 {
			finalText = resp.Content
			break LOOP
		}

		// Si hay tool calls: append assistant message + ejecutar tools
		messages = append(messages, llm.Message{
			Role: "assistant", Content: resp.Content, ToolCalls: resp.ToolCalls,
		})
		for _, tc := range resp.ToolCalls {
			result, terr := r.executeTool(ctx, skillBySlug[tc.Name], tc.Arguments)
			if terr != nil {
				result = fmt.Sprintf("tool error: %v", terr)
			}
			messages = append(messages, llm.Message{
				Role: "tool", ToolCallID: tc.ID, Content: result,
			})
		}
	}
	if iterations >= agent.MaxIterations && finalText == "" && finalErr == nil {
		finalErr = ErrMaxIterations
	}

	// Persistir resultado + accounting
	status := StatusCompleted
	errStr := ""
	if finalErr != nil {
		status = StatusFailed
		errStr = finalErr.Error()
	}
	finishedAt := time.Now().UTC()
	outputJSON, _ := json.Marshal(map[string]any{"text": finalText})

	_, _ = r.Pool.Exec(ctx,
		`UPDATE agent_runs SET status = $1, outputs = $2, error = $3,
		    tokens_input = $4, tokens_output = $5, iterations = $6, finished_at = $7
		 WHERE id = $8`,
		status, outputJSON, nullStr(errStr), totalIn, totalOut, iterations, finishedAt, runID)

	if r.Billing != nil && totalIn+totalOut > 0 {
		_, _ = r.Billing.IncrementTokens(ctx, agent.OrganizationID, int64(totalIn+totalOut))
		_, _ = r.Billing.IncrementRuns(ctx, agent.OrganizationID)
	}

	if r.Audit != nil {
		_ = r.Audit.Record(ctx, audit.Event{
			OrganizationID: &agent.OrganizationID,
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

	return &RunResult{
		RunID: runID, Status: status, Output: finalText, Error: errStr,
		TokensInput: totalIn, TokensOutput: totalOut, Iterations: iterations,
		StartedAt: now, FinishedAt: finishedAt,
	}, nil
}

// loadSkillTools resuelve los skills asignados a ToolDef LLM-format.
func (r *Runner) loadSkillTools(ctx context.Context, agent *agentsvc.Agent) ([]llm.ToolDef, map[string]*skillsvc.Skill, error) {
	out := make([]llm.ToolDef, 0, len(agent.SkillsSlugs))
	bySlug := make(map[string]*skillsvc.Skill, len(agent.SkillsSlugs))
	for _, slug := range agent.SkillsSlugs {
		sk, err := r.Skills.GetBySlug(ctx, agent.OrganizationID, slug)
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

// executeTool ejecuta un skill. Soporta:
//   - prompt: devuelve content del skill con variables sustituidas (best effort)
//   - code/api/mcp_tool: pending HU-05.5 — por ahora devuelve descripción
func (r *Runner) executeTool(ctx context.Context, sk *skillsvc.Skill, args map[string]any) (string, error) {
	if sk == nil {
		return "", errors.New("skill not loaded")
	}
	// Validate input
	if err := r.Skills.ValidateInput(ctx, sk.ID, args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	switch sk.SkillType {
	case skillsvc.TypePrompt:
		// Sustitución básica {{var}} → value
		return substituteVars(sk.Content, args), nil
	case skillsvc.TypeCode, skillsvc.TypeAPI, skillsvc.TypeMCPTool:
		// Stub: devuelve el contenido del skill como respuesta. HU-05.5 implementará
		// runtime real (sandboxed code exec, HTTP call, MCP forward).
		return fmt.Sprintf("(skill %s tipo %s ejecutado con args: %v) — runtime pending HU-05.5",
			sk.Slug, sk.SkillType, args), nil
	default:
		return "", fmt.Errorf("unknown skill_type: %s", sk.SkillType)
	}
}

// failedRun crea un agent_run con status=failed sin invocar LLM.
func (r *Runner) failedRun(ctx context.Context, orgID uuid.UUID, in RunInput, reason string, err error) (*RunResult, error) {
	inputsJSON, _ := json.Marshal(map[string]any{
		"user_prompt": in.UserPrompt,
		"variables":   in.Variables,
	})
	now := time.Now().UTC()
	var runID uuid.UUID
	dbErr := r.Pool.QueryRow(ctx,
		`INSERT INTO agent_runs (organization_id, agent_id, user_id, status, inputs, error, started_at, finished_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
		 RETURNING id`,
		orgID, in.AgentID, in.UserID, StatusFailed, inputsJSON, err.Error(), now,
	).Scan(&runID)
	if dbErr != nil {
		return nil, fmt.Errorf("create failed run: %w", dbErr)
	}
	return &RunResult{
		RunID: runID, Status: StatusFailed, Error: err.Error(),
		StartedAt: now, FinishedAt: now,
	}, err
}

// substituteVars hace replace simple {{name}} → value en el template.
func substituteVars(tpl string, args map[string]any) string {
	out := tpl
	for k, v := range args {
		placeholder := "{{" + k + "}}"
		out = replaceAll(out, placeholder, fmt.Sprint(v))
	}
	return out
}

func replaceAll(s, old, new string) string {
	// stdlib strings.Replace evitamos para mantener footprint local
	return stringsReplace(s, old, new, -1)
}

// Importamos strings.Replace via wrapper para no agregar otra import.
// (En Go stdlib esto sería trivial; aquí mantengo un re-export controlado.)
var stringsReplace = func() func(s, old, new string, n int) string {
	return _stringsReplace
}()

func _stringsReplace(s, old, new string, n int) string {
	// minimal impl idéntica a strings.Replace pero local.
	if old == "" || s == "" {
		return s
	}
	var b []byte
	i := 0
	for {
		j := indexOf(s[i:], old)
		if j < 0 || n == 0 {
			b = append(b, s[i:]...)
			break
		}
		b = append(b, s[i:i+j]...)
		b = append(b, new...)
		i += j + len(old)
		if n > 0 {
			n--
		}
	}
	return string(b)
}

func indexOf(s, sub string) int {
	if len(sub) == 0 {
		return 0
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// ensure pgx import (para future uso de tx en runs)
var _ = pgx.ErrNoRows
