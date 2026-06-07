//go:build integration

// HU-02.4 audit-log integration tests con testcontainers.

package audit_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/audit"
	dmigrate "nunezlagos/domain/internal/migrate"
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
		`INSERT INTO organizations (name, slug) VALUES ('Test', 'test') RETURNING id`,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// Escenario 1: Record básico inserta row.
func TestPGRecorder_Record_HappyPath(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	orgID := seedOrg(t, pool)
	r := &audit.PGRecorder{Pool: pool}

	err := r.Record(ctx, audit.Event{
		OrganizationID: &orgID,
		ActorType:      audit.ActorUser,
		Action:         "test.action",
		EntityType:     "test_entity",
		NewValues:      map[string]any{"foo": "bar"},
		IPAddress:      "10.0.0.1",
		RequestID:      "req-abc",
	})
	require.NoError(t, err)

	var count int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_log WHERE action='test.action'`).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

// Escenario 2: JSON diff oldValues/newValues persistido.
func TestPGRecorder_Record_JSONValues(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	orgID := seedOrg(t, pool)
	r := &audit.PGRecorder{Pool: pool}

	err := r.Record(ctx, audit.Event{
		OrganizationID: &orgID,
		Action:         "user.updated",
		EntityType:     "user",
		OldValues:      map[string]any{"name": "old"},
		NewValues:      map[string]any{"name": "new"},
	})
	require.NoError(t, err)

	var oldJSON, newJSON []byte
	err = pool.QueryRow(ctx, `
		SELECT old_values, new_values FROM audit_log WHERE action='user.updated'
	`).Scan(&oldJSON, &newJSON)
	require.NoError(t, err)
	require.Contains(t, string(oldJSON), "old")
	require.Contains(t, string(newJSON), "new")
}

// Escenario 3: Required fields validation.
func TestPGRecorder_Record_ValidationErrors(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	r := &audit.PGRecorder{Pool: pool}

	require.Error(t, r.Record(ctx, audit.Event{EntityType: "x"}), "missing action")
	require.Error(t, r.Record(ctx, audit.Event{Action: "x"}), "missing entity_type")
}

// Escenario 4: ActorType default a system si vacío.
func TestPGRecorder_DefaultActorType(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	orgID := seedOrg(t, pool)
	r := &audit.PGRecorder{Pool: pool}

	err := r.Record(ctx, audit.Event{
		OrganizationID: &orgID,
		Action:         "auto",
		EntityType:     "x",
	})
	require.NoError(t, err)

	var actorType string
	err = pool.QueryRow(ctx, `SELECT actor_type FROM audit_log WHERE action='auto'`).Scan(&actorType)
	require.NoError(t, err)
	require.Equal(t, "system", actorType)
}

// Sabotaje: simular un app_user que intenta UPDATE → debe fallar (HU-25.6 REVOKE).
func TestSabotage_AuditLog_AppUser_UpdateDenied(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	orgID := seedOrg(t, pool)
	r := &audit.PGRecorder{Pool: pool}
	require.NoError(t, r.Record(ctx, audit.Event{
		OrganizationID: &orgID,
		Action:         "to_be_hidden",
		EntityType:     "test",
	}))

	// Setear password al rol app_user para login y probar UPDATE
	_, err := pool.Exec(ctx, `ALTER ROLE app_user WITH LOGIN PASSWORD 'testpass'`)
	require.NoError(t, err)

	// Conectar como app_user
	dsn := pool.Config().ConnString()
	// reemplazar credenciales (mismo host/db)
	// formato: postgres://USER:PASS@host:port/db?...
	// Asumimos formato estándar testcontainers.
	pubConn, err := pgx.Connect(ctx, replaceCredentials(dsn, "app_user", "testpass"))
	require.NoError(t, err)
	defer pubConn.Close(ctx)

	_, err = pubConn.Exec(ctx, `UPDATE audit_log SET action='hidden' WHERE action='to_be_hidden'`)
	require.Error(t, err, "app_user UPDATE audit_log debe fallar (HU-25.6 REVOKE UPDATE)")

	_, err = pubConn.Exec(ctx, `DELETE FROM audit_log WHERE action='to_be_hidden'`)
	require.Error(t, err, "app_user DELETE audit_log debe fallar")
}

// replaceCredentials reemplaza user:pass en un postgres:// URL.
func replaceCredentials(dsn, user, pass string) string {
	// formato: postgres://OLDUSER:OLDPASS@HOST...
	atIdx := -1
	schemeEnd := -1
	for i := 0; i < len(dsn); i++ {
		if i+3 <= len(dsn) && dsn[i:i+3] == "://" {
			schemeEnd = i + 3
			break
		}
	}
	for i := schemeEnd; i < len(dsn); i++ {
		if dsn[i] == '@' {
			atIdx = i
			break
		}
	}
	if schemeEnd < 0 || atIdx < 0 {
		return dsn
	}
	return dsn[:schemeEnd] + user + ":" + pass + dsn[atIdx:]
}
