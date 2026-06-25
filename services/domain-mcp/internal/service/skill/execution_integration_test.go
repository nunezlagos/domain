//go:build integration

package skill_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/db"
	"nunezlagos/domain/internal/llm"
	dmigrate "nunezlagos/domain/internal/migrate"
	skillrunner "nunezlagos/domain/internal/runner/skill"
	skillsvc "nunezlagos/domain/internal/service/skill"

	"github.com/google/uuid"
)

func setupExec(t *testing.T) (*skillsvc.ExecutionService, *skillsvc.Service, uuid.UUID, uuid.UUID, func()) {
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
	skillS := &skillsvc.Service{Pool: pools.App, Audit: rec, Embedder: llm.FakeEmbedder{}}
	org, owner, err := seedOrgUser(ctx, pools.App, "ExecOrg", "execorg", "e@x.com", "E")
	require.NoError(t, err)

	exec := &skillsvc.ExecutionService{
		Pool: pools.App, Skills: skillS,
		Versions: &skillsvc.VersionStore{Pool: pools.App},
		Runner:   skillrunner.New(),
	}
	return exec, skillS, org.ID, owner.UserID, func() {
		pools.Close()
		_ = pgC.Terminate(ctx)
	}
}

func TestExecute_Sync_HappyPath(t *testing.T) {
	exec, skillS, orgID, userID, cleanup := setupExec(t)
	defer cleanup()
	ctx := context.Background()

	sk, err := skillS.Create(ctx, skillsvc.CreateInput{
		OrganizationID: orgID, Slug: "greeter", Name: "Greeter",
		SkillType: skillsvc.TypePrompt, Content: "Hola {{name}}",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []any{"name"},
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
		},
		ActorID: userID,
	})
	require.NoError(t, err)

	e, err := exec.Execute(ctx, skillsvc.ExecuteInput{
		OrganizationID: orgID, SkillID: sk.ID,
		Parameters: map[string]any{"name": "Alice"},
	})
	require.NoError(t, err)
	require.Equal(t, "completed", e.Status)
	require.Equal(t, "sync", e.Mode)
	require.NotNil(t, e.Output)
	require.Contains(t, *e.Output, "Alice")
	require.NotNil(t, e.ExecutionTimeMs)
	require.NotNil(t, e.CompletedAt)
}

func TestExecute_InvalidParams_Rejected(t *testing.T) {
	exec, skillS, orgID, userID, cleanup := setupExec(t)
	defer cleanup()
	ctx := context.Background()

	sk, err := skillS.Create(ctx, skillsvc.CreateInput{
		OrganizationID: orgID, Slug: "strict", Name: "Strict",
		SkillType: skillsvc.TypePrompt, Content: "x {{name}}",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []any{"name"},
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
		},
		ActorID: userID,
	})
	require.NoError(t, err)

	_, err = exec.Execute(ctx, skillsvc.ExecuteInput{
		OrganizationID: orgID, SkillID: sk.ID,
		Parameters: map[string]any{}, // falta required "name"
	})
	require.ErrorIs(t, err, skillsvc.ErrInvalidParams)
}

func TestExecute_Async_PollUntilCompleted(t *testing.T) {
	exec, skillS, orgID, userID, cleanup := setupExec(t)
	defer cleanup()
	ctx := context.Background()

	sk, err := skillS.Create(ctx, skillsvc.CreateInput{
		OrganizationID: orgID, Slug: "async-greeter", Name: "Async",
		SkillType: skillsvc.TypePrompt, Content: "ok {{n}}",
		InputSchema: map[string]any{"type": "object"},
		ActorID:     userID,
	})
	require.NoError(t, err)

	e, err := exec.Execute(ctx, skillsvc.ExecuteInput{
		OrganizationID: orgID, SkillID: sk.ID, Mode: "async",
		Parameters: map[string]any{"n": "1"},
	})
	require.NoError(t, err)
	require.Equal(t, "async", e.Mode)
	require.Contains(t, []string{"pending", "running"}, e.Status)

	deadline := time.Now().Add(10 * time.Second)
	for {
		got, err := exec.Get(ctx, orgID, e.ID)
		require.NoError(t, err)
		if got.Status == "completed" {
			require.NotNil(t, got.Output)
			break
		}
		var errMsg string
		if got.Error != nil {
			errMsg = *got.Error
		}
		require.NotEqual(t, "failed", got.Status, "async falló: %s", errMsg)
		require.True(t, time.Now().Before(deadline), "async nunca completó (status=%s)", got.Status)
		time.Sleep(100 * time.Millisecond)
	}
}

func TestExecute_ScrubbedParamsPersisted(t *testing.T) {
	exec, skillS, orgID, userID, cleanup := setupExec(t)
	defer cleanup()
	ctx := context.Background()

	sk, err := skillS.Create(ctx, skillsvc.CreateInput{
		OrganizationID: orgID, Slug: "scrubbed", Name: "Scrubbed",
		SkillType: skillsvc.TypePrompt, Content: "x",
		InputSchema: map[string]any{"type": "object"},
		ActorID:     userID,
	})
	require.NoError(t, err)

	e, err := exec.Execute(ctx, skillsvc.ExecuteInput{
		OrganizationID: orgID, SkillID: sk.ID,
		Parameters: map[string]any{"api_token": "secreto-real", "city": "stgo"},
	})
	require.NoError(t, err)

	got, err := exec.Get(ctx, orgID, e.ID)
	require.NoError(t, err)
	require.Equal(t, "[REDACTED]", got.Parameters["api_token"], "secrets nunca en claro en el log")
	require.Equal(t, "stgo", got.Parameters["city"])
}

func TestExecute_CrossOrg_NotFound(t *testing.T) {
	exec, skillS, orgID, userID, cleanup := setupExec(t)
	defer cleanup()
	ctx := context.Background()

	sk, err := skillS.Create(ctx, skillsvc.CreateInput{
		OrganizationID: orgID, Slug: "mine", Name: "Mine",
		SkillType: skillsvc.TypePrompt, Content: "x",
		InputSchema: map[string]any{"type": "object"},
		ActorID:     userID,
	})
	require.NoError(t, err)

	_, err = exec.Execute(ctx, skillsvc.ExecuteInput{
		OrganizationID: uuid.New(), SkillID: sk.ID,
	})
	require.ErrorIs(t, err, skillsvc.ErrNotFound, "cross-org debe ser not found (anti-enumeration)")


	e, err := exec.Execute(ctx, skillsvc.ExecuteInput{OrganizationID: orgID, SkillID: sk.ID})
	require.NoError(t, err)
	_, err = exec.Get(ctx, uuid.New(), e.ID)
	require.ErrorIs(t, err, skillsvc.ErrExecutionNotFound)
}
