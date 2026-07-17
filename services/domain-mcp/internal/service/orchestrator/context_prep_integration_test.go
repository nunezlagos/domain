//go:build integration

package orchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/metrics"
	dmigrate "nunezlagos/domain/internal/migrate"
	skillsvc "nunezlagos/domain/internal/service/skill"
)

// DOMAINSERV-38: prepSkills usa SearchHybrid + ApplicableSkillIDs — inyecta las
// skills relevantes y aplicables, EXCLUYE las desactivadas por proyecto, y
// registra result=ok en la métrica de observabilidad (antes silencioso).
func TestPrepSkills_FiltraExcluidaYRegistraOK(t *testing.T) {
	ctx := context.Background()
	pgC, err := postgres.Run(ctx, "pgvector/pgvector:pg16",
		postgres.WithDatabase("test"), postgres.WithUsername("test"), postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2)),
	)
	require.NoError(t, err)
	defer func() { _ = pgC.Terminate(ctx) }()
	dsn, _ := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, dmigrate.Up(dsn))
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	// la tabla organizations fue removida (migración 000143): orgID es un UUID
	// libre; SearchHybrid lo ignora. users/projects ya no tienen organization_id.
	orgID := uuid.New()
	var projectID, userID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, role) VALUES ('o@x.com','owner') RETURNING id`).Scan(&userID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO projects (name, slug) VALUES ('P','p') RETURNING id`).Scan(&projectID))

	skillSvc := &skillsvc.Service{Pool: pool, Audit: &audit.PGRecorder{Pool: pool}, Embedder: llm.FakeEmbedder{}}
	_, err = skillSvc.Create(ctx, skillsvc.CreateInput{
		OrganizationID: orgID, Slug: "included-skill", Name: "Included Skill",
		Description: "skill relevante para la fase apply", SkillType: "prompt", Content: "body", ActorID: userID,
	})
	require.NoError(t, err)
	excl, err := skillSvc.Create(ctx, skillsvc.CreateInput{
		OrganizationID: orgID, Slug: "excluded-skill", Name: "Excluded Skill",
		Description: "skill que el proyecto desactivó", SkillType: "prompt", Content: "body", ActorID: userID,
	})
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO project_skills (project_id, skill_id, is_enabled) VALUES ($1,$2,FALSE)`, projectID, excl.ID)
	require.NoError(t, err)

	s := New(pool, &audit.PGRecorder{Pool: pool}, nil, "test")
	s.Skills = skillSvc
	s.Metrics = metrics.New()

	var b strings.Builder
	s.prepSkills(ctx, orgID, projectID, "sdd-apply", &b)
	out := b.String()

	require.Contains(t, out, "Included Skill", "la skill aplicable debe inyectarse")
	require.NotContains(t, out, "Excluded Skill", "la skill excluida por proyecto NO debe inyectarse")
	c := s.Metrics.OrchestratorContextPrepSectionsTotal.WithLabelValues("skills", "ok")
	require.Equal(t, 1.0, testutil.ToFloat64(c), "debe registrar result=ok")
}
