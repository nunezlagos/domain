//go:build integration

package skill_suggestions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	dmigrate "nunezlagos/domain/internal/migrate"
)

func setupDB(t *testing.T) (*pgxpool.Pool, func()) {
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
	return pool, func() {
		pool.Close()
		_ = pgC.Terminate(ctx)
	}
}

// seedSkill inserta un skill minimo y devuelve su id.
func seedSkill(t *testing.T, pool *pgxpool.Pool, slug, content string, seed bool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	// La migracion 000143 dropea la tabla organizations (CASCADE), pero la columna
	// skills.organization_id sobrevive como NOT NULL sin FK. Single-tenant: un UUID
	// arbitrario satisface el NOT NULL sin necesidad de fila padre.
	orgID := uuid.New()
	var id uuid.UUID
	err := pool.QueryRow(ctx, `
		INSERT INTO skills (organization_id, slug, name, description, skill_type, content, seed_managed)
		VALUES ($1, $2, $3, $4, 'prompt', $5, $6) RETURNING id`,
		orgID, slug, slug, "desc "+slug, content, seed,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func newService(pool *pgxpool.Pool) *Service {
	return &Service{Pool: pool} // Audit nil -> RecordOrLog no-op
}

func TestCreate_DedupPending(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	s := newService(pool)

	in := CreateInput{SkillSlug: "alpha", Kind: KindRefine, Payload: []byte(`{"instruction":"x"}`)}
	first, err := s.Create(ctx, in)
	require.NoError(t, err)
	require.NotNil(t, first)

	// Segunda Create identica mientras la primera sigue pending -> dedup (nil,nil).
	second, err := s.Create(ctx, in)
	require.NoError(t, err)
	require.Nil(t, second, "dedup: no debe crear una segunda pendiente identica")
}

func TestApproveRejectFlow(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	s := newService(pool)

	sug, err := s.Create(ctx, CreateInput{SkillSlug: "beta", Kind: KindArchive, Payload: []byte(`{"reason":"unused"}`)})
	require.NoError(t, err)

	reviewer := uuid.New()
	approved, err := s.Approve(ctx, sug.ID, &reviewer)
	require.NoError(t, err)
	require.Equal(t, StatusApproved, approved.Status)

	// Aprobar de nuevo -> ya no esta pending.
	_, err = s.Approve(ctx, sug.ID, &reviewer)
	require.ErrorIs(t, err, ErrNotPending)

	// Rechazar una ya aprobada -> tampoco esta pending.
	_, err = s.Reject(ctx, sug.ID, &reviewer)
	require.ErrorIs(t, err, ErrNotPending)
}

func TestReject_FromPending(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	s := newService(pool)

	sug, err := s.Create(ctx, CreateInput{SkillSlug: "gamma", Kind: KindRefine, Payload: []byte(`{"instruction":"x"}`)})
	require.NoError(t, err)
	rejected, err := s.Reject(ctx, sug.ID, nil)
	require.NoError(t, err)
	require.Equal(t, StatusRejected, rejected.Status)
}

func TestApply_Archive_SoftDeletesSkill(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	s := newService(pool)

	skillID := seedSkill(t, pool, "to-archive", "content", false)
	sug, _ := s.Create(ctx, CreateInput{SkillSlug: "to-archive", Kind: KindArchive, Payload: []byte(`{"reason":"0 uso"}`)})
	_, err := s.Approve(ctx, sug.ID, nil)
	require.NoError(t, err)

	applied, res, err := s.Apply(ctx, sug.ID, nil)
	require.NoError(t, err)
	require.Equal(t, StatusApplied, applied.Status)
	require.Equal(t, "to-archive", res.ArchivedSlug)
	require.NotNil(t, applied.AppliedAt)

	// El skill quedo soft-deleted.
	var deleted bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM skills WHERE id=$1`, skillID).Scan(&deleted))
	require.True(t, deleted, "archive debe soft-delete el skill")

	// Re-apply -> ya aplicada.
	_, _, err = s.Apply(ctx, sug.ID, nil)
	require.ErrorIs(t, err, ErrAlreadyApplied)
}

func TestApply_Archive_SeedManagedBlocked(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	s := newService(pool)

	seedSkill(t, pool, "seed-skill", "content", true)
	sug, _ := s.Create(ctx, CreateInput{SkillSlug: "seed-skill", Kind: KindArchive, Payload: []byte(`{"reason":"x"}`)})
	_, err := s.Approve(ctx, sug.ID, nil)
	require.NoError(t, err)

	_, _, err = s.Apply(ctx, sug.ID, nil)
	require.ErrorIs(t, err, ErrSeedManaged)

	// Rollback: la sugerencia sigue approved, applied_at NULL.
	after, _ := s.Get(ctx, sug.ID)
	require.Equal(t, StatusApproved, after.Status)
	require.Nil(t, after.AppliedAt)
}

func TestApply_NotApproved(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	s := newService(pool)

	sug, _ := s.Create(ctx, CreateInput{SkillSlug: "delta", Kind: KindArchive, Payload: []byte(`{}`)})
	_, _, err := s.Apply(ctx, sug.ID, nil) // sigue pending
	require.ErrorIs(t, err, ErrNotApproved)
}

func TestApply_Refine_WithPayloadContent(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	s := newService(pool)

	skillID := seedSkill(t, pool, "to-refine", "viejo contenido", false)
	payload := []byte(`{"new_content":"nuevo contenido mejorado","changelog":"mejora"}`)
	sug, _ := s.Create(ctx, CreateInput{SkillSlug: "to-refine", Kind: KindRefine, Payload: payload})
	_, err := s.Approve(ctx, sug.ID, nil)
	require.NoError(t, err)

	_, res, err := s.Apply(ctx, sug.ID, nil)
	require.NoError(t, err)
	require.Equal(t, KindRefine, res.Kind)

	var content string
	require.NoError(t, pool.QueryRow(ctx, `SELECT content FROM skills WHERE id=$1`, skillID).Scan(&content))
	require.Equal(t, "nuevo contenido mejorado", content)
}

func TestApply_Refine_NoContentNoLLM_Degrades(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	s := newService(pool) // Refiner nil

	seedSkill(t, pool, "refine-nollm", "viejo", false)
	sug, _ := s.Create(ctx, CreateInput{SkillSlug: "refine-nollm", Kind: KindRefine, Payload: []byte(`{"instruction":"mejorar"}`)})
	_, err := s.Approve(ctx, sug.ID, nil)
	require.NoError(t, err)

	_, _, err = s.Apply(ctx, sug.ID, nil)
	require.ErrorIs(t, err, ErrApplyUnavailable)

	after, _ := s.Get(ctx, sug.ID)
	require.Equal(t, StatusApproved, after.Status) // rollback
}

func TestApply_Refine_WithFakeRefiner(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	s := newService(pool)
	s.Refiner = fakeRefiner{out: "contenido generado por LLM"}

	skillID := seedSkill(t, pool, "refine-llm", "viejo", false)
	sug, _ := s.Create(ctx, CreateInput{SkillSlug: "refine-llm", Kind: KindRefine, Payload: []byte(`{"instruction":"mejorar"}`)})
	_, err := s.Approve(ctx, sug.ID, nil)
	require.NoError(t, err)

	_, _, err = s.Apply(ctx, sug.ID, nil)
	require.NoError(t, err)

	var content string
	require.NoError(t, pool.QueryRow(ctx, `SELECT content FROM skills WHERE id=$1`, skillID).Scan(&content))
	require.Equal(t, "contenido generado por LLM", content)
}

func TestApply_Split_CreatesChildrenAndSoftDeletesParent(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	s := newService(pool)

	parentID := seedSkill(t, pool, "big-skill", "hace muchas cosas", false)
	payload, _ := json.Marshal(splitPayload{Children: []splitChild{
		{Slug: "child-a", Name: "Child A", Content: "parte A"},
		{Slug: "child-b", Name: "Child B", Content: "parte B"},
	}})
	sug, _ := s.Create(ctx, CreateInput{SkillSlug: "big-skill", Kind: KindSplit, Payload: payload})
	_, err := s.Approve(ctx, sug.ID, nil)
	require.NoError(t, err)

	_, res, err := s.Apply(ctx, sug.ID, nil)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"child-a", "child-b"}, res.CreatedSkills)

	// Hijos creados con parent_skill_id = parentID.
	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM skills WHERE parent_skill_id=$1 AND deleted_at IS NULL`, parentID).Scan(&n))
	require.Equal(t, 2, n)

	// Original soft-deleted.
	var deleted bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM skills WHERE id=$1`, parentID).Scan(&deleted))
	require.True(t, deleted)
}

func TestApply_Merge_ConsolidatesAndSupersedes(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	s := newService(pool)

	aID := seedSkill(t, pool, "merge-a", "contenido a", false)
	bID := seedSkill(t, pool, "merge-b", "contenido b", false)
	payload, _ := json.Marshal(mergePayload{
		With: []string{"merge-b"}, MergedSlug: "merged-ab", MergedName: "Merged AB", MergedContent: "consolidado",
	})
	sug, _ := s.Create(ctx, CreateInput{SkillSlug: "merge-a", Kind: KindMerge, Payload: payload})
	_, err := s.Approve(ctx, sug.ID, nil)
	require.NoError(t, err)

	_, res, err := s.Apply(ctx, sug.ID, nil)
	require.NoError(t, err)
	require.Contains(t, res.CreatedSkills, "merged-ab")
	require.ElementsMatch(t, []string{"merge-a", "merge-b"}, res.SupersededSlugs)

	// Consolidado existe.
	var mergedID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT id FROM skills WHERE slug='merged-ab' AND deleted_at IS NULL`).Scan(&mergedID))

	// Originales superseded_by = consolidado + soft-deleted.
	for _, id := range []uuid.UUID{aID, bID} {
		var supBy *uuid.UUID
		var deleted bool
		require.NoError(t, pool.QueryRow(ctx, `SELECT superseded_by, deleted_at IS NOT NULL FROM skills WHERE id=$1`, id).Scan(&supBy, &deleted))
		require.NotNil(t, supBy)
		require.Equal(t, mergedID, *supBy)
		require.True(t, deleted)
	}
}

func TestApply_Merge_SeedManagedBlocked(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	s := newService(pool)

	// merge-seed-b es seed_managed: MERGE jamas debe superseder un seed.
	seedSkill(t, pool, "merge-seed-a", "contenido a", false)
	seedSkill(t, pool, "merge-seed-b", "contenido b", true)
	payload, _ := json.Marshal(mergePayload{
		With: []string{"merge-seed-b"}, MergedSlug: "merged-seed", MergedName: "Merged", MergedContent: "x",
	})
	sug, _ := s.Create(ctx, CreateInput{SkillSlug: "merge-seed-a", Kind: KindMerge, Payload: payload})
	_, err := s.Approve(ctx, sug.ID, nil)
	require.NoError(t, err)

	_, _, err = s.Apply(ctx, sug.ID, nil)
	require.ErrorIs(t, err, ErrSeedManaged)

	// Rollback: la sugerencia sigue approved, nada quedo borrado/consolidado.
	after, _ := s.Get(ctx, sug.ID)
	require.Equal(t, StatusApproved, after.Status)
	require.Nil(t, after.AppliedAt)

	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM skills WHERE slug='merged-seed'`).Scan(&n))
	require.Equal(t, 0, n, "no debe crear el consolidado si el merge se aborto")
}

func TestApply_ConcurrentDoubleApply(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	s := newService(pool)

	seedSkill(t, pool, "race-skill", "x", false)
	sug, _ := s.Create(ctx, CreateInput{SkillSlug: "race-skill", Kind: KindArchive, Payload: []byte(`{"reason":"x"}`)})
	_, err := s.Approve(ctx, sug.ID, nil)
	require.NoError(t, err)

	_, _, err = s.Apply(ctx, sug.ID, nil)
	require.NoError(t, err)
	// Segundo apply -> guard optimista.
	_, _, err = s.Apply(ctx, sug.ID, nil)
	require.True(t, errors.Is(err, ErrAlreadyApplied), fmt.Sprintf("esperaba ErrAlreadyApplied, got %v", err))
}

func TestGet_NotFound(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()
	s := newService(pool)
	_, err := s.Get(context.Background(), uuid.New())
	require.ErrorIs(t, err, ErrNotFound)
}

func TestList_Filters(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	s := newService(pool)

	_, _ = s.Create(ctx, CreateInput{SkillSlug: "f1", Kind: KindRefine, Payload: []byte(`{}`)})
	_, _ = s.Create(ctx, CreateInput{SkillSlug: "f2", Kind: KindArchive, Payload: []byte(`{}`)})

	all, err := s.List(ctx, ListFilter{})
	require.NoError(t, err)
	require.Len(t, all, 2)

	byKind, err := s.List(ctx, ListFilter{Kind: KindArchive})
	require.NoError(t, err)
	require.Len(t, byKind, 1)
	require.Equal(t, "f2", byKind[0].SkillSlug)

	bySlug, err := s.List(ctx, ListFilter{SkillSlug: "f1"})
	require.NoError(t, err)
	require.Len(t, bySlug, 1)

	pending, err := s.CountPending(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(2), pending)
}
