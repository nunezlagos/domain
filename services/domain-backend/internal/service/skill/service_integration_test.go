//go:build integration

package skill_test

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
	"nunezlagos/domain/internal/service/skill"
)

type fix struct {
	svc    *skill.Service
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
	org, owner, _ := seedOrgUser(ctx, pools.App, "Acme", "acme", "o@x.com", "O")

	svc := &skill.Service{Pool: pools.App, Audit: rec, Embedder: llm.FakeEmbedder{}}
	return &fix{svc: svc, orgID: org.ID, userID: owner.UserID}, func() {
		pools.Close()
		_ = pgC.Terminate(ctx)
	}
}

func validSkillInput(orgID, userID uuid.UUID, slug, typ string) skill.CreateInput {
	return skill.CreateInput{
		OrganizationID: orgID,
		Slug:           slug,
		Name:           "Test skill",
		Description:    "skill de prueba que hace cosas",
		SkillType:      typ,
		Content:        "contenido del skill",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"text": map[string]any{"type": "string"}},
			"required":   []any{"text"},
		},
		OutputSchema: map[string]any{
			"type": "string",
		},
		Tags:    []string{"test"},
		ActorID: userID,
	}
}

func TestSkill_Create_Prompt(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	sk, err := f.svc.Create(context.Background(),
		validSkillInput(f.orgID, f.userID, "test-skill", skill.TypePrompt))
	require.NoError(t, err)
	require.Equal(t, skill.TypePrompt, sk.SkillType)
	require.Equal(t, 30, sk.TimeoutSeconds)
}

func TestSkill_Create_Code_API_MCPTool(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	for i, typ := range []string{skill.TypeCode, skill.TypeAPI, skill.TypeMCPTool} {
		slug := []string{"code-skill", "api-skill", "mcp-skill"}[i]
		sk, err := f.svc.Create(ctx, validSkillInput(f.orgID, f.userID, slug, typ))
		require.NoErrorf(t, err, "type=%s", typ)
		require.Equal(t, typ, sk.SkillType)
	}
}

func TestSkill_Create_InvalidType(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	in := validSkillInput(f.orgID, f.userID, "x", "unknown_type")
	_, err := f.svc.Create(context.Background(), in)
	require.ErrorIs(t, err, skill.ErrInvalidType)
}

func TestSkill_Create_InvalidSchema(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	in := validSkillInput(f.orgID, f.userID, "x", skill.TypePrompt)
	in.InputSchema = map[string]any{"type": "invalid-type-xyz"}
	_, err := f.svc.Create(context.Background(), in)
	require.ErrorIs(t, err, skill.ErrInvalidSchema)
}

func TestSkill_Create_SlugTaken(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	_, err := f.svc.Create(ctx, validSkillInput(f.orgID, f.userID, "dup", skill.TypePrompt))
	require.NoError(t, err)
	_, err = f.svc.Create(ctx, validSkillInput(f.orgID, f.userID, "dup", skill.TypeCode))
	require.ErrorIs(t, err, skill.ErrSlugTaken)
}

func TestSkill_GetBySlug(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	sk, _ := f.svc.Create(ctx, validSkillInput(f.orgID, f.userID, "get-test", skill.TypePrompt))
	got, err := f.svc.GetBySlug(ctx, f.orgID, "get-test")
	require.NoError(t, err)
	require.Equal(t, sk.ID, got.ID)
}

func TestSkill_Update_NameTriggersReembed(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	sk, _ := f.svc.Create(ctx, validSkillInput(f.orgID, f.userID, "upd", skill.TypePrompt))
	newName := "Otro nombre completamente distinto"
	updated, err := f.svc.Update(ctx, sk.ID, skill.UpdateInput{
		Name: &newName, ActorID: f.userID,
	})
	require.NoError(t, err)
	require.Equal(t, "Otro nombre completamente distinto", updated.Name)
}

func TestSkill_List_FilterByType(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	_, _ = f.svc.Create(ctx, validSkillInput(f.orgID, f.userID, "p1", skill.TypePrompt))
	_, _ = f.svc.Create(ctx, validSkillInput(f.orgID, f.userID, "p2", skill.TypePrompt))
	_, _ = f.svc.Create(ctx, validSkillInput(f.orgID, f.userID, "c1", skill.TypeCode))

	prompts, err := f.svc.List(ctx, f.orgID, skill.ListFilter{SkillType: skill.TypePrompt})
	require.NoError(t, err)
	require.Len(t, prompts, 2)
	codes, _ := f.svc.List(ctx, f.orgID, skill.ListFilter{SkillType: skill.TypeCode})
	require.Len(t, codes, 1)
}

func TestSkill_SearchHybrid(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	skA := validSkillInput(f.orgID, f.userID, "a", skill.TypePrompt)
	skA.Description = "genera un resumen ejecutivo de textos largos"
	_, _ = f.svc.Create(ctx, skA)
	skB := validSkillInput(f.orgID, f.userID, "b", skill.TypePrompt)
	skB.Description = "envía email transaccional"
	_, _ = f.svc.Create(ctx, skB)

	results, err := f.svc.SearchHybrid(ctx, f.orgID, "resumen ejecutivo", 5)
	require.NoError(t, err)
	require.NotEmpty(t, results)
	require.Contains(t, results[0].Description, "resumen")
}

func TestSkill_SoftDelete(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	sk, _ := f.svc.Create(ctx, validSkillInput(f.orgID, f.userID, "del", skill.TypePrompt))
	require.NoError(t, f.svc.SoftDelete(ctx, sk.ID, f.userID))
	_, err := f.svc.GetByID(ctx, sk.ID)
	require.ErrorIs(t, err, skill.ErrNotFound)
}

func TestSkill_SoftDelete_RejectIfHasDeps(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	parent, _ := f.svc.Create(ctx, validSkillInput(f.orgID, f.userID, "parent", skill.TypePrompt))

	child := validSkillInput(f.orgID, f.userID, "child", skill.TypeCode)
	child.DependsOn = []string{"parent"}
	_, _ = f.svc.Create(ctx, child)

	err := f.svc.SoftDelete(ctx, parent.ID, f.userID)
	require.ErrorIs(t, err, skill.ErrHasDependencies)
}

func TestSkill_ValidateInput_Pass(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	sk, _ := f.svc.Create(ctx, validSkillInput(f.orgID, f.userID, "v", skill.TypePrompt))
	err := f.svc.ValidateInput(ctx, sk.ID, map[string]any{"text": "hola"})
	require.NoError(t, err)
}

func TestSkill_ValidateInput_Reject(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	sk, _ := f.svc.Create(ctx, validSkillInput(f.orgID, f.userID, "v", skill.TypePrompt))
	err := f.svc.ValidateInput(ctx, sk.ID, map[string]any{}) // missing required "text"
	require.Error(t, err)
}

// Sabotaje: tabla CHECK constraint también rechaza tipo inválido si bypassan
// la validación de service.
func TestSabotage_Skill_TypeCheckEnforcedByDB(t *testing.T) {
	f, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	_, err := f.svc.Pool.Exec(ctx,
		`INSERT INTO skills (organization_id, slug, name, skill_type, content)
		 VALUES ($1, 'bypass', 'X', 'fake_type', 'x')`, f.orgID)
	require.Error(t, err, "DB CHECK constraint debe rechazar skill_type fuera del whitelist")
}
