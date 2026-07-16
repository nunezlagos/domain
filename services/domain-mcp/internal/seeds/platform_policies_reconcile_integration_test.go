//go:build integration

package seeds_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/agentprotocol"
	"nunezlagos/domain/internal/seeds"
)

const staleBody = "STALE pre-neutralización: usá los tools, detectá el cwd, llamá bootstrap"

func runPlatformPoliciesSeeder(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = (&seeds.PlatformPoliciesSeeder{}).Run(ctx, tx, seeds.EnvProd)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
}

func policyBody(ctx context.Context, t *testing.T, pool *pgxpool.Pool, slug string) string {
	t.Helper()
	var body string
	err := pool.QueryRow(ctx,
		`SELECT body_md FROM platform_policies WHERE slug=$1 AND is_active`, slug,
	).Scan(&body)
	require.NoError(t, err)
	return body
}

func policyUserModified(ctx context.Context, t *testing.T, pool *pgxpool.Pool, slug string) bool {
	t.Helper()
	var flag bool
	err := pool.QueryRow(ctx,
		`SELECT is_user_modified FROM platform_policies WHERE slug=$1 AND is_active`, slug,
	).Scan(&flag)
	require.NoError(t, err)
	return flag
}

// DOMAINSERV-34: la migración 000270 resetea is_user_modified SOLO en los slugs
// listados; el re-seed reaplica su body del fuente. Una fila user_modified NO
// listada (sdd-auto-trigger) se preserva intacta.
func TestPlatformPolicies_Reconcile_ResetSlugsReaplican_NoListadosPreservados(t *testing.T) {
	pool, cleanup := setupSeededDB(t)
	defer cleanup()
	ctx := context.Background()

	runPlatformPoliciesSeeder(ctx, t, pool)

	_, err := pool.Exec(ctx,
		`UPDATE platform_policies SET body_md=$1, is_user_modified=true
		 WHERE slug IN ('agent-protocol','agent-voice','sdd-auto-trigger')`, staleBody)
	require.NoError(t, err)

	mig, err := os.ReadFile("../migrate/migrations/000270_reconcile_stale_user_modified_policies.up.sql")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(mig))
	require.NoError(t, err)

	require.False(t, policyUserModified(ctx, t, pool, "agent-protocol"), "reset por la migración")
	require.False(t, policyUserModified(ctx, t, pool, "agent-voice"), "reset por la migración")
	require.True(t, policyUserModified(ctx, t, pool, "sdd-auto-trigger"), "NO listado: preservado")

	runPlatformPoliciesSeeder(ctx, t, pool)

	require.Equal(t, agentprotocol.Full, policyBody(ctx, t, pool, "agent-protocol"),
		"agent-protocol reconciliado al body del fuente")
	av := policyBody(ctx, t, pool, "agent-voice")
	require.NotEqual(t, staleBody, av, "agent-voice ya no es el body stale")
	require.Contains(t, av, "Puede ser cálido", "agent-voice reconciliado al fuente neutral")
	require.Equal(t, staleBody, policyBody(ctx, t, pool, "sdd-auto-trigger"),
		"sdd-auto-trigger user_modified preservado por el guard del UPSERT")
}
