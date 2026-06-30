//go:build integration

package mcpserver

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/auth/apikey"
	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/store/txctx"
)

// TestWireup_SurfacesSQLErrorOnRollback verifica que cuando una query
// dentro del tx falla (y el handler la ignora silenciosamente con
// `_ = err`), el wireup devuelve el error SQL original capturado por el
// tracer global — no el mensaje generico "transaction aborted before
// commit (Rollback)" que veíamos antes del fix.
//
// Reproduce el bug 38.13 ("ErrTxCommitRollback sin causa visible").
func TestWireup_SurfacesSQLErrorOnRollback(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupWireupPool(t, ctx)
	defer cleanup()

	orgID, userID := seedOrgAndMember(t, ctx, pool)
	principal := &apikey.Principal{
		OrganizationID: orgID.String(),
		UserID:         userID.String(),
		Role:           "owner",
	}
	deps := &Deps{Pool: pool, Principal: principal}

	handler := func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tx := txctx.TxFromContext(ctx)
		if tx == nil {
			return mcp.NewToolResultError("no tx in ctx"), nil
		}
		var x int
		if err := tx.QueryRow(ctx, `SELECT 1`).Scan(&x); err != nil {
			return mcp.NewToolResultErrorFromErr("ok-query failed", err), nil
		}
		// Query MALA — error ignorado (simula el patron del codigo real)
		if _, err := tx.Exec(ctx,
			`SELECT * FROM table_that_does_not_exist_xyz`); err != nil {
			// _ = err intencional: aqui estaba el bug
		}
		return mcp.NewToolResultText(fmt.Sprintf("handler done %d", x)), nil
	}

	wrapped := withOrgTxHandler(deps, handler)
	result, err := wrapped(ctx, mcp.CallToolRequest{})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.IsError, "result debe ser error porque la tx esta aborted")

	text := result.Content[0].(mcp.TextContent).Text
	t.Logf("result.Content[0]: %s", text)

	require.True(t, strings.Contains(text, "transaction aborted"),
		"el mensaje debe indicar que la tx fue abortada. Got: %s", text)
	require.True(t, strings.Contains(text, "table_that_does_not_exist_xyz"),
		"el SQL ejecutado debe aparecer para diagnosticar. Got: %s", text)
}

// TestWireup_CommitSuccessWhenNoErrors verifica el camino feliz: con
// un handler que no falla queries, Commit pasa y la response es success.
// Sanea que el cambio al wireup no rompio el happy path.
func TestWireup_CommitSuccessWhenNoErrors(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupWireupPool(t, ctx)
	defer cleanup()

	orgID, userID := seedOrgAndMember(t, ctx, pool)
	principal := &apikey.Principal{
		OrganizationID: orgID.String(),
		UserID:         userID.String(),
		Role:           "owner",
	}
	deps := &Deps{Pool: pool, Principal: principal}

	handler := func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tx := txctx.TxFromContext(ctx)
		if tx == nil {
			return mcp.NewToolResultError("no tx in ctx"), nil
		}
		var x int
		if err := tx.QueryRow(ctx, `SELECT 1`).Scan(&x); err != nil {
			return mcp.NewToolResultErrorFromErr("ok-query failed", err), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("ok %d", x)), nil
	}

	wrapped := withOrgTxHandler(deps, handler)
	result, err := wrapped(ctx, mcp.CallToolRequest{})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.IsError, "happy path no debe ser error")
	require.True(t, strings.Contains(result.Content[0].(mcp.TextContent).Text, "ok "),
		"happy path debe contener el texto del handler")
}

// helpers --------------------------------------------------------------

func setupWireupPool(t *testing.T, ctx context.Context) (*pgxpool.Pool, func()) {
	t.Helper()
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
	cleanup := func() { _ = pgC.Terminate(ctx) }

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	require.NoError(t, dmigrate.Up(dsn))

	cfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	cfg.ConnConfig.Tracer = SQLErrorCaptureTracer()
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	return pool, func() {
		pool.Close()
		cleanup()
	}
}

func seedOrgAndMember(t *testing.T, ctx context.Context, pool *pgxpool.Pool) (uuid.UUID, uuid.UUID) {
	t.Helper()
	var orgID, userID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug, settings) VALUES ('w-test', 'w-test', '{}') RETURNING id`,
	).Scan(&orgID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (organization_id, email, name, role) VALUES ($1, 'w-test@x', 'w-test', 'owner') RETURNING id`,
		orgID,
	).Scan(&userID))
	return orgID, userID
}
