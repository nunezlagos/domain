//go:build integration

package spec_test

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
	specsvc "nunezlagos/domain/internal/service/spec"
)

type fix struct {
	svc  *specsvc.Service
	huID uuid.UUID
}

func setupSpec(t *testing.T) (*fix, func()) {
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
	svc := &specsvc.Service{Pool: pools.App, Audit: rec}

	var reqID, huID uuid.UUID
	err = pools.App.QueryRow(ctx,
		`INSERT INTO requirements (slug, title) VALUES ('REQ-spec-test', 'Spec Test REQ') RETURNING id`,
	).Scan(&reqID)
	require.NoError(t, err)

	err = pools.App.QueryRow(ctx,
		`INSERT INTO user_stories (req_id, slug, title) VALUES ($1, 'HU-spec-test', 'Test HU') RETURNING id`,
		reqID,
	).Scan(&huID)
	require.NoError(t, err)

	return &fix{svc: svc, huID: huID}, func() {
		pools.Close()
		_ = pgC.Terminate(ctx)
	}
}

func TestCreateProposal_OK(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	p, err := f.svc.CreateProposal(ctx, f.huID, "Intention", "Scope", "Approach", "Risks", "TestingNotes")
	require.NoError(t, err)
	require.Equal(t, f.huID, p.HuID)
	require.Equal(t, 1, p.Version)
	require.Equal(t, specsvc.PropStatusDraft, p.Status)
	require.Equal(t, "Intention", p.Intention)
	require.NotNil(t, p.Risks)
	require.Equal(t, "Risks", *p.Risks)
}

func TestCreateProposal_VersionIncrement(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	p1, err := f.svc.CreateProposal(ctx, f.huID, "v1", "Scope", "Approach", "", "")
	require.NoError(t, err)
	require.Equal(t, 1, p1.Version)

	p2, err := f.svc.CreateProposal(ctx, f.huID, "v2", "Scope", "Approach", "", "")
	require.NoError(t, err)
	require.Equal(t, 2, p2.Version)
}

func TestCreateProposal_HUNotFound(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	_, err := f.svc.CreateProposal(ctx, uuid.New(), "intent", "scope", "approach", "", "")
	require.ErrorIs(t, err, specsvc.ErrHUNotFound)
}

func TestGetLatestProposal_OK(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	_, _ = f.svc.CreateProposal(ctx, f.huID, "v1", "S1", "A1", "", "")
	_, _ = f.svc.CreateProposal(ctx, f.huID, "v2", "S2", "A2", "", "")

	latest, err := f.svc.GetLatestProposal(ctx, f.huID)
	require.NoError(t, err)
	require.Equal(t, 2, latest.Version)
}

func TestGetLatestProposal_NotFound(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	_, err := f.svc.GetLatestProposal(ctx, f.huID)
	require.ErrorIs(t, err, specsvc.ErrNotFound)
}

func TestGetProposalVersion_OK(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	p1, err := f.svc.CreateProposal(ctx, f.huID, "v1", "S1", "A1", "", "")
	require.NoError(t, err)

	got, err := f.svc.GetProposalVersion(ctx, f.huID, p1.Version)
	require.NoError(t, err)
	require.Equal(t, p1.ID, got.ID)
}

func TestGetProposalVersion_NotFound(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	_, err := f.svc.GetProposalVersion(ctx, f.huID, 99)
	require.ErrorIs(t, err, specsvc.ErrNotFound)
}

func TestListProposalVersions(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	_, _ = f.svc.CreateProposal(ctx, f.huID, "v1", "", "", "", "")
	_, _ = f.svc.CreateProposal(ctx, f.huID, "v2", "", "", "", "")

	versions, err := f.svc.ListProposalVersions(ctx, f.huID)
	require.NoError(t, err)
	require.Len(t, versions, 2)
}

func TestChangeProposalStatus_Approve(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	p, err := f.svc.CreateProposal(ctx, f.huID, "intent", "scope", "approach", "", "")
	require.NoError(t, err)

	updated, err := f.svc.ChangeProposalStatus(ctx, p.ID, specsvc.PropStatusApproved, "")
	require.NoError(t, err)
	require.Equal(t, specsvc.PropStatusApproved, updated.Status)
}

func TestChangeProposalStatus_Reject(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	p, err := f.svc.CreateProposal(ctx, f.huID, "intent", "scope", "approach", "", "")
	require.NoError(t, err)

	reason := "does not meet requirements"
	updated, err := f.svc.ChangeProposalStatus(ctx, p.ID, specsvc.PropStatusRejected, reason)
	require.NoError(t, err)
	require.Equal(t, specsvc.PropStatusRejected, updated.Status)
	require.NotNil(t, updated.RejectionReason)
	require.Equal(t, reason, *updated.RejectionReason)
}

func TestChangeProposalStatus_InvalidTransition(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	p, err := f.svc.CreateProposal(ctx, f.huID, "intent", "scope", "approach", "", "")
	require.NoError(t, err)

	_, err = f.svc.ChangeProposalStatus(ctx, p.ID, specsvc.PropStatusApproved, "")
	require.NoError(t, err)

	_, err = f.svc.ChangeProposalStatus(ctx, p.ID, specsvc.PropStatusDraft, "")
	require.ErrorIs(t, err, specsvc.ErrInvalidTransition)
}

func TestChangeProposalStatus_InvalidStatus(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	p, err := f.svc.CreateProposal(ctx, f.huID, "intent", "scope", "approach", "", "")
	require.NoError(t, err)

	_, err = f.svc.ChangeProposalStatus(ctx, p.ID, "bogus", "")
	require.ErrorIs(t, err, specsvc.ErrInvalidStatus)
}

func TestChangeProposalStatus_NotFound(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	_, err := f.svc.ChangeProposalStatus(ctx, uuid.New(), specsvc.PropStatusApproved, "")
	require.ErrorIs(t, err, specsvc.ErrNotFound)
}

func TestCreateDesign_OK(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	d, err := f.svc.CreateDesign(ctx, f.huID, nil, "Arch decisions", "Alt", "Flow", "TDD", "Risks")
	require.NoError(t, err)
	require.Equal(t, f.huID, d.HuID)
	require.Equal(t, 1, d.Version)
	require.Equal(t, specsvc.DesignStatusDraft, d.Status)
	require.Equal(t, "Arch decisions", d.ArchDecisions)
	require.NotNil(t, d.Alternatives)
	require.Equal(t, "Alt", *d.Alternatives)
}

func TestCreateDesign_VersionIncrement(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	d1, err := f.svc.CreateDesign(ctx, f.huID, nil, "v1", "", "", "", "")
	require.NoError(t, err)
	require.Equal(t, 1, d1.Version)

	d2, err := f.svc.CreateDesign(ctx, f.huID, nil, "v2", "", "", "", "")
	require.NoError(t, err)
	require.Equal(t, 2, d2.Version)
}

func TestCreateDesign_HUNotFound(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	_, err := f.svc.CreateDesign(ctx, uuid.New(), nil, "arch", "", "", "", "")
	require.ErrorIs(t, err, specsvc.ErrHUNotFound)
}

func TestGetLatestDesign_OK(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	_, _ = f.svc.CreateDesign(ctx, f.huID, nil, "v1", "", "", "", "")
	_, _ = f.svc.CreateDesign(ctx, f.huID, nil, "v2", "", "", "", "")

	latest, err := f.svc.GetLatestDesign(ctx, f.huID)
	require.NoError(t, err)
	require.Equal(t, 2, latest.Version)
}

func TestGetLatestDesign_NotFound(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	_, err := f.svc.GetLatestDesign(ctx, f.huID)
	require.ErrorIs(t, err, specsvc.ErrNotFound)
}

func TestListDesignsByHU(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	_, _ = f.svc.CreateDesign(ctx, f.huID, nil, "d1", "", "", "", "")
	_, _ = f.svc.CreateDesign(ctx, f.huID, nil, "d2", "", "", "", "")

	designs, err := f.svc.ListDesignsByHU(ctx, f.huID)
	require.NoError(t, err)
	require.Len(t, designs, 2)
}

func TestChangeDesignStatus_OK(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	d, err := f.svc.CreateDesign(ctx, f.huID, nil, "arch", "", "", "", "")
	require.NoError(t, err)

	updated, err := f.svc.ChangeDesignStatus(ctx, d.ID, specsvc.DesignStatusFinal)
	require.NoError(t, err)
	require.Equal(t, specsvc.DesignStatusFinal, updated.Status)
}

func TestChangeDesignStatus_InvalidStatus(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	d, err := f.svc.CreateDesign(ctx, f.huID, nil, "arch", "", "", "", "")
	require.NoError(t, err)

	_, err = f.svc.ChangeDesignStatus(ctx, d.ID, "bogus")
	require.ErrorIs(t, err, specsvc.ErrInvalidStatus)
}

func TestChangeDesignStatus_NotFound(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	_, err := f.svc.ChangeDesignStatus(ctx, uuid.New(), specsvc.DesignStatusFinal)
	require.ErrorIs(t, err, specsvc.ErrNotFound)
}

func TestSabotage_ProposalUniqueConstraint(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	_, err := f.svc.CreateProposal(ctx, f.huID, "v1", "S", "A", "", "")
	require.NoError(t, err)

	// Force-insert duplicado (hu_id + version violando UNIQUE)
	_, err = f.svc.Pool.Exec(ctx,
		`INSERT INTO proposals (hu_id, version, intention, scope, approach)
		 VALUES ($1, 1, 'dup', 'S', 'A')`, f.huID)
	require.ErrorContains(t, err, "unique")
}
