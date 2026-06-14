//go:build integration

package agentrunner_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/db"
	"nunezlagos/domain/internal/llm"
	dmigrate "nunezlagos/domain/internal/migrate"
	agentrunner "nunezlagos/domain/internal/runner/agent"
	agentsvc "nunezlagos/domain/internal/service/agent"
	"nunezlagos/domain/internal/service/billing"
	orgsvc "nunezlagos/domain/internal/service/org"
	"nunezlagos/domain/internal/service/skill"
)

// stubProvider responde con texto fijo o tool calls según script.
type stubProvider struct {
	responses []*llm.Response
	calls     int
}

func (s *stubProvider) Name() string { return "stub" }
func (s *stubProvider) Complete(ctx context.Context, opts llm.CompletionOptions) (*llm.Response, error) {
	if s.calls >= len(s.responses) {
		return s.responses[len(s.responses)-1], nil
	}
	r := s.responses[s.calls]
	s.calls++
	return r, nil
}
func (s *stubProvider) CompleteStream(ctx context.Context, opts llm.CompletionOptions) (<-chan llm.StreamChunk, error) {
	return nil, nil
}

type fix struct {
	runner    *agentrunner.Runner
	agents    *agentsvc.Service
	skills    *skill.Service
	billing   *billing.Service
	orgID     uuid.UUID
	userID    uuid.UUID
	provider  *stubProvider
}

func setup(t *testing.T, responses []*llm.Response) (*fix, func()) {
	t.Helper()
	ctx := context.Background()
	pgC, err := postgres.Run(ctx,
		"pgvector/pgvector:pg16",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
		),
	)
	require.NoError(t, err)
	dsn, _ := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, dmigrate.Up(dsn))

	pools, err := db.OpenWithRoleOverride(ctx, dsn, "app_user", "app_admin")
	require.NoError(t, err)

	rec := &audit.PGRecorder{Pool: pools.Auth}
	orgS := &orgsvc.Service{Pool: pools.App, Audit: rec}
	org, owner, _ := orgS.Create(ctx, "Acme", "acme", "o@x.com", "O")

	skillSvc := &skill.Service{Pool: pools.App, Audit: rec, Embedder: llm.FakeEmbedder{}}
	agentSvc := &agentsvc.Service{Pool: pools.App, Audit: rec}
	billSvc := &billing.Service{Pool: pools.App}
	_ = billSvc.AssignPlan(ctx, org.ID, "enterprise") // unlimited tokens

	provider := &stubProvider{responses: responses}
	factory := llm.NewFactory()
	factory.Register("anthropic", provider)

	runner := &agentrunner.Runner{
		Pool: pools.App, Audit: rec, Factory: factory,
		Agents: agentSvc, Skills: skillSvc, Billing: billSvc,
	}

	return &fix{
			runner: runner, agents: agentSvc, skills: skillSvc, billing: billSvc,
			orgID: org.ID, userID: owner.UserID, provider: provider,
		}, func() {
			pools.Close()
			_ = pgC.Terminate(ctx)
		}
}

func TestRunner_BasicCompletion(t *testing.T) {
	f, cleanup := setup(t, []*llm.Response{
		{Content: "Hola desde el agente!", FinishReason: "stop",
			Usage: llm.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}},
	})
	defer cleanup()
	ctx := context.Background()

	ag, err := f.agents.Create(ctx, agentsvc.CreateInput{
		OrganizationID: f.orgID, Slug: "test-agent", Name: "Test",
		Provider: "anthropic", Model: "claude-sonnet-4-6",
		SystemPrompt: "Eres conciso", ActorID: f.userID,
	})
	require.NoError(t, err)

	res, err := f.runner.Run(ctx, agentrunner.RunInput{
		AgentID: ag.ID, UserID: &f.userID, UserPrompt: "Saluda",
	})
	require.NoError(t, err)
	require.Equal(t, agentrunner.StatusCompleted, res.Status)
	require.Equal(t, "Hola desde el agente!", res.Output)
	require.Equal(t, 15, res.TokensInput+res.TokensOutput)
	require.Equal(t, 1, res.Iterations)

	// Verificar persistencia en agent_runs
	var status string
	var iters int
	require.NoError(t, f.runner.Pool.QueryRow(ctx,
		`SELECT status, iterations FROM agent_runs WHERE id = $1`,
		res.RunID).Scan(&status, &iters))
	require.Equal(t, "completed", status)
	require.Equal(t, 1, iters)
}

func TestRunner_ToolCallLoop(t *testing.T) {
	// Response 1: el agente pide ejecutar el skill "search"
	// Response 2 (después del tool result): el agente termina con respuesta final
	f, cleanup := setup(t, []*llm.Response{
		{Content: "voy a buscar", FinishReason: "tool_use",
			ToolCalls: []llm.ToolCall{
				{ID: "tu_1", Name: "search-greet", Arguments: map[string]any{"name": "Mario"}},
			},
			Usage: llm.Usage{PromptTokens: 20, CompletionTokens: 10, TotalTokens: 30}},
		{Content: "Hola Mario, te saludo desde el skill", FinishReason: "stop",
			Usage: llm.Usage{PromptTokens: 30, CompletionTokens: 15, TotalTokens: 45}},
	})
	defer cleanup()
	ctx := context.Background()

	// Crear skill "search-greet" tipo prompt
	_, err := f.skills.Create(ctx, skill.CreateInput{
		OrganizationID: f.orgID, Slug: "search-greet", Name: "Saludador",
		Description:   "saluda al nombre dado",
		SkillType:     skill.TypePrompt,
		Content:       "Hola {{name}}!",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"name": map[string]any{"type": "string"}},
			"required":   []any{"name"},
		},
		ActorID: f.userID,
	})
	require.NoError(t, err)

	ag, err := f.agents.Create(ctx, agentsvc.CreateInput{
		OrganizationID: f.orgID, Slug: "greeter", Name: "Greeter",
		Provider: "anthropic", Model: "claude-sonnet-4-6",
		SkillsSlugs: []string{"search-greet"},
		ActorID:     f.userID,
	})
	require.NoError(t, err)

	res, err := f.runner.Run(ctx, agentrunner.RunInput{
		AgentID: ag.ID, UserID: &f.userID, UserPrompt: "Saludá a Mario",
	})
	require.NoError(t, err)
	require.Equal(t, agentrunner.StatusCompleted, res.Status)
	require.Equal(t, "Hola Mario, te saludo desde el skill", res.Output)
	require.Equal(t, 2, res.Iterations, "dos calls al LLM (initial + post-tool)")
	require.Equal(t, 50, res.TokensInput, "tokens acumulados ambas calls")
	require.Equal(t, 25, res.TokensOutput)
}

func TestRunner_FailsIfProviderNotRegistered(t *testing.T) {
	f, cleanup := setup(t, nil)
	defer cleanup()
	ctx := context.Background()

	ag, err := f.agents.Create(ctx, agentsvc.CreateInput{
		OrganizationID: f.orgID, Slug: "agent", Name: "X",
		Provider: "openai", // openai NO está registrado en el factory de setup
		Model:    "gpt-4o", ActorID: f.userID,
	})
	require.NoError(t, err)

	res, err := f.runner.Run(ctx, agentrunner.RunInput{
		AgentID: ag.ID, UserID: &f.userID, UserPrompt: "x",
	})
	require.Error(t, err)
	require.NotNil(t, res)
	require.Equal(t, agentrunner.StatusFailed, res.Status)
	require.Contains(t, res.Error, "provider")
}

func TestRunner_MaxIterationsBreak(t *testing.T) {
	// Cada respuesta pide tool_use, nunca termina con stop → debe alcanzar max
	infiniteTool := &llm.Response{
		Content: "más",
		FinishReason: "tool_use",
		ToolCalls: []llm.ToolCall{
			{ID: "tu", Name: "loop", Arguments: map[string]any{}},
		},
		Usage: llm.Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
	}
	f, cleanup := setup(t, []*llm.Response{infiniteTool, infiniteTool, infiniteTool, infiniteTool})
	defer cleanup()
	ctx := context.Background()

	_, _ = f.skills.Create(ctx, skill.CreateInput{
		OrganizationID: f.orgID, Slug: "loop", Name: "loop",
		SkillType: skill.TypePrompt, Content: "ok",
		ActorID: f.userID,
	})
	ag, _ := f.agents.Create(ctx, agentsvc.CreateInput{
		OrganizationID: f.orgID, Slug: "looper", Name: "X",
		Provider: "anthropic", Model: "claude-sonnet-4-6", SkillsSlugs: []string{"loop"},
		MaxIterations: 3, ActorID: f.userID,
	})

	res, err := f.runner.Run(ctx, agentrunner.RunInput{
		AgentID: ag.ID, UserID: &f.userID, UserPrompt: "loop",
	})
	require.NoError(t, err)
	require.Equal(t, agentrunner.StatusFailed, res.Status)
	require.Equal(t, 3, res.Iterations)
	require.Contains(t, res.Error, "max iterations")
}

// Sabotaje: agent_run se persiste como FAILED si el provider falla, no se cuelga
func TestSabotage_Runner_PersistsFailedRun(t *testing.T) {
	f, cleanup := setup(t, nil)
	defer cleanup()
	ctx := context.Background()
	ag, _ := f.agents.Create(ctx, agentsvc.CreateInput{
		OrganizationID: f.orgID, Slug: "x", Name: "X",
		Provider: "openai", Model: "gpt-4o-mini", ActorID: f.userID,
	})
	res, _ := f.runner.Run(ctx, agentrunner.RunInput{
		AgentID: ag.ID, UserPrompt: "x",
	})
	var count int
	require.NoError(t, f.runner.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM agent_runs WHERE id = $1 AND status = 'failed'`,
		res.RunID).Scan(&count))
	require.Equal(t, 1, count, "run debe persistirse aunque haya fallado al inicio")
}
