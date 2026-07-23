//go:build integration

package projectmerge_test

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
	"nunezlagos/domain/internal/service/projectmerge"
)

func setup(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	ctx := context.Background()
	pgC, err := postgres.Run(ctx,
		"pgvector/pgvector:pg16",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
	)
	require.NoError(t, err)
	dsn, _ := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, dmigrate.Up(dsn))
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	return pool, func() { pool.Close(); _ = pgC.Terminate(ctx) }
}

// TestProjectMerge_MovesAllEntities_AndDedupesSkills cubre el criterio del
// ticket ajustado al schema real (000142 purgó los uniques org-based): mueve
// cada entidad project-scoped, dedupe project_skills por skill_id y
// soft-deletea el source.
func TestProjectMerge_MovesAllEntities_AndDedupesSkills(t *testing.T) {
	pool, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()

	exec := func(sql string, args ...any) {
		_, e := pool.Exec(ctx, sql, args...)
		require.NoError(t, e)
	}
	count := func(sql string, args ...any) int {
		var n int
		require.NoError(t, pool.QueryRow(ctx, sql, args...).Scan(&n))
		return n
	}
	var src, tgt string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO projects (name, slug) VALUES ('S','src') RETURNING id::text`).Scan(&src))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO projects (name, slug) VALUES ('T','tgt') RETURNING id::text`).Scan(&tgt))

	var skillA, skillB string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO skills (slug, name, skill_type) VALUES ('a','A','prompt') RETURNING id::text`).Scan(&skillA))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO skills (slug, name, skill_type) VALUES ('b','B','prompt') RETURNING id::text`).Scan(&skillB))

	// source: una fila de cada entidad
	exec(`INSERT INTO knowledge_observations (project_id, content) VALUES ($1,'obs')`, src)
	exec(`INSERT INTO project_skills (project_id, skill_id) VALUES ($1,$2),($1,$3)`, src, skillA, skillB)
	exec(`INSERT INTO project_policies (project_id, slug, name, kind, body_md) VALUES ($1,'pol','P','convention','x')`, src)
	exec(`INSERT INTO prompts (project_id, slug, body) VALUES ($1,'pr','b')`, src)
	exec(`INSERT INTO project_repositories (project_id, name, url) VALUES ($1,'repo','u')`, src)
	exec(`INSERT INTO knowledge_docs (project_id, title, body) VALUES ($1,'d','b')`, src)
	exec(`INSERT INTO workflows (id, status, project_id) VALUES ($1,'completed',$2)`, uuid.New(), src)

	// target: skillA ya presente → la fila del source se descarta (dedupe)
	exec(`INSERT INTO project_skills (project_id, skill_id) VALUES ($1,$2)`, tgt, skillA)

	svc := &projectmerge.Service{Pool: pool}
	rep, err := svc.Merge(ctx, uuid.MustParse(src), uuid.MustParse(tgt), uuid.Nil)
	require.NoError(t, err)

	require.Equal(t, 1, rep.ObservationsMoved)
	require.Equal(t, 1, rep.SkillsMoved, "solo skillB se mueve")
	require.Equal(t, 1, rep.SkillsDeduped, "skillA ya estaba en target")
	require.Equal(t, 1, rep.PoliciesMoved)
	require.Equal(t, 1, rep.PromptsMoved)
	require.Equal(t, 1, rep.ReposMoved)
	require.Equal(t, 1, rep.DocsMoved)
	require.Equal(t, 1, rep.WorkflowsMoved)

	// target recibió todo; source quedó vacío y soft-deleted
	require.Equal(t, 2, count(`SELECT count(*) FROM project_skills WHERE project_id=$1`, tgt),
		"skillA (ya estaba) + skillB (movido); la fila source de skillA se descartó")
	require.Equal(t, 0, count(`SELECT count(*) FROM project_skills WHERE project_id=$1`, src))
	require.Equal(t, 1, count(`SELECT count(*) FROM knowledge_observations WHERE project_id=$1`, tgt))
	require.Equal(t, 1, count(`SELECT count(*) FROM project_policies WHERE project_id=$1`, tgt))
	require.Equal(t, 1, count(`SELECT count(*) FROM workflows WHERE project_id=$1`, tgt))
	require.Equal(t, 1, count(`SELECT count(*) FROM projects WHERE id=$1 AND deleted_at IS NOT NULL`, src))
	require.Equal(t, 1, count(`SELECT count(*) FROM project_merges WHERE source_id=$1 AND target_id=$2`, src, tgt))
}

// TestProjectMerge_AlreadyMerged rechaza re-fusionar un source ya soft-deleted.
func TestProjectMerge_AlreadyMerged(t *testing.T) {
	pool, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()

	var src, tgt string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO projects (name, slug, deleted_at) VALUES ('S','src', now()) RETURNING id::text`).Scan(&src))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO projects (name, slug) VALUES ('T','tgt') RETURNING id::text`).Scan(&tgt))

	svc := &projectmerge.Service{Pool: pool}
	_, err := svc.Merge(ctx, uuid.MustParse(src), uuid.MustParse(tgt), uuid.Nil)
	require.ErrorIs(t, err, projectmerge.ErrAlreadyMerged)
}
