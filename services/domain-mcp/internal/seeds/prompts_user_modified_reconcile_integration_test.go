//go:build integration

package seeds_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/seeds"
)

func runFirstResponseSeeder(ctx context.Context, t *testing.T, pool *pgxpool.Pool) seeds.Report {
	t.Helper()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	rep, err := (&seeds.FirstResponsePromptSeeder{}).Run(ctx, tx, seeds.EnvProd)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	return rep
}

func promptBody(ctx context.Context, t *testing.T, pool *pgxpool.Pool, slug string) string {
	t.Helper()
	var body string
	err := pool.QueryRow(ctx,
		`SELECT body FROM prompts
		 WHERE slug=$1 AND project_id IS NULL AND is_active AND deleted_at IS NULL
		 ORDER BY version DESC LIMIT 1`, slug).Scan(&body)
	require.NoError(t, err)
	return body
}

// DOMAINSERV-27: el guard AND NOT is_user_modified preserva una fila editada por
// el usuario (Preserved) y reconcilia una no editada (Updated) al re-seedear.
func TestPromptSeeder_Run_UserModifiedPreservado_NoModificadoReconciliado(t *testing.T) {
	pool, cleanup := setupSeededDB(t)
	defer cleanup()
	ctx := context.Background()

	const stale = "STALE body editado por el usuario"

	require.Equal(t, 1, runFirstResponseSeeder(ctx, t, pool).Created, "primer seed crea la fila")

	_, err := pool.Exec(ctx,
		`UPDATE prompts SET body=$1, is_user_modified=true
		 WHERE slug='first-response' AND project_id IS NULL`, stale)
	require.NoError(t, err)

	rep := runFirstResponseSeeder(ctx, t, pool)
	require.Equal(t, 1, rep.Preserved, "fila user_modified NO se pisa")
	require.Equal(t, 0, rep.Updated)
	require.Equal(t, stale, promptBody(ctx, t, pool, "first-response"))

	_, err = pool.Exec(ctx,
		`UPDATE prompts SET is_user_modified=false
		 WHERE slug='first-response' AND project_id IS NULL`)
	require.NoError(t, err)

	rep = runFirstResponseSeeder(ctx, t, pool)
	require.Equal(t, 1, rep.Updated, "fila no user_modified se reconcilia")
	require.Equal(t, 0, rep.Preserved)
	require.Equal(t, strings.TrimSpace(seeds.DefaultFirstResponsePromptBody),
		promptBody(ctx, t, pool, "first-response"))
}
