//go:build integration



package projectlink_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/service/projectlink"
)

func setup(t *testing.T) (*pgxpool.Pool, string, string, func()) {
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
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)

	// org_id es un UUID libre sin respaldo en BD (migraciones 000142/000143).
	orgID := uuid.NewString()

	var projectID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO projects (name, slug, repository_url)
		VALUES ('P', 'p', 'https://github.com/acme/widgets') RETURNING id::text
	`).Scan(&projectID))

	return pool, projectID, orgID, func() {
		pool.Close()
		_ = pgC.Terminate(ctx)
	}
}

func TestProjectLink_LinkByGitRemote_Found(t *testing.T) {
	pool, _, _, cleanup := setup(t)
	defer cleanup()

	svc := projectlink.New(pool)
	id, _, slug, err := svc.LinkByGitRemote(context.Background(), "https://github.com/acme/widgets")
	require.NoError(t, err)
	require.NotEmpty(t, id)
	require.Equal(t, "p", slug)
}

func TestProjectLink_LinkByGitRemote_NotFound(t *testing.T) {
	pool, _, _, cleanup := setup(t)
	defer cleanup()

	svc := projectlink.New(pool)
	id, _, _, err := svc.LinkByGitRemote(context.Background(), "https://github.com/does-not-exist/repo")
	require.NoError(t, err, "LinkByGitRemote no debe fallar cuando no encuentra")
	require.Empty(t, id, "LinkByGitRemote debe devolver '' cuando no encuentra (no auto-create)")
}

func TestProjectLink_LinkByGitRemote_NormalizesGitSuffix(t *testing.T) {
	pool, _, _, cleanup := setup(t)
	defer cleanup()

	svc := projectlink.New(pool)
	id, _, _, err := svc.LinkByGitRemote(context.Background(), "https://github.com/acme/widgets.git")
	require.NoError(t, err)
	require.NotEmpty(t, id, "debe normalizar el .git suffix")
}

func TestProjectLink_UpdateBranch_Persists(t *testing.T) {
	pool, pid, _, cleanup := setup(t)
	defer cleanup()

	svc := projectlink.New(pool)
	require.NoError(t, svc.UpdateBranch(context.Background(), pid, "feature/foo"))

	var branch string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT current_branch FROM projects WHERE id = $1::uuid`, pid).Scan(&branch))
	require.Equal(t, "feature/foo", branch)
}

func TestProjectLink_UpdateBranch_RejectsInvalid(t *testing.T) {
	pool, pid, _, cleanup := setup(t)
	defer cleanup()

	svc := projectlink.New(pool)
	err := svc.UpdateBranch(context.Background(), pid, "evil; DROP TABLE users")
	require.ErrorIs(t, err, projectlink.ErrInvalidBranch)
}

func TestProjectLink_GetRules_EmptyDefault(t *testing.T) {
	pool, pid, _, cleanup := setup(t)
	defer cleanup()

	svc := projectlink.New(pool)
	rules, err := svc.GetRules(context.Background(), pid)
	require.NoError(t, err)
	require.Empty(t, rules, "rules debe ser vacío por default")
}

func TestProjectLink_SetRules_PersistsArray(t *testing.T) {
	pool, pid, _, cleanup := setup(t)
	defer cleanup()

	svc := projectlink.New(pool)
	require.NoError(t, svc.SetRules(context.Background(), pid, []string{"sdd-tdd-strict", "no-co-authored-ia"}))

	got, err := svc.GetRules(context.Background(), pid)
	require.NoError(t, err)
	require.Equal(t, []string{"sdd-tdd-strict", "no-co-authored-ia"}, got)
}
