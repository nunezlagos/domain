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
	svc     *specsvc.Service
	issueID uuid.UUID
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

	// project_id es NOT NULL en sdd_requirements tras el scoping por proyecto:
	// sembramos un project y lo asociamos al requirement.
	var projectID, reqID, issueID uuid.UUID
	err = pools.App.QueryRow(ctx,
		`INSERT INTO projects (name, slug) VALUES ('Spec Test Project', 'spec-test') RETURNING id`,
	).Scan(&projectID)
	require.NoError(t, err)

	err = pools.App.QueryRow(ctx,
		`INSERT INTO sdd_requirements (project_id, slug, title) VALUES ($1, 'REQ-spec-test', 'Spec Test REQ') RETURNING id`,
		projectID,
	).Scan(&reqID)
	require.NoError(t, err)

	err = pools.App.QueryRow(ctx,
		`INSERT INTO issues (project_id, req_id, slug, title) VALUES ($1, $2, 'HU-spec-test', 'Test HU') RETURNING id`,
		projectID, reqID,
	).Scan(&issueID)
	require.NoError(t, err)

	return &fix{svc: svc, issueID: issueID}, func() {
		pools.Close()
		_ = pgC.Terminate(ctx)
	}
}

func TestCreateProposal_OK(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	p, err := f.svc.CreateProposal(ctx, f.issueID, "Intention", "Scope", "Approach", "Risks", "TestingNotes")
	require.NoError(t, err)
	require.Equal(t, f.issueID, p.HuID)
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

	p1, err := f.svc.CreateProposal(ctx, f.issueID, "v1", "Scope", "Approach", "", "")
	require.NoError(t, err)
	require.Equal(t, 1, p1.Version)

	p2, err := f.svc.CreateProposal(ctx, f.issueID, "v2", "Scope", "Approach", "", "")
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

	_, _ = f.svc.CreateProposal(ctx, f.issueID, "v1", "S1", "A1", "", "")
	_, _ = f.svc.CreateProposal(ctx, f.issueID, "v2", "S2", "A2", "", "")

	latest, err := f.svc.GetLatestProposal(ctx, f.issueID)
	require.NoError(t, err)
	require.Equal(t, 2, latest.Version)
}

func TestGetLatestProposal_NotFound(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	_, err := f.svc.GetLatestProposal(ctx, f.issueID)
	require.ErrorIs(t, err, specsvc.ErrNotFound)
}

func TestGetProposalVersion_OK(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	p1, err := f.svc.CreateProposal(ctx, f.issueID, "v1", "S1", "A1", "", "")
	require.NoError(t, err)

	got, err := f.svc.GetProposalVersion(ctx, f.issueID, p1.Version)
	require.NoError(t, err)
	require.Equal(t, p1.ID, got.ID)
}

func TestGetProposalVersion_NotFound(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	_, err := f.svc.GetProposalVersion(ctx, f.issueID, 99)
	require.ErrorIs(t, err, specsvc.ErrNotFound)
}

func TestListProposalVersions(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	_, _ = f.svc.CreateProposal(ctx, f.issueID, "v1", "", "", "", "")
	_, _ = f.svc.CreateProposal(ctx, f.issueID, "v2", "", "", "", "")

	versions, err := f.svc.ListProposalVersions(ctx, f.issueID)
	require.NoError(t, err)
	require.Len(t, versions, 2)
}

func TestChangeProposalStatus_Approve(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	p, err := f.svc.CreateProposal(ctx, f.issueID, "intent", "scope", "approach", "", "")
	require.NoError(t, err)

	updated, err := f.svc.ChangeProposalStatus(ctx, p.ID, specsvc.PropStatusApproved, "")
	require.NoError(t, err)
	require.Equal(t, specsvc.PropStatusApproved, updated.Status)
}

func TestChangeProposalStatus_Reject(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	p, err := f.svc.CreateProposal(ctx, f.issueID, "intent", "scope", "approach", "", "")
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

	p, err := f.svc.CreateProposal(ctx, f.issueID, "intent", "scope", "approach", "", "")
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

	p, err := f.svc.CreateProposal(ctx, f.issueID, "intent", "scope", "approach", "", "")
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

	d, err := f.svc.CreateDesign(ctx, f.issueID, nil, "Arch decisions", "Alt", "Flow", "TDD", "Risks")
	require.NoError(t, err)
	require.Equal(t, f.issueID, d.HuID)
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

	d1, err := f.svc.CreateDesign(ctx, f.issueID, nil, "v1", "", "", "", "")
	require.NoError(t, err)
	require.Equal(t, 1, d1.Version)

	d2, err := f.svc.CreateDesign(ctx, f.issueID, nil, "v2", "", "", "", "")
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

	_, _ = f.svc.CreateDesign(ctx, f.issueID, nil, "v1", "", "", "", "")
	_, _ = f.svc.CreateDesign(ctx, f.issueID, nil, "v2", "", "", "", "")

	latest, err := f.svc.GetLatestDesign(ctx, f.issueID)
	require.NoError(t, err)
	require.Equal(t, 2, latest.Version)
}

func TestGetLatestDesign_NotFound(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	_, err := f.svc.GetLatestDesign(ctx, f.issueID)
	require.ErrorIs(t, err, specsvc.ErrNotFound)
}

func TestListDesignsByHU(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	_, _ = f.svc.CreateDesign(ctx, f.issueID, nil, "d1", "", "", "", "")
	_, _ = f.svc.CreateDesign(ctx, f.issueID, nil, "d2", "", "", "", "")

	designs, err := f.svc.ListDesignsByHU(ctx, f.issueID)
	require.NoError(t, err)
	require.Len(t, designs, 2)
}

func TestChangeDesignStatus_OK(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	d, err := f.svc.CreateDesign(ctx, f.issueID, nil, "arch", "", "", "", "")
	require.NoError(t, err)

	updated, err := f.svc.ChangeDesignStatus(ctx, d.ID, specsvc.DesignStatusFinal)
	require.NoError(t, err)
	require.Equal(t, specsvc.DesignStatusFinal, updated.Status)
}

func TestChangeDesignStatus_InvalidStatus(t *testing.T) {
	f, cleanup := setupSpec(t)
	defer cleanup()
	ctx := context.Background()

	d, err := f.svc.CreateDesign(ctx, f.issueID, nil, "arch", "", "", "", "")
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

	_, err := f.svc.CreateProposal(ctx, f.issueID, "v1", "S", "A", "", "")
	require.NoError(t, err)


	_, err = f.svc.Pool.Exec(ctx,
		`INSERT INTO sdd_proposals (issue_id, version, intention, scope, approach)
		 VALUES ($1, 1, 'dup', 'S', 'A')`, f.issueID)
	require.ErrorContains(t, err, "unique")
}
