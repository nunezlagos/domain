//go:build integration

package agent_test

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
	"nunezlagos/domain/internal/service/agent"
	orgsvc "nunezlagos/domain/internal/service/org"
	"nunezlagos/domain/internal/service/skill"
)

type fix struct {
	svc    *agent.Service
	skills *skill.Service
	orgID  uuid.UUID
	userID uuid.UUID
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
	orgS := &orgsvc.Service{Pool: pools.App, Audit: rec}
	org, owner, _ := orgS.Create(ctx, "Acme", "acme", "o@x.com", "O")

	svc := &agent.Service{Pool: pools.App, Audit: rec}
	skillSvc := &skill.Service{Pool: pools.App, Audit: rec, Embedder: llm.FakeEmbedder{}}
	return &fix{svc: svc, skills: skillSvc, orgID: org.ID, userID: owner.UserID}, func() {
		pools.Close()
		_ = pgC.Terminate(ctx)
	}
}

func TestAgent_Create(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	a, err := f.svc.Create(ctx, agent.CreateInput{
		OrganizationID: f.orgID,
		Slug:           "code-reviewer",
		Name:           "Code Reviewer",
		Description:    "revisa código y sugiere mejoras",
		Provider:       "anthropic",
		Model:          "claude-sonnet-4-6",
		SystemPrompt:   "Eres un revisor de código senior.",
		ActorID:        f.userID,
	})
	require.NoError(t, err)
	require.Equal(t, "code-reviewer", a.Slug)
	require.Equal(t, "anthropic", a.Provider)
	require.Equal(t, 20, a.MaxIterations)
}

func TestAgent_Create_InvalidProvider(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	_, err := f.svc.Create(context.Background(), agent.CreateInput{
		OrganizationID: f.orgID, Slug: "x", Name: "x", Provider: "fake_provider",
		Model: "x", ActorID: f.userID,
	})
	require.ErrorIs(t, err, agent.ErrProviderInvalid)
}

func TestAgent_Create_WithValidSkills(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	// Crear skill primero
	_, err := f.skills.Create(ctx, skill.CreateInput{
		OrganizationID: f.orgID, Slug: "review-code",
		Name: "review", Description: "rev", SkillType: skill.TypePrompt,
		Content: "x", ActorID: f.userID,
	})
	require.NoError(t, err)

	a, err := f.svc.Create(ctx, agent.CreateInput{
		OrganizationID: f.orgID, Slug: "agent-with-skills",
		Name: "X", Provider: "anthropic", Model: "claude-sonnet-4-6",
		SkillsSlugs: []string{"review-code"}, ActorID: f.userID,
	})
	require.NoError(t, err)
	require.Equal(t, []string{"review-code"}, a.SkillsSlugs)
}

func TestAgent_Create_RejectsUnknownSkills(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	_, err := f.svc.Create(context.Background(), agent.CreateInput{
		OrganizationID: f.orgID, Slug: "x", Name: "x",
		Provider: "anthropic", Model: "claude-sonnet-4-6",
		SkillsSlugs: []string{"no-existe"}, ActorID: f.userID,
	})
	require.ErrorIs(t, err, agent.ErrSkillNotFound)
}

func TestAgent_Create_SlugTaken(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	_, err := f.svc.Create(ctx, agent.CreateInput{
		OrganizationID: f.orgID, Slug: "dup", Name: "X",
		Provider: "anthropic", Model: "claude-sonnet-4-6", ActorID: f.userID,
	})
	require.NoError(t, err)
	_, err = f.svc.Create(ctx, agent.CreateInput{
		OrganizationID: f.orgID, Slug: "dup", Name: "Y",
		Provider: "openai", Model: "gpt-4o-mini", ActorID: f.userID,
	})
	require.ErrorIs(t, err, agent.ErrSlugTaken)
}

func TestAgent_GetBySlug(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	a, _ := f.svc.Create(ctx, agent.CreateInput{
		OrganizationID: f.orgID, Slug: "get", Name: "X",
		Provider: "anthropic", Model: "claude-sonnet-4-6", ActorID: f.userID,
	})
	got, err := f.svc.GetBySlug(ctx, f.orgID, "get")
	require.NoError(t, err)
	require.Equal(t, a.ID, got.ID)
}

func TestAgent_Update_Model(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	a, _ := f.svc.Create(ctx, agent.CreateInput{
		OrganizationID: f.orgID, Slug: "u", Name: "X",
		Provider: "anthropic", Model: "claude-sonnet-4-6", ActorID: f.userID,
	})
	newModel := "claude-opus-4-7"
	upd, err := f.svc.Update(ctx, a.ID, agent.UpdateInput{
		Model: &newModel, ActorID: f.userID,
	})
	require.NoError(t, err)
	require.Equal(t, "claude-opus-4-7", upd.Model)
}

func TestAgent_List(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	_, _ = f.svc.Create(ctx, agent.CreateInput{
		OrganizationID: f.orgID, Slug: "a", Name: "X",
		Provider: "anthropic", Model: "claude-sonnet-4-6", ActorID: f.userID,
	})
	_, _ = f.svc.Create(ctx, agent.CreateInput{
		OrganizationID: f.orgID, Slug: "b", Name: "Y",
		Provider: "openai", Model: "gpt-4o-mini", ActorID: f.userID,
	})
	list, err := f.svc.List(ctx, f.orgID, 10)
	require.NoError(t, err)
	require.Len(t, list, 2)
}

func TestAgent_SoftDelete(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	a, _ := f.svc.Create(ctx, agent.CreateInput{
		OrganizationID: f.orgID, Slug: "del", Name: "X",
		Provider: "anthropic", Model: "claude-sonnet-4-6", ActorID: f.userID,
	})
	require.NoError(t, f.svc.SoftDelete(ctx, a.ID, f.userID))
	_, err := f.svc.GetByID(ctx, a.ID)
	require.ErrorIs(t, err, agent.ErrNotFound)
}

func TestAgent_Create_AutoSlug_CollisionSuffix(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()

	a1, err := f.svc.Create(ctx, agent.CreateInput{
		OrganizationID: f.orgID, Name: "Code Reviewer Pro",
		Provider: "anthropic", Model: "claude-sonnet-4-6", ActorID: f.userID,
	})
	require.NoError(t, err)
	require.Equal(t, "code-reviewer-pro", a1.Slug, "slug derivado del name")

	a2, err := f.svc.Create(ctx, agent.CreateInput{
		OrganizationID: f.orgID, Name: "Code Reviewer Pro",
		Provider: "anthropic", Model: "claude-sonnet-4-6", ActorID: f.userID,
	})
	require.NoError(t, err)
	require.Equal(t, "code-reviewer-pro-2", a2.Slug, "colisión genera sufijo -2")
}

func TestAgent_Create_TemperatureOutOfRange(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	bad := 2.5
	_, err := f.svc.Create(ctx, agent.CreateInput{
		OrganizationID: f.orgID, Slug: "hot", Name: "X",
		Provider: "anthropic", Model: "claude-sonnet-4-6",
		Temperature: &bad, ActorID: f.userID,
	})
	require.ErrorIs(t, err, agent.ErrTemperatureRange)

	neg := -0.1
	_, err = f.svc.Create(ctx, agent.CreateInput{
		OrganizationID: f.orgID, Slug: "cold", Name: "X",
		Provider: "anthropic", Model: "claude-sonnet-4-6",
		Temperature: &neg, ActorID: f.userID,
	})
	require.ErrorIs(t, err, agent.ErrTemperatureRange)
}

func TestAgent_Update_Versioning(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	a, err := f.svc.Create(ctx, agent.CreateInput{
		OrganizationID: f.orgID, Slug: "ver", Name: "X",
		Provider: "anthropic", Model: "claude-sonnet-4-6",
		SystemPrompt: "v1 prompt", ActorID: f.userID,
	})
	require.NoError(t, err)

	for _, sp := range []string{"v2 prompt", "v3 prompt", "v4 prompt"} {
		spc := sp
		_, err := f.svc.Update(ctx, a.ID, agent.UpdateInput{SystemPrompt: &spc, ActorID: f.userID})
		require.NoError(t, err)
	}

	versions, err := f.svc.GetVersions(ctx, a.ID, 0)
	require.NoError(t, err)
	require.Len(t, versions, 3, "3 updates = 3 snapshots")
	require.Equal(t, 3, versions[0].Version, "más reciente primero")
	require.Equal(t, "v3 prompt", versions[0].Snapshot["system_prompt"],
		"snapshot guarda la config PREVIA al update")
	require.Equal(t, "v1 prompt", versions[2].Snapshot["system_prompt"])
}

func TestAgent_Versions_PurgeOver50(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	a, err := f.svc.Create(ctx, agent.CreateInput{
		OrganizationID: f.orgID, Slug: "purge", Name: "X",
		Provider: "anthropic", Model: "claude-sonnet-4-6", ActorID: f.userID,
	})
	require.NoError(t, err)

	// Simular 55 versiones históricas vía SQL (más rápido que 55 updates)
	_, err = f.svc.Pool.Exec(ctx,
		`INSERT INTO agent_versions (agent_id, version, snapshot)
		 SELECT $1, gs, '{}' FROM generate_series(1, 55) gs`, a.ID)
	require.NoError(t, err)

	// Un update real dispara el purge
	name := "Y"
	_, err = f.svc.Update(ctx, a.ID, agent.UpdateInput{Name: &name, ActorID: f.userID})
	require.NoError(t, err)

	var count, minVer, maxVer int
	require.NoError(t, f.svc.Pool.QueryRow(ctx,
		`SELECT COUNT(*), MIN(version), MAX(version) FROM agent_versions WHERE agent_id=$1`,
		a.ID).Scan(&count, &minVer, &maxVer))
	require.Equal(t, 50, count, "purge mantiene exactamente 50")
	require.Equal(t, 56, maxVer, "la versión nueva existe")
	require.Equal(t, 7, minVer, "las más viejas se purgaron")
}

// Sabotaje: modelo inexistente en model_registry → Create falla (excepto ollama).
func TestSabotage_Agent_UnknownModelRejected(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	_, err := f.svc.Create(ctx, agent.CreateInput{
		OrganizationID: f.orgID, Slug: "ghost", Name: "X",
		Provider: "anthropic", Model: "claude-no-existe-99", ActorID: f.userID,
	})
	require.ErrorIs(t, err, agent.ErrModelUnknown)

	// ollama exento: modelos locales arbitrarios permitidos
	_, err = f.svc.Create(ctx, agent.CreateInput{
		OrganizationID: f.orgID, Slug: "local", Name: "X",
		Provider: "ollama", Model: "mi-modelo-local", ActorID: f.userID,
	})
	require.NoError(t, err)
}

// Sabotaje: actualizar agente para asignar skill no existente debe fallar
// (validación de skills al Update también).
func TestSabotage_Agent_UpdateRejectsBadSkill(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	a, _ := f.svc.Create(ctx, agent.CreateInput{
		OrganizationID: f.orgID, Slug: "s", Name: "X",
		Provider: "anthropic", Model: "claude-sonnet-4-6", ActorID: f.userID,
	})
	_, err := f.svc.Update(ctx, a.ID, agent.UpdateInput{
		SkillsSlugs: []string{"fantasma"}, ActorID: f.userID,
	})
	require.ErrorIs(t, err, agent.ErrSkillNotFound)
}
