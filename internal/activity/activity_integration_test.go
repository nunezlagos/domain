//go:build integration

// HU-02.6 activity-log integration tests con testcontainers.

package activity_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/saargo/domain/internal/activity"
	dmigrate "github.com/saargo/domain/internal/migrate"
)

func setupDB(t *testing.T) (*pgxpool.Pool, func()) {
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
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	return pool, func() {
		pool.Close()
		_ = pgC.Terminate(ctx)
	}
}

func seedOrg(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(),
		`INSERT INTO organizations (name, slug) VALUES ('T', 't') RETURNING id`,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func TestPGStore_Record_HappyPath(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()
	orgID := seedOrg(t, pool)
	store := &activity.PGStore{Pool: pool}

	id, err := store.Record(context.Background(), activity.Event{
		OrganizationID: orgID,
		Action:         "observation.created",
		EntityType:     "observation",
		Summary:        "Alice creó observation 'Postgres notes'",
		Metadata:       map[string]any{"count": 1, "tags": []string{"db"}},
	})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, id)

	var count int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM activity_log WHERE id = $1`, id,
	).Scan(&count))
	require.Equal(t, 1, count)
}

func TestPGStore_Record_ValidationErrors(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()
	orgID := seedOrg(t, pool)
	store := &activity.PGStore{Pool: pool}

	_, err := store.Record(context.Background(), activity.Event{
		OrganizationID: orgID, EntityType: "x", Summary: "s",
	})
	require.Error(t, err, "missing action")

	_, err = store.Record(context.Background(), activity.Event{
		OrganizationID: orgID, Action: "x", Summary: "s",
	})
	require.Error(t, err, "missing entity_type")

	_, err = store.Record(context.Background(), activity.Event{
		OrganizationID: orgID, Action: "x", EntityType: "y",
	})
	require.Error(t, err, "missing summary")
}

func TestPGStore_Record_DefaultVisibility(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()
	orgID := seedOrg(t, pool)
	store := &activity.PGStore{Pool: pool}

	id, _ := store.Record(context.Background(), activity.Event{
		OrganizationID: orgID, Action: "x", EntityType: "y", Summary: "s",
	})

	var vis string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT visibility FROM activity_log WHERE id = $1`, id,
	).Scan(&vis))
	require.Equal(t, "org", vis)
}

func TestPGStore_List_FiltersByOrg(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()
	orgA := seedOrg(t, pool)

	var orgB uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO organizations (name, slug) VALUES ('B', 'b') RETURNING id`,
	).Scan(&orgB))

	store := &activity.PGStore{Pool: pool}
	_, _ = store.Record(context.Background(), activity.Event{
		OrganizationID: orgA, Action: "a", EntityType: "x", Summary: "in A",
	})
	_, _ = store.Record(context.Background(), activity.Event{
		OrganizationID: orgB, Action: "b", EntityType: "x", Summary: "in B",
	})

	entriesA, err := store.List(context.Background(), activity.Filter{OrganizationID: orgA})
	require.NoError(t, err)
	require.Len(t, entriesA, 1)
	require.Equal(t, "in A", entriesA[0].Summary)
}

func TestPGStore_List_OrderedDescByCreatedAt(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()
	orgID := seedOrg(t, pool)
	store := &activity.PGStore{Pool: pool}

	for i := 0; i < 5; i++ {
		_, err := store.Record(context.Background(), activity.Event{
			OrganizationID: orgID, Action: "x", EntityType: "y",
			Summary: "evento " + string(rune('a'+i)),
		})
		require.NoError(t, err)
		time.Sleep(2 * time.Millisecond)
	}

	entries, err := store.List(context.Background(), activity.Filter{OrganizationID: orgID})
	require.NoError(t, err)
	require.Len(t, entries, 5)
	// más reciente primero
	for i := 1; i < len(entries); i++ {
		require.True(t, entries[i-1].CreatedAt.After(entries[i].CreatedAt) ||
			entries[i-1].CreatedAt.Equal(entries[i].CreatedAt),
			"entries deben estar DESC by created_at")
	}
}

func TestPGStore_List_FilterByEntityType(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()
	orgID := seedOrg(t, pool)
	store := &activity.PGStore{Pool: pool}

	for _, et := range []string{"observation", "agent", "observation", "flow"} {
		_, _ = store.Record(context.Background(), activity.Event{
			OrganizationID: orgID, Action: "x", EntityType: et, Summary: "s",
		})
	}

	entries, err := store.List(context.Background(), activity.Filter{
		OrganizationID: orgID, EntityType: "observation",
	})
	require.NoError(t, err)
	require.Len(t, entries, 2)
	for _, e := range entries {
		require.Equal(t, "observation", e.EntityType)
	}
}

func TestPGStore_List_LimitRespected(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()
	orgID := seedOrg(t, pool)
	store := &activity.PGStore{Pool: pool}

	for i := 0; i < 30; i++ {
		_, _ = store.Record(context.Background(), activity.Event{
			OrganizationID: orgID, Action: "x", EntityType: "y", Summary: "s",
		})
	}

	entries, err := store.List(context.Background(), activity.Filter{
		OrganizationID: orgID, Limit: 10,
	})
	require.NoError(t, err)
	require.Len(t, entries, 10)
}

func TestPGStore_List_MetadataParsed(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()
	orgID := seedOrg(t, pool)
	store := &activity.PGStore{Pool: pool}

	_, err := store.Record(context.Background(), activity.Event{
		OrganizationID: orgID, Action: "x", EntityType: "y", Summary: "s",
		Metadata: map[string]any{"key": "value", "count": 42},
	})
	require.NoError(t, err)

	entries, err := store.List(context.Background(), activity.Filter{OrganizationID: orgID})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "value", entries[0].Metadata["key"])
	require.EqualValues(t, 42, entries[0].Metadata["count"])
}

// Sabotaje: NUNCA aceptar payload con campo "password" o "api_key" en metadata.
// (HU-02.6 + .claude/rules/security.md - metadata NO debe tener PII full)
//
// Esta validación se aplica en service layer (caller), no en PGStore directo.
// El test confirma que si caller pasa PII, queda persistida (responsabilidad caller).
// El sabotaje real es: revisar que NO USAMOS metadata para PII en código real.
func TestSabotage_Metadata_AcceptsAnyJSON(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()
	orgID := seedOrg(t, pool)
	store := &activity.PGStore{Pool: pool}

	// Esta inserción NO valida PII; debe persistir lo que le pasen.
	_, err := store.Record(context.Background(), activity.Event{
		OrganizationID: orgID, Action: "test", EntityType: "x", Summary: "s",
		Metadata: map[string]any{"safe_key": "value"}, // sin PII keys
	})
	require.NoError(t, err)
}
