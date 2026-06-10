//go:build integration

package extsync_test

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
	"nunezlagos/domain/internal/service/extsync"
)

type fix struct {
	svc      *extsync.Service
	orgID    uuid.UUID
	provider *extsync.Provider
}

func setupExtSync(t *testing.T) (*fix, func()) {
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

	svc := &extsync.Service{Pool: pools.App}

	var orgID uuid.UUID
	err = pools.App.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('Acme', 'acme') RETURNING id`,
	).Scan(&orgID)
	require.NoError(t, err)

	provider, err := svc.RegisterProvider(ctx, orgID, extsync.ProviderJira,
		"Acme Jira", "https://acme.atlassian.net", "DIDE",
		map[string]any{"auth_ref": "jira_token_v1"})
	require.NoError(t, err)

	return &fix{svc: svc, orgID: orgID, provider: provider}, func() {
		pools.Close()
		_ = pgC.Terminate(ctx)
	}
}

func TestRegisterProvider_Upsert(t *testing.T) {
	f, cleanup := setupExtSync(t)
	defer cleanup()

	// Re-registrar mismo provider con base_url distinto debe hacer upsert.
	updated, err := f.svc.RegisterProvider(context.Background(), f.orgID,
		extsync.ProviderJira, "Acme Jira Updated",
		"https://acme.atlassian.net/v2", "DIDE",
		map[string]any{"auth_ref": "jira_token_v2"})
	require.NoError(t, err)
	require.Equal(t, f.provider.ID, updated.ID)
	require.Equal(t, "Acme Jira Updated", updated.DisplayName)
}

func TestRegisterProvider_InvalidProvider(t *testing.T) {
	f, cleanup := setupExtSync(t)
	defer cleanup()
	_, err := f.svc.RegisterProvider(context.Background(), f.orgID,
		"bitbucket", "x", "https://y", "", nil)
	require.ErrorIs(t, err, extsync.ErrInvalidProvider)
}

func TestRegisterPush_OK(t *testing.T) {
	f, cleanup := setupExtSync(t)
	defer cleanup()
	issueID := uuid.New()

	st, err := f.svc.RegisterPush(context.Background(), f.provider.ID,
		extsync.EntityHU, issueID, "DIDE-100",
		"https://acme.atlassian.net/browse/DIDE-100", "Story",
		map[string]any{"summary": "Test HU"})
	require.NoError(t, err)
	require.Equal(t, extsync.StatusOK, st.SyncStatus)
	require.Equal(t, "DIDE-100", st.ExternalKey)
	require.NotNil(t, st.LastPushedAt)
}

func TestRegisterPush_InvalidEntity(t *testing.T) {
	f, cleanup := setupExtSync(t)
	defer cleanup()
	_, err := f.svc.RegisterPush(context.Background(), f.provider.ID,
		"epic", uuid.New(), "K-1", "url", "Story", nil)
	require.ErrorIs(t, err, extsync.ErrInvalidEntity)
}

func TestMarkDrift_AndResolve(t *testing.T) {
	f, cleanup := setupExtSync(t)
	defer cleanup()
	ctx := context.Background()

	st, err := f.svc.RegisterPush(ctx, f.provider.ID, extsync.EntityHU,
		uuid.New(), "DIDE-101", "url", "Story",
		map[string]any{"summary": "Original"})
	require.NoError(t, err)

	drift, err := f.svc.MarkDrift(ctx, st.ID, map[string]any{
		"summary": map[string]any{"jira": "Edited", "last_pushed": "Original"},
	})
	require.NoError(t, err)
	require.Equal(t, extsync.StatusConflict, drift.SyncStatus)
	require.NotNil(t, drift.DriftDetectedAt)

	resolved, err := f.svc.MarkResolved(ctx, st.ID)
	require.NoError(t, err)
	require.Equal(t, extsync.StatusOK, resolved.SyncStatus)
	require.Nil(t, resolved.DriftDetectedAt)
}

func TestMarkPartial(t *testing.T) {
	f, cleanup := setupExtSync(t)
	defer cleanup()
	ctx := context.Background()

	st, _ := f.svc.RegisterPush(ctx, f.provider.ID, extsync.EntityHU,
		uuid.New(), "DIDE-102", "url", "Story", nil)
	partial, err := f.svc.MarkPartial(ctx, st.ID, []any{
		map[string]any{"attachment": "img1.png", "error": "429"},
	})
	require.NoError(t, err)
	require.Equal(t, extsync.StatusPartial, partial.SyncStatus)
}

func TestGetByEntity(t *testing.T) {
	f, cleanup := setupExtSync(t)
	defer cleanup()
	ctx := context.Background()

	issueID := uuid.New()
	created, _ := f.svc.RegisterPush(ctx, f.provider.ID, extsync.EntityHU,
		issueID, "DIDE-103", "url", "Story", nil)

	got, err := f.svc.GetByEntity(ctx, f.provider.ID, extsync.EntityHU, issueID)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
}

func TestListConflicts(t *testing.T) {
	f, cleanup := setupExtSync(t)
	defer cleanup()
	ctx := context.Background()

	st, _ := f.svc.RegisterPush(ctx, f.provider.ID, extsync.EntityHU,
		uuid.New(), "DIDE-104", "url", "Story", nil)
	_, _ = f.svc.MarkDrift(ctx, st.ID, map[string]any{"x": "y"})

	conflicts, err := f.svc.ListConflicts(ctx, 10)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(conflicts), 1)
}

// Sabotaje: UNIQUE(provider_id, external_key) impide duplicar
func TestSabotage_DuplicateExternalKey(t *testing.T) {
	f, cleanup := setupExtSync(t)
	defer cleanup()
	ctx := context.Background()

	_, err := f.svc.RegisterPush(ctx, f.provider.ID, extsync.EntityHU,
		uuid.New(), "DIDE-DUP", "url", "Story", nil)
	require.NoError(t, err)

	_, err = f.svc.RegisterPush(ctx, f.provider.ID, extsync.EntityHU,
		uuid.New(), "DIDE-DUP", "url2", "Story", nil)
	require.Error(t, err)
}
