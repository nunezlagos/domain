//go:build integration

package flowrunner_test

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
	flowrunner "nunezlagos/domain/internal/runner/flow"
	skillrunner "nunezlagos/domain/internal/runner/skill"
	"nunezlagos/domain/internal/service/flow"
	"nunezlagos/domain/internal/service/observation"
	projsvc "nunezlagos/domain/internal/service/project"
	skillsvc "nunezlagos/domain/internal/service/skill"
)

type fix struct {
	runner     *flowrunner.Runner
	flows      *flow.Service
	skills     *skillsvc.Service
	projects   *projsvc.Service
	orgID      uuid.UUID
	projectID  uuid.UUID
	userID     uuid.UUID
}

func setup(t *testing.T) (*fix, func()) {
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
	projS := &projsvc.Service{Pool: pools.App, Audit: rec}
	flowS := &flow.Service{Pool: pools.App, Audit: rec}
	skillS := &skillsvc.Service{Pool: pools.App, Audit: rec, Embedder: llm.FakeEmbedder{}}
	obsS := &observation.Service{Pool: pools.App, Audit: rec, Embedder: llm.FakeEmbedder{}}

	org, owner, _ := seedOrgUser(ctx, pools.App, "Acme", "acme", "o@x.com", "O")
	proj, _ := projS.Create(ctx, projsvc.CreateInput{
		OrganizationID: org.ID, Name: "Demo", Slug: "demo", ActorID: owner.UserID,
	})

	runner := &flowrunner.Runner{
		Pool: pools.App, Audit: rec, Flows: flowS,
		Skills: skillS, Observations: obsS,
		SkillRunner: skillrunner.New(),
	}
	return &fix{
		runner: runner, flows: flowS, skills: skillS, projects: projS,
		orgID: org.ID, projectID: proj.ID, userID: owner.UserID,
	}, func() {
		pools.Close()
		_ = pgC.Terminate(ctx)
	}
}

func TestFlow_BasicSkillRun(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()

	_, err := f.skills.Create(ctx, skillsvc.CreateInput{
		OrganizationID: f.orgID, Slug: "greeting", Name: "Greeting",
		SkillType: skillsvc.TypePrompt, Content: "Hola {{name}}!",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{"name": map[string]any{"type": "string"}},
			"required": []any{"name"},
		},
		ActorID: f.userID,
	})
	require.NoError(t, err)

	fl, err := f.flows.Create(ctx, flow.CreateInput{
		OrganizationID: f.orgID, Slug: "greet-flow", Name: "Greet flow",
		Spec: flow.Spec{
			Version: 1,
			Steps: []flow.Step{
				{ID: "greet", Type: flow.StepTypeSkillRun, Config: map[string]any{
					"skill_slug": "greeting", "args": map[string]any{"name": "Mario"},
				}},
			},
		},
		ActorID: f.userID,
	})
	require.NoError(t, err)

	res, err := f.runner.Run(ctx, flowrunner.RunInput{
		FlowID: fl.ID, TriggeredBy: &f.userID, TriggerType: "manual",
	})
	require.NoError(t, err)
	require.Equal(t, flowrunner.StatusCompleted, res.Status)
	greetOut := res.Outputs["greet"].(map[string]any)
	require.Equal(t, "Hola Mario!", greetOut["result"])
}

func TestFlow_OnErrorContinue(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()

	// Skill que va a fallar (var missing)
	_, _ = f.skills.Create(ctx, skillsvc.CreateInput{
		OrganizationID: f.orgID, Slug: "fail-skill", Name: "Fail",
		SkillType: skillsvc.TypePrompt, Content: "Hola {{missing}}",
		InputSchema: map[string]any{"type": "object"},
		ActorID: f.userID,
	})
	_, _ = f.skills.Create(ctx, skillsvc.CreateInput{
		OrganizationID: f.orgID, Slug: "ok-skill", Name: "OK",
		SkillType: skillsvc.TypePrompt, Content: "tudo bem",
		InputSchema: map[string]any{"type": "object"},
		ActorID: f.userID,
	})

	fl, _ := f.flows.Create(ctx, flow.CreateInput{
		OrganizationID: f.orgID, Slug: "fc", Name: "FC",
		Spec: flow.Spec{
			Version: 1,
			Steps: []flow.Step{
				{ID: "s1", Type: flow.StepTypeSkillRun,
					Config: map[string]any{"skill_slug": "fail-skill", "args": map[string]any{}},
					OnError: "continue"},
				{ID: "s2", Type: flow.StepTypeSkillRun,
					Config: map[string]any{"skill_slug": "ok-skill", "args": map[string]any{}}},
			},
		},
		ActorID: f.userID,
	})

	res, err := f.runner.Run(ctx, flowrunner.RunInput{
		FlowID: fl.ID, TriggeredBy: &f.userID,
	})
	require.NoError(t, err)
	require.Equal(t, flowrunner.StatusCompleted, res.Status,
		"on_error=continue debe avanzar al siguiente step")
	require.Contains(t, res.Outputs, "s1")
	require.Contains(t, res.Outputs, "s2")
}

func TestFlow_OnErrorFailAborts(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()

	_, _ = f.skills.Create(ctx, skillsvc.CreateInput{
		OrganizationID: f.orgID, Slug: "fail", Name: "F",
		SkillType: skillsvc.TypePrompt, Content: "{{missing}}",
		InputSchema: map[string]any{"type": "object"},
		ActorID: f.userID,
	})

	fl, _ := f.flows.Create(ctx, flow.CreateInput{
		OrganizationID: f.orgID, Slug: "fa", Name: "FA",
		Spec: flow.Spec{
			Version: 1, Steps: []flow.Step{
				{ID: "x", Type: flow.StepTypeSkillRun,
					Config: map[string]any{"skill_slug": "fail", "args": map[string]any{}},
					// OnError default = fail
				},
			},
		},
		ActorID: f.userID,
	})
	res, err := f.runner.Run(ctx, flowrunner.RunInput{FlowID: fl.ID})
	require.NoError(t, err)
	require.Equal(t, flowrunner.StatusFailed, res.Status)
	require.Contains(t, res.Error, "step 'x'")
}

func TestFlow_MemSaveStep(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()

	fl, _ := f.flows.Create(ctx, flow.CreateInput{
		OrganizationID: f.orgID, Slug: "ms", Name: "MS",
		Spec: flow.Spec{
			Version: 1, Steps: []flow.Step{
				{ID: "save", Type: flow.StepTypeMemSave, Config: map[string]any{
					"project_id": f.projectID.String(),
					"content":    "registro automático desde flow",
				}},
			},
		},
		ActorID: f.userID,
	})

	res, err := f.runner.Run(ctx, flowrunner.RunInput{
		FlowID: fl.ID, TriggeredBy: &f.userID,
	})
	require.NoError(t, err)
	require.Equal(t, flowrunner.StatusCompleted, res.Status)
	saveOut := res.Outputs["save"].(map[string]any)
	require.NotEmpty(t, saveOut["observation_id"])
}

func TestFlow_HTTPRequestNotImplemented(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()

	fl, _ := f.flows.Create(ctx, flow.CreateInput{
		OrganizationID: f.orgID, Slug: "hni", Name: "HNI",
		Spec: flow.Spec{
			Version: 1, Steps: []flow.Step{
				{ID: "h", Type: flow.StepTypeHTTPRequest, Config: map[string]any{}},
			},
		},
		ActorID: f.userID,
	})
	res, err := f.runner.Run(ctx, flowrunner.RunInput{FlowID: fl.ID})
	require.NoError(t, err)
	require.Equal(t, flowrunner.StatusFailed, res.Status)
	require.Contains(t, res.Error, "not implemented")
}

func TestFlow_InactiveFails(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	fl, _ := f.flows.Create(ctx, flow.CreateInput{
		OrganizationID: f.orgID, Slug: "i", Name: "I",
		Spec: flow.Spec{Version: 1, Steps: []flow.Step{
			{ID: "x", Type: flow.StepTypeMemSave, Config: map[string]any{
				"project_id": f.projectID.String(), "content": "x",
			}},
		}},
		ActorID: f.userID,
	})
	active := false
	_, err := f.flows.Update(ctx, fl.ID, flow.UpdateInput{
		IsActive: &active, ActorID: f.userID,
	})
	require.NoError(t, err)
	_, err = f.runner.Run(ctx, flowrunner.RunInput{FlowID: fl.ID})
	require.ErrorIs(t, err, flowrunner.ErrFlowInactive)
}

func TestFlowSpec_Validate(t *testing.T) {
	// version 0 inválido
	s := flow.Spec{Version: 0, Steps: []flow.Step{{ID: "x", Type: "agent_run"}}}
	require.Error(t, s.Validate())
	// duplicate ids
	s = flow.Spec{Version: 1, Steps: []flow.Step{
		{ID: "x", Type: "agent_run"}, {ID: "x", Type: "skill_run"},
	}}
	require.Error(t, s.Validate())
	// unknown type
	s = flow.Spec{Version: 1, Steps: []flow.Step{{ID: "x", Type: "unknown"}}}
	require.Error(t, s.Validate())
	// happy
	s = flow.Spec{Version: 1, Steps: []flow.Step{{ID: "x", Type: "agent_run"}}}
	require.NoError(t, s.Validate())
}

// Sabotaje: spec con on_error a step_id inexistente debe fail Validate.
func TestSabotage_SpecValidate_OnErrorUnknownStep(t *testing.T) {
	s := flow.Spec{Version: 1, Steps: []flow.Step{
		{ID: "x", Type: "agent_run", OnError: "no-existe"},
	}}
	require.Error(t, s.Validate())
}
