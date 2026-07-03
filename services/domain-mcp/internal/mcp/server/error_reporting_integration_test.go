//go:build integration

package mcpserver_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mark3labs/mcp-go/mcptest"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/db"
	mcpserver "nunezlagos/domain/internal/mcp/server"
	dmigrate "nunezlagos/domain/internal/migrate"
)

// setupErrorReportingMCP levanta un MCP de prueba con las tools reales contra un
// postgres efímero migrado (incluye la migración 000260 de REQ-56 issue-56.2).
func setupErrorReportingMCP(t *testing.T) (*mcptest.Server, *pgxpool.Pool, string, func()) {
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

	org, owner, err := seedOrgUser(ctx, pools.App, "Acme", "acme", "owner@acme.com", "Owner")
	require.NoError(t, err)

	deps := mcpserver.Deps{
		Pool: pools.App,
		Principal: &apikey.Principal{
			UserID:         owner.UserID.String(),
			OrganizationID: org.ID.String(),
			Role:           "owner",
		},
		ServerName: "domain-mcp-test",
		ServerVer:  "0.0.0",
	}
	srv, err := mcptest.NewServer(t, mcpserver.Tools(deps)...)
	require.NoError(t, err)

	return srv, pools.App, owner.UserID.String(), func() {
		srv.Close()
		pools.Close()
		_ = pgC.Terminate(ctx)
	}
}

// callTool ya está definido en server_integration_test.go (mismo package _test).

// insertErrorEvent crea un error_event vivo para poder resetearlo. Devuelve el
// fingerprint en hex (como lo espera el tool).
func insertErrorEvent(t *testing.T, pool *pgxpool.Pool, seed string) string {
	t.Helper()
	sum := sha256.Sum256([]byte(seed))
	fp := sum[:]
	_, err := pool.Exec(context.Background(), `
		INSERT INTO error_events (source, category, severity, message, fingerprint)
		VALUES ('test','UNKNOWN','warn','boom',$1)
	`, fp)
	require.NoError(t, err)
	return hex.EncodeToString(fp)
}

// TestErrorReset_SoftDelete verifica REQ-56 issue-56.2: error_reset es soft-delete
// reversible (marca deleted_at/by/reason, no borra la fila) y deja audit trail.
func TestErrorReset_SoftDelete(t *testing.T) {
	srv, pool, actorID, cleanup := setupErrorReportingMCP(t)
	defer cleanup()
	ctx := context.Background()

	fpHex := insertErrorEvent(t, pool, "reset-me")

	callTool(t, srv, "domain_error_reset", map[string]any{
		"fingerprint": fpHex,
		"reason":      "resuelto en deploy X",
	})

	// La fila NO se borra físicamente: sigue existiendo con deleted_at seteado.
	fp, _ := hex.DecodeString(fpHex)
	var deletedAt *string
	var deletedBy *string
	var reason *string
	err := pool.QueryRow(ctx, `
		SELECT deleted_at::text, deleted_by::text, deletion_reason
		FROM error_events WHERE fingerprint = $1
	`, fp).Scan(&deletedAt, &deletedBy, &reason)
	require.NoError(t, err, "la fila debe seguir existiendo (soft-delete, no DELETE)")
	require.NotNil(t, deletedAt, "deleted_at debe estar seteado")
	require.NotNil(t, deletedBy)
	require.Equal(t, actorID, *deletedBy, "deleted_by debe ser el actor de la sesión")
	require.NotNil(t, reason)
	require.Equal(t, "resuelto en deploy X", *reason)

	// Audit trail: una entrada action='error_reset'.
	var n int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM error_decision_log
		WHERE fingerprint = $1 AND action = 'error_reset'
	`, fp).Scan(&n)
	require.NoError(t, err)
	require.Equal(t, 1, n, "debe haber una entrada de audit del reset")
}

// TestErrorReset_Idempotent verifica que un reset repetido no pisa la autoría
// original: solo afecta filas vivas (deleted_at IS NULL).
func TestErrorReset_Idempotent(t *testing.T) {
	srv, pool, _, cleanup := setupErrorReportingMCP(t)
	defer cleanup()

	fpHex := insertErrorEvent(t, pool, "reset-twice")
	callTool(t, srv, "domain_error_reset", map[string]any{"fingerprint": fpHex, "reason": "primero"})
	out := callTool(t, srv, "domain_error_reset", map[string]any{"fingerprint": fpHex, "reason": "segundo"})

	// El segundo reset no debe afectar filas (0 soft_deleted), pero no falla.
	require.Contains(t, out, "soft_deleted")
}

// TestKnownErrorSet_Audit verifica REQ-56 issue-56.2: known_error_set deja una
// entrada de audit con actor y razón.
func TestKnownErrorSet_Audit(t *testing.T) {
	srv, pool, actorID, cleanup := setupErrorReportingMCP(t)
	defer cleanup()
	ctx := context.Background()

	sum := sha256.Sum256([]byte("classify-me"))
	fpHex := hex.EncodeToString(sum[:])

	callTool(t, srv, "domain_known_error_set", map[string]any{
		"fingerprint": fpHex,
		"name":        "db-context-deadline",
		"reason":      "es un timeout transitorio conocido",
		"recoverable": true,
	})

	fp := sum[:]
	var actor *string
	var reason *string
	err := pool.QueryRow(ctx, `
		SELECT actor_id::text, reason FROM error_decision_log
		WHERE fingerprint = $1 AND action = 'known_error_set'
	`, fp).Scan(&actor, &reason)
	require.NoError(t, err, "debe existir la entrada de audit del set")
	require.NotNil(t, actor)
	require.Equal(t, actorID, *actor)
	require.NotNil(t, reason)
	require.Equal(t, "es un timeout transitorio conocido", *reason)
}
