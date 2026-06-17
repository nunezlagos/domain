//go:build integration

// issue-21.6 paso 1: integration tests del PGStore de api keys.
//
// Cubre la ruta de AUTH (Issue → Resolve) que antes no tenía cobertura de DB, y
// valida que el org del Principal se deriva de users.organization_id (no de
// api_keys.organization_id, que se dejó de escribir/leer).

package apikey

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	dmigrate "nunezlagos/domain/internal/migrate"
)

// setupKeyStore levanta PG, migra, inserta org + user owner, y devuelve el store.
func setupKeyStore(t *testing.T) (*PGStore, uuid.UUID, uuid.UUID, func()) {
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

	var orgID, userID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('Acme', 'acme') RETURNING id`).Scan(&orgID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (organization_id, email, name, role)
		 VALUES ($1, 'owner@acme.com', 'Owner', 'owner') RETURNING id`, orgID).Scan(&userID))

	return &PGStore{Pool: pool}, orgID, userID, func() {
		pool.Close()
		_ = pgC.Terminate(ctx)
	}
}

// Ruta de auth: Issue emite, Resolve la valida y arma el Principal con el org
// derivado del user.
func TestStore_IssueAndResolve(t *testing.T) {
	s, orgID, userID, cleanup := setupKeyStore(t)
	defer cleanup()
	ctx := context.Background()

	plaintext, keyID, err := s.Issue(ctx, orgID, userID, "k1", "live")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, keyID)
	require.NotEmpty(t, plaintext)

	p, err := s.Resolve(ctx, plaintext)
	require.NoError(t, err)
	require.Equal(t, userID.String(), p.UserID)
	require.Equal(t, orgID.String(), p.OrganizationID, "org del Principal se deriva del user")
	require.Equal(t, keyID.String(), p.APIKeyID)
	require.Equal(t, "owner", p.Role)
}

func TestStore_Resolve_WrongKey(t *testing.T) {
	s, orgID, userID, cleanup := setupKeyStore(t)
	defer cleanup()
	ctx := context.Background()

	_, _, err := s.Issue(ctx, orgID, userID, "k1", "live")
	require.NoError(t, err)

	_, err = s.Resolve(ctx, "live_deadbeefdeadbeefdeadbeefdeadbeef")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestStore_Resolve_Revoked(t *testing.T) {
	s, orgID, userID, cleanup := setupKeyStore(t)
	defer cleanup()
	ctx := context.Background()

	plaintext, keyID, err := s.Issue(ctx, orgID, userID, "k1", "live")
	require.NoError(t, err)
	require.NoError(t, s.Revoke(ctx, keyID))

	_, err = s.Resolve(ctx, plaintext)
	require.ErrorIs(t, err, ErrNotFound, "una key revocada no resuelve")
}

func TestStore_List(t *testing.T) {
	s, orgID, userID, cleanup := setupKeyStore(t)
	defer cleanup()
	ctx := context.Background()

	_, _, err := s.Issue(ctx, orgID, userID, "k1", "live")
	require.NoError(t, err)
	_, _, err = s.Issue(ctx, orgID, userID, "k2", "live")
	require.NoError(t, err)

	keys, err := s.List(ctx, orgID)
	require.NoError(t, err)
	require.Len(t, keys, 2)
	for _, k := range keys {
		require.Equal(t, orgID, k.OrgID, "List deriva el org del user")
		require.Equal(t, userID, k.UserID)
	}
}

func TestStore_Rotate(t *testing.T) {
	s, orgID, userID, cleanup := setupKeyStore(t)
	defer cleanup()
	ctx := context.Background()

	oldPlain, oldID, err := s.Issue(ctx, orgID, userID, "k1", "live")
	require.NoError(t, err)

	newPlain, newID, err := s.Rotate(ctx, oldID, orgID, userID, "k1-rot", "live")
	require.NoError(t, err)
	require.NotEqual(t, oldID, newID)

	// la vieja ya no resuelve, la nueva sí (con el org del user)
	_, err = s.Resolve(ctx, oldPlain)
	require.ErrorIs(t, err, ErrNotFound)

	p, err := s.Resolve(ctx, newPlain)
	require.NoError(t, err)
	require.Equal(t, orgID.String(), p.OrganizationID)
	require.Equal(t, newID.String(), p.APIKeyID)
}
