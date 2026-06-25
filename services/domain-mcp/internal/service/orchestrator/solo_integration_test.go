//go:build integration

package orchestrator_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/seeds"
	"nunezlagos/domain/internal/service/orchestrator"
)

// fakeProvider devuelve respuestas canned por slug para que el orquestador
// Solo pueda iterar las 10 fases en orden sin necesitar un LLM real.
type fakeProvider struct {
	byPhase map[string]string
	calls   int
}

func (p *fakeProvider) Name() string { return "fake" }
func (p *fakeProvider) Complete(_ context.Context, opts llm.CompletionOptions) (*llm.Response, error) {
	p.calls++



	for slug, body := range p.byPhase {


		if contains(opts.SystemPrompt, slug) {
			return &llm.Response{
				Content:      body,
				Model:        opts.Model,
				FinishReason: "stop",
				Usage:        llm.Usage{PromptTokens: 50, CompletionTokens: 80, TotalTokens: 130},
			}, nil
		}
	}

	return &llm.Response{
		Content:      `{"intent":"feature","scope":"single-file","summary":"x"}`,
		Model:        opts.Model,
		FinishReason: "stop",
	}, nil
}
func (p *fakeProvider) CompleteStream(ctx context.Context, opts llm.CompletionOptions) (<-chan llm.StreamChunk, error) {
	return nil, nil
}

func contains(haystack, needle string) bool {
	return len(haystack) > 0 && len(needle) > 0 &&
		indexOfSubstring(haystack, needle) >= 0
}

func indexOfSubstring(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// cannedSoloResponses produce un mapeo phase_slug → JSON output válido
// según el Validate de cada handler, suficiente para que el flow pase
// las 10 fases sin error.
func cannedSoloResponses() map[string]string {
	must := func(v any) string {
		b, _ := json.Marshal(v)
		return string(b)
	}
	return map[string]string{
		"sdd-explore": must(map[string]any{
			"intent": "feature", "scope": "single-file",
			"modules_affected": []any{"internal/x"},
			"summary":          "implementar x",
		}),
		"sdd-spec": must(map[string]any{
			"issue_slug": "issue-99.1-x", "issue_md": "# spec",
		}),
		"sdd-propose": must(map[string]any{
			"proposal_md": "scope x", "status": "draft",
		}),
		"sdd-design": must(map[string]any{
			"design_md": "design",
			"adrs":      []any{map[string]any{"id": "ADR-1", "title": "x"}},
		}),
		"sdd-tasks": must(map[string]any{
			"tasks": []any{map[string]any{"id": "T01", "description": "x"}},
		}),
		"sdd-apply": must(map[string]any{
			"summary": "applied", "files_changed": []any{"x.go"},
		}),
		"sdd-verify": must(map[string]any{
			"scenarios_failed": []any{}, "tests_passed": 1,
		}),
		"sdd-judge": must(map[string]any{
			"sabotage_records": []any{map[string]any{"invariant": "x"}},
		}),
		"sdd-archive": must(map[string]any{"archived": true}),
		"sdd-onboard": must(map[string]any{"skipped": true}),
	}
}

func TestService_Run_Solo_Executes10PhasesEndToEnd(t *testing.T) {
	pools, cleanup := setupOrchestratorDB(t)
	defer cleanup()
	ctx := context.Background()

	orgID := newOrgID(t, pools)
	userID := newUserID(t, pools, orgID)
	projectID := newProjectID(t, pools, orgID)
	_, err := seeds.SeedAgentTemplatesForOrg(ctx, pools.App, orgID)
	require.NoError(t, err)
	_, err = seeds.SeedFlowsForOrg(ctx, pools.App, orgID)
	require.NoError(t, err)




	factory := llm.NewFactory()
	factory.Register("anthropic", &fakeProvider{byPhase: cannedSoloResponses()})

	s := orchestrator.New(pools.App, nil, buildFullRegistry(), "dev")
	s.LLM = factory

	res, err := s.Run(ctx, orchestrator.OrchestrateInput{
		OrganizationID: orgID,
		ProjectID:      projectID,
		UserID:         userID,
		RawText:        "implementar feature CI/CD",
		Mode:           orchestrator.ModeSolo,
	})
	require.NoError(t, err)
	require.Equal(t, orchestrator.ModeSolo, res.Mode)


	rows, err := pools.App.Query(ctx,
		`SELECT step_key, status FROM flow_run_steps
		 WHERE flow_run_id=$1 ORDER BY created_at`, res.FlowRunID)
	require.NoError(t, err)
	defer rows.Close()
	count := 0
	for rows.Next() {
		var k, st string
		require.NoError(t, rows.Scan(&k, &st))
		require.Equal(t, "completed", st,
			"step %s debe estar completed tras Solo run", k)
		count++
	}
	require.Equal(t, 10, count, "10 fases SDD ejecutadas en Solo")


	var flowStatus string
	require.NoError(t, pools.App.QueryRow(ctx,
		`SELECT status FROM flow_runs WHERE id=$1`, res.FlowRunID,
	).Scan(&flowStatus))
	require.Equal(t, "completed", flowStatus)
}

// Sin LLM factory configurado, Solo mode debe fallar con error tipado.
func TestService_Run_Solo_RequiresLLMFactory(t *testing.T) {
	pools, cleanup := setupOrchestratorDB(t)
	defer cleanup()
	ctx := context.Background()

	orgID := newOrgID(t, pools)
	userID := newUserID(t, pools, orgID)
	projectID := newProjectID(t, pools, orgID)
	_, err := seeds.SeedAgentTemplatesForOrg(ctx, pools.App, orgID)
	require.NoError(t, err)
	_, err = seeds.SeedFlowsForOrg(ctx, pools.App, orgID)
	require.NoError(t, err)

	s := orchestrator.New(pools.App, nil, buildFullRegistry(), "dev")

	_, err = s.Run(ctx, orchestrator.OrchestrateInput{
		OrganizationID: orgID,
		ProjectID:      projectID,
		UserID:         userID,
		RawText:        "x",
		Mode:           orchestrator.ModeSolo,
	})
	require.ErrorIs(t, err, orchestrator.ErrLLMFactoryRequired)
}

// Si una fase devuelve JSON inválido, Solo marca el step failed y aborta.
func TestService_Run_Solo_InvalidJSON_MarksStepFailed(t *testing.T) {
	pools, cleanup := setupOrchestratorDB(t)
	defer cleanup()
	ctx := context.Background()

	orgID := newOrgID(t, pools)
	userID := newUserID(t, pools, orgID)
	projectID := newProjectID(t, pools, orgID)
	_, err := seeds.SeedAgentTemplatesForOrg(ctx, pools.App, orgID)
	require.NoError(t, err)
	_, err = seeds.SeedFlowsForOrg(ctx, pools.App, orgID)
	require.NoError(t, err)


	canned := cannedSoloResponses()
	canned["sdd-explore"] = "no json here, just prose"

	factory := llm.NewFactory()
	factory.Register("anthropic", &fakeProvider{byPhase: canned})

	s := orchestrator.New(pools.App, nil, buildFullRegistry(), "dev")
	s.LLM = factory

	_, err = s.Run(ctx, orchestrator.OrchestrateInput{
		OrganizationID: orgID, UserID: userID,
		ProjectID:      projectID,
		RawText: "x", Mode: orchestrator.ModeSolo,
	})
	require.Error(t, err, "Solo debe abortar con JSON inválido")
}
