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
	"nunezlagos/domain/internal/dispatch"
	"nunezlagos/domain/internal/llm"
	dmigrate "nunezlagos/domain/internal/migrate"
	skillrunner "nunezlagos/domain/internal/runner/skill"
	skillsvc "nunezlagos/domain/internal/service/skill"

	"encoding/json"

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

// createdByOf lee directamente skill_executions.created_by para una ejecución.
func createdByOf(t *testing.T, exec *skillsvc.ExecutionService, id uuid.UUID) *uuid.UUID {
	t.Helper()
	var cb *uuid.UUID
	err := exec.Pool.QueryRow(context.Background(),
		`SELECT created_by FROM skill_executions WHERE id = $1`, id,
	).Scan(&cb)
	require.NoError(t, err)
	return cb
}

// TestDispatcher_SkillExecution_PersistsCreatedBy ejercita el ENTRYPOINT REAL
// de producción (el RunFunc del dispatcher que envuelve RunSkillForDispatcher),
// NO un INSERT crudo. Antes este path llamaba SkillRunner.Execute + uuid.New()
// sin insertar fila en skill_executions, por lo que created_by nunca se poblaba
// y unique_callers_count quedaba en 0 (blocker HU-52.2). Este test garantiza que
// el path real persiste la fila CON created_by = TriggeredBy del Request.
func TestDispatcher_SkillExecution_PersistsCreatedBy(t *testing.T) {
	exec, skillS, orgID, userID, cleanup := setupExec(t)
	defer cleanup()
	ctx := context.Background()

	sk, err := skillS.Create(ctx, skillsvc.CreateInput{
		OrganizationID: orgID, Slug: "dispatch-greeter", Name: "DispatchGreeter",
		SkillType: skillsvc.TypePrompt, Content: "Hola {{name}}",
		InputSchema: map[string]any{"type": "object"},
		ActorID:     userID,
	})
	require.NoError(t, err)

	// Construir el adapter REAL (como en cmd/*) y obtener el RunFunc que usa el
	// dispatcher en producción.
	adapters := &dispatch.Adapters{
		SkillRunner: skillrunner.New(),
		Skills:      skillS,
		SkillExec:   exec,
	}
	runSkill := adapters.RunSkillForDispatcher()

	inputs, _ := json.Marshal(map[string]any{"name": "Alice"})
	res, err := runSkill(ctx, dispatch.Request{
		OrgID: orgID, Source: dispatch.SourceMCP, TargetType: dispatch.TargetSkill,
		TargetID: sk.ID, Inputs: inputs, TriggeredBy: &userID,
	})
	require.NoError(t, err)
	require.Equal(t, "completed", res.Status)
	require.NotEqual(t, uuid.Nil, res.RunID, "el RunID debe ser el id real de skill_executions, no uno fabricado")

	// La fila DEBE existir y tener created_by = userID (no NULL).
	got, err := exec.Get(ctx, orgID, res.RunID)
	require.NoError(t, err, "el path real debe haber insertado la fila en skill_executions")
	require.Equal(t, "completed", got.Status)

	cb := createdByOf(t, exec, res.RunID)
	require.NotNil(t, cb, "created_by no debe ser NULL cuando hay TriggeredBy (caller del Principal)")
	require.Equal(t, userID, *cb, "created_by debe ser el caller propagado por el dispatcher")
}

// TestDispatcher_SkillExecution_SystemTrigger_NullCaller: triggers de sistema
// (cron/webhook) no traen user → TriggeredBy nil → created_by NULL persistido.
func TestDispatcher_SkillExecution_SystemTrigger_NullCaller(t *testing.T) {
	exec, skillS, orgID, userID, cleanup := setupExec(t)
	defer cleanup()
	ctx := context.Background()

	sk, err := skillS.Create(ctx, skillsvc.CreateInput{
		OrganizationID: orgID, Slug: "cron-skill", Name: "CronSkill",
		SkillType: skillsvc.TypePrompt, Content: "tick",
		InputSchema: map[string]any{"type": "object"},
		ActorID:     userID,
	})
	require.NoError(t, err)

	adapters := &dispatch.Adapters{
		SkillRunner: skillrunner.New(),
		Skills:      skillS,
		SkillExec:   exec,
	}
	runSkill := adapters.RunSkillForDispatcher()

	res, err := runSkill(ctx, dispatch.Request{
		OrgID: orgID, Source: dispatch.SourceCron, TargetType: dispatch.TargetSkill,
		TargetID: sk.ID, Inputs: nil, TriggeredBy: nil,
	})
	require.NoError(t, err)

	got, err := exec.Get(ctx, orgID, res.RunID)
	require.NoError(t, err)
	require.Equal(t, "completed", got.Status)

	cb := createdByOf(t, exec, res.RunID)
	require.Nil(t, cb, "trigger de sistema sin caller -> created_by NULL")
}
