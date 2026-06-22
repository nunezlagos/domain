//go:build integration

package issuebuilder_test

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
	dmigrate "nunezlagos/domain/internal/migrate"
	hb "nunezlagos/domain/internal/service/issuebuilder"
)

func setupHB(t *testing.T) (*hb.Service, func()) {
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
	svc := &hb.Service{Pool: pools.App, Audit: rec}

	return svc, func() {
		pools.Close()
		_ = pgC.Terminate(ctx)
	}
}

func TestStart_FeatureMode_FirstQuestion(t *testing.T) {
	svc, cleanup := setupHB(t)
	defer cleanup()
	ctx := context.Background()

	d, q, err := svc.Start(ctx, hb.ModeFeature, "Quiero exportar runs a CSV", nil, nil)
	require.NoError(t, err)
	require.Equal(t, hb.StatusInProgress, d.Status)
	require.Equal(t, 8, d.TotalSteps)
	require.NotNil(t, q)
	require.Equal(t, "change_type", q.Key)
	require.Equal(t, "1/8", q.Progress)
	require.NotEmpty(t, q.Options)
}

func TestStart_InvalidMode(t *testing.T) {
	svc, cleanup := setupHB(t)
	defer cleanup()
	_, _, err := svc.Start(context.Background(), "bogus", "idea", nil, nil)
	require.ErrorIs(t, err, hb.ErrInvalidMode)
}

func TestStart_UnsupportedMode(t *testing.T) {
	t.Skip("pre-existente: todos los modes (feature/bug-fix/refactor/doc/rfc) tienen flow implementado, ErrUnsupportedMode no se dispara. Se mantiene como guard para cuando se agregue un mode nuevo con flow placeholder.")
	svc, cleanup := setupHB(t)
	defer cleanup()
	_, _, err := svc.Start(context.Background(), hb.ModeBugFix, "idea", nil, nil)
	require.ErrorIs(t, err, hb.ErrUnsupportedMode)
}

func TestStart_EmptyIdea(t *testing.T) {
	svc, cleanup := setupHB(t)
	defer cleanup()
	_, _, err := svc.Start(context.Background(), hb.ModeFeature, "  ", nil, nil)
	require.Error(t, err)
}

func TestAnswer_AdvancesStep(t *testing.T) {
	svc, cleanup := setupHB(t)
	defer cleanup()
	ctx := context.Background()

	d, _, err := svc.Start(ctx, hb.ModeFeature, "idea", nil, nil)
	require.NoError(t, err)

	d2, q, err := svc.Answer(ctx, d.ID, "feature")
	require.NoError(t, err)
	require.Equal(t, 1, d2.CurrentStep)
	require.Equal(t, "audience", q.Key)
}

func TestAnswer_InvalidValue(t *testing.T) {
	svc, cleanup := setupHB(t)
	defer cleanup()
	ctx := context.Background()

	d, _, err := svc.Start(ctx, hb.ModeFeature, "idea", nil, nil)
	require.NoError(t, err)

	_, _, err = svc.Answer(ctx, d.ID, "bogus_change_type")
	require.ErrorIs(t, err, hb.ErrInvalidAnswer)
}

func TestAnswer_NotFound(t *testing.T) {
	svc, cleanup := setupHB(t)
	defer cleanup()
	_, _, err := svc.Answer(context.Background(), uuid.New(), "feature")
	require.ErrorIs(t, err, hb.ErrNotFound)
}

func TestFullFlow_FeatureMode(t *testing.T) {
	svc, cleanup := setupHB(t)
	defer cleanup()
	ctx := context.Background()

	d, _, err := svc.Start(ctx, hb.ModeFeature, "Exportar runs CSV", nil, nil)
	require.NoError(t, err)

	answers := []any{
		"feature",
		"dx-engineer",
		"REQ-08-agent-system",
		"M",
		"alta",
		"agent-runs-csv-export",
		"Exportar runs como CSV streaming",
		"Endpoint /api/v1/runs/export.csv que streamea runs filtrados",
	}

	for i, a := range answers {
		d2, q, err := svc.Answer(ctx, d.ID, a)
		require.NoErrorf(t, err, "step %d failed", i)
		if i < len(answers)-1 {
			require.NotNil(t, q, "expected next question at step %d", i)
		} else {
			require.Nil(t, q, "expected no question after last step")
			require.Equal(t, hb.StatusFinished, d2.Status)
		}
	}

	prev, err := svc.BuildPreview(ctx, d.ID)
	require.NoError(t, err)
	require.Contains(t, prev.Files, "issue.md")
	require.Contains(t, prev.Files, "state.yaml")
	require.Contains(t, prev.TargetPath, "REQ-08-agent-system")
	require.Contains(t, prev.SuggestedSlug, "agent-runs-csv-export")

	committed, err := svc.Commit(ctx, d.ID)
	require.NoError(t, err)
	require.Equal(t, hb.StatusCommitted, committed.Status)
	require.NotNil(t, committed.CommittedAt)
}

func TestAbandon_FromInProgress(t *testing.T) {
	svc, cleanup := setupHB(t)
	defer cleanup()
	ctx := context.Background()

	d, _, err := svc.Start(ctx, hb.ModeFeature, "idea", nil, nil)
	require.NoError(t, err)

	err = svc.Abandon(ctx, d.ID, "changed mind")
	require.NoError(t, err)

	d2, err := svc.Get(ctx, d.ID)
	require.NoError(t, err)
	require.Equal(t, hb.StatusAbandoned, d2.Status)
}

func TestList_ByStatus(t *testing.T) {
	svc, cleanup := setupHB(t)
	defer cleanup()
	ctx := context.Background()

	_, _, _ = svc.Start(ctx, hb.ModeFeature, "idea1", nil, nil)
	_, _, _ = svc.Start(ctx, hb.ModeFeature, "idea2", nil, nil)

	drafts, err := svc.List(ctx, hb.StatusInProgress)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(drafts), 2)
}

// Sabotaje: si current_step se incrementa más allá del flow, Answer debe
// fallar (no infinite loop). Forzamos current_step a un valor inválido y
// verificamos detección.
func TestSabotage_OvershootStep(t *testing.T) {
	svc, cleanup := setupHB(t)
	defer cleanup()
	ctx := context.Background()

	d, _, err := svc.Start(ctx, hb.ModeFeature, "idea", nil, nil)
	require.NoError(t, err)

	// Forzamos current_step más allá del último válido
	_, err = svc.Pool.Exec(ctx,
		`UPDATE issue_drafts SET current_step = total_steps + 5 WHERE id = $1`, d.ID,
	)
	require.NoError(t, err)

	_, _, err = svc.Answer(ctx, d.ID, "anything")
	require.Error(t, err)
}
