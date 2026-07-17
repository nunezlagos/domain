//go:build integration

package intake_test

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
	"nunezlagos/domain/internal/service/intake"
)

// setupIntake levanta la DB, el service y un proyecto fixture. Fase 2: Submit
// exige ProjectID (issue_intake_payloads.project_id es NOT NULL), asi que el
// fixture devuelve el id del proyecto al cual scopear los intakes del test.
func setupIntake(t *testing.T) (*intake.Service, uuid.UUID, uuid.UUID, func()) {
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


	var projectID uuid.UUID
	require.NoError(t, pools.App.QueryRow(ctx,
		`INSERT INTO projects (name, slug)
		 VALUES ('Test Project', 'test-project') RETURNING id`,
	).Scan(&projectID))

	// issue_intake_payloads.reviewer_id tiene FK a users(id): Approve/Reject
	// escriben ese campo, asi que sembramos un user real y devolvemos su id
	// para usarlo como reviewer en los tests.
	var reviewerID uuid.UUID
	require.NoError(t, pools.App.QueryRow(ctx,
		`INSERT INTO users (email, name, role)
		 VALUES ('reviewer@test.com', 'Reviewer', 'owner') RETURNING id`,
	).Scan(&reviewerID))

	svc := &intake.Service{Pool: pools.App, Audit: &audit.PGRecorder{Pool: pools.Auth}}
	return svc, projectID, reviewerID, func() {
		pools.Close()
		_ = pgC.Terminate(ctx)
	}
}

func TestSubmit_AgentSource_OK(t *testing.T) {
	svc, projectID, _, cleanup := setupIntake(t)
	defer cleanup()

	p, err := svc.Submit(context.Background(), intake.SubmitInput{
		Source:    intake.SourceAgent,
		ProjectID: &projectID,
		RawText:   "El export a CSV no aparece para directores",
	})
	require.NoError(t, err)
	require.Equal(t, intake.StatusReceived, p.Status)
	require.Equal(t, "agent", p.Source)
}

func TestSubmit_InvalidSource(t *testing.T) {
	svc, _, _, cleanup := setupIntake(t)
	defer cleanup()

	_, err := svc.Submit(context.Background(), intake.SubmitInput{
		Source:  "twitter",
		RawText: "x",
	})
	require.ErrorIs(t, err, intake.ErrInvalidSource)
}

func TestSubmit_EmptyText(t *testing.T) {
	svc, _, _, cleanup := setupIntake(t)
	defer cleanup()

	_, err := svc.Submit(context.Background(), intake.SubmitInput{
		Source: intake.SourceAgent,
	})
	require.Error(t, err)
}

func TestFullFlow_AgentToCommitted(t *testing.T) {
	svc, projectID, reviewerID, cleanup := setupIntake(t)
	defer cleanup()
	ctx := context.Background()

	p, err := svc.Submit(ctx, intake.SubmitInput{
		Source:    intake.SourceAgent,
		ProjectID: &projectID,
		RawText:   "Necesito exportar runs como CSV",
	})
	require.NoError(t, err)

	_, err = svc.UpdateClassification(ctx, p.ID, "feat", "medium", 0.85, "feature request claro")
	require.NoError(t, err)

	_, err = svc.MarkPendingReview(ctx, p.ID,
		"CSV export de runs",
		"Endpoint streaming para exportar runs filtrados",
		"REQ-08-agent-system",
		map[string]any{"slug": "csv-export"},
		[]any{},
		intake.MergeActionCreateNew,
	)
	require.NoError(t, err)

	approved, err := svc.Approve(ctx, p.ID, reviewerID)
	require.NoError(t, err)
	require.Equal(t, intake.StatusApproved, approved.Status)

	// committed_req_id -> sdd_requirements(id) y committed_issue_id ->
	// issues(id) tienen FK (requirements/user_stories fueron renombradas en
	// migs 000151/000073). Ambas exigen project_id NOT NULL (mig 000167).
	var reqID, issueID uuid.UUID
	require.NoError(t, svc.Pool.QueryRow(ctx,
		`INSERT INTO sdd_requirements (slug, title, project_id) VALUES ('REQ-99-intake', 'Req intake', $1) RETURNING id`,
		projectID,
	).Scan(&reqID))
	require.NoError(t, svc.Pool.QueryRow(ctx,
		`INSERT INTO issues (slug, title, req_id, project_id) VALUES ('HU-99-intake', 'HU intake', $1, $2) RETURNING id`,
		reqID, projectID,
	).Scan(&issueID))
	committed, err := svc.LinkCommitted(ctx, p.ID, &reqID, &issueID)
	require.NoError(t, err)
	require.Equal(t, intake.StatusCommitted, committed.Status)
}

func TestApprove_WrongStatus(t *testing.T) {
	svc, projectID, _, cleanup := setupIntake(t)
	defer cleanup()
	ctx := context.Background()
	p, _ := svc.Submit(ctx, intake.SubmitInput{Source: intake.SourceAgent, ProjectID: &projectID, RawText: "x"})
	_, err := svc.Approve(ctx, p.ID, uuid.New())
	require.ErrorIs(t, err, intake.ErrInvalidStatus)
}

func TestReject_FromPending(t *testing.T) {
	svc, projectID, reviewerID, cleanup := setupIntake(t)
	defer cleanup()
	ctx := context.Background()
	p, _ := svc.Submit(ctx, intake.SubmitInput{Source: intake.SourceAgent, ProjectID: &projectID, RawText: "x"})
	_, _ = svc.UpdateClassification(ctx, p.ID, "fix", "low", 0.7, "")
	_, _ = svc.MarkPendingReview(ctx, p.ID, "t", "d", "REQ-04-opsx-sdd",
		map[string]any{}, []any{}, intake.MergeActionCreateNew)
	rejected, err := svc.Reject(ctx, p.ID, reviewerID, "duplicate of issue-99")
	require.NoError(t, err)
	require.Equal(t, intake.StatusRejected, rejected.Status)
	require.NotNil(t, rejected.RejectionReason)
}

func TestListPending_ExcludesCommitted(t *testing.T) {
	svc, projectID, reviewerID, cleanup := setupIntake(t)
	defer cleanup()
	ctx := context.Background()

	p1, _ := svc.Submit(ctx, intake.SubmitInput{Source: intake.SourceAgent, ProjectID: &projectID, RawText: "open"})
	p2, _ := svc.Submit(ctx, intake.SubmitInput{Source: intake.SourceAgent, ProjectID: &projectID, RawText: "closed"})
	_, _ = svc.UpdateClassification(ctx, p2.ID, "fix", "low", 0.9, "")
	_, _ = svc.MarkPendingReview(ctx, p2.ID, "t", "d", "REQ-04-opsx-sdd",
		map[string]any{}, []any{}, intake.MergeActionCreateNew)
	_, _ = svc.Approve(ctx, p2.ID, reviewerID)
	var reqID, issueID uuid.UUID
	require.NoError(t, svc.Pool.QueryRow(ctx,
		`INSERT INTO sdd_requirements (slug, title, project_id) VALUES ('REQ-99-list', 'Req list', $1) RETURNING id`,
		projectID,
	).Scan(&reqID))
	require.NoError(t, svc.Pool.QueryRow(ctx,
		`INSERT INTO issues (slug, title, req_id, project_id) VALUES ('HU-99-list', 'HU list', $1, $2) RETURNING id`,
		reqID, projectID,
	).Scan(&issueID))
	_, _ = svc.LinkCommitted(ctx, p2.ID, nil, &issueID)

	list, err := svc.ListPending(ctx, 10)
	require.NoError(t, err)
	ids := map[uuid.UUID]bool{}
	for _, x := range list {
		ids[x.ID] = true
	}
	require.True(t, ids[p1.ID], "open intake should be listed")
	require.False(t, ids[p2.ID], "committed intake should NOT be listed")
}

// Sabotaje: status='committed' impide approve directo.
func TestSabotage_ApproveOnCommittedRejected(t *testing.T) {
	svc, projectID, _, cleanup := setupIntake(t)
	defer cleanup()
	ctx := context.Background()
	p, _ := svc.Submit(ctx, intake.SubmitInput{Source: intake.SourceAgent, ProjectID: &projectID, RawText: "x"})


	_, err := svc.Pool.Exec(ctx,
		`UPDATE issue_intake_payloads SET status = 'committed' WHERE id = $1`, p.ID)
	require.NoError(t, err)

	_, err = svc.Approve(ctx, p.ID, uuid.New())
	require.ErrorIs(t, err, intake.ErrInvalidStatus)
}
