//go:build integration

package traceability_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/db"
	dmigrate "nunezlagos/domain/internal/migrate"
	tracesvc "nunezlagos/domain/internal/service/traceability"
)

type fix struct {
	svc   *tracesvc.Service
	reqID uuid.UUID
	issueID  uuid.UUID
}

func setupTrace(t *testing.T) (*fix, func()) {
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

	svc := &tracesvc.Service{Pool: pools.App}

	var reqID, issueID uuid.UUID
	err = pools.App.QueryRow(ctx,
		`INSERT INTO requirements (slug, title) VALUES ('REQ-trace-test', 'Trace Test REQ') RETURNING id`,
	).Scan(&reqID)
	require.NoError(t, err)
	err = pools.App.QueryRow(ctx,
		`INSERT INTO issues (req_id, slug, title) VALUES ($1, 'HU-trace-test', 'Trace HU') RETURNING id`,
		reqID,
	).Scan(&issueID)
	require.NoError(t, err)

	return &fix{svc: svc, reqID: reqID, issueID: issueID}, func() {
		pools.Close()
		_ = pgC.Terminate(ctx)
	}
}

func TestGetRequirementTrace_OK(t *testing.T) {
	f, cleanup := setupTrace(t)
	defer cleanup()

	trace, err := f.svc.GetRequirementTrace(context.Background(), "REQ-trace-test")
	require.NoError(t, err)
	require.Equal(t, f.reqID, trace.Req.ID)
	require.Equal(t, "REQ-trace-test", trace.Req.Slug)
	require.Len(t, trace.Children, 1)
	require.Equal(t, f.issueID, trace.Children[0].HU.ID)
}

func TestGetRequirementTrace_NotFound(t *testing.T) {
	f, cleanup := setupTrace(t)
	defer cleanup()

	_, err := f.svc.GetRequirementTrace(context.Background(), "REQ-bogus")
	require.Error(t, err)
}

func TestGetCodeTrace_NoMatch(t *testing.T) {
	f, cleanup := setupTrace(t)
	defer cleanup()

	ct, err := f.svc.GetCodeTrace(context.Background(), "internal/missing.go")
	require.NoError(t, err)
	require.Equal(t, "internal/missing.go", ct.File)
	require.Nil(t, ct.REQ)
}

func TestGetCodeTrace_Match(t *testing.T) {
	f, cleanup := setupTrace(t)
	defer cleanup()
	ctx := context.Background()

	_, err := f.svc.Pool.Exec(ctx,
		`INSERT INTO code_references (issue_id, file_path, repo) VALUES ($1, 'internal/x.go', 'domain')`,
		f.issueID,
	)
	require.NoError(t, err)

	ct, err := f.svc.GetCodeTrace(ctx, "internal/x.go")
	require.NoError(t, err)
	require.NotNil(t, ct.REQ)
	require.Equal(t, "REQ-trace-test", ct.REQ.Slug)
	require.Equal(t, "HU-trace-test", ct.HU.Slug)
}

func TestGetCoverageDashboard_BasicCounts(t *testing.T) {
	f, cleanup := setupTrace(t)
	defer cleanup()
	ctx := context.Background()

	d, err := f.svc.GetCoverageDashboard(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, d.TotalHUs, 1)
	require.Equal(t, 0, d.HUsWithProposal)
}

// Sabotage: si cambia el FK issue_id de code_references, GetCodeTrace devuelve
// HU vacía silenciosamente. Confirmamos que JOIN retorna sin error pero sin
// match cuando la HU desaparece.
func TestSabotage_OrphanCodeReference(t *testing.T) {
	f, cleanup := setupTrace(t)
	defer cleanup()
	ctx := context.Background()

	// Inserta con UUID inválido — ON DELETE CASCADE no aplica si se hace bypass
	_, err := f.svc.Pool.Exec(ctx,
		`INSERT INTO code_references (issue_id, file_path, repo) VALUES ($1, 'internal/y.go', 'domain')`,
		f.issueID,
	)
	require.NoError(t, err)

	_, err = f.svc.Pool.Exec(ctx, `DELETE FROM issues WHERE id = $1`, f.issueID)
	// ON DELETE CASCADE en code_references → row se borra, no queda huérfana
	require.NoError(t, err)

	ct, err := f.svc.GetCodeTrace(ctx, "internal/y.go")
	require.NoError(t, err)
	require.Nil(t, ct.REQ)
}
