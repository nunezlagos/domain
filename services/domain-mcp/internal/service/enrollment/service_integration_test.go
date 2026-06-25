//go:build integration





package enrollment

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/audit"
	dmigrate "nunezlagos/domain/internal/migrate"
)

func setupEnrollmentDB(t *testing.T) (*pgxpool.Pool, func()) {
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

func TestRotate_CreatesNewAndRevokesPrevious(t *testing.T) {
	pool, cleanup := setupEnrollmentDB(t)
	defer cleanup()
	svc := &Service{Pool: pool, Audit: &audit.NopRecorder{}}

	r1, err := svc.Rotate(context.Background(), uuid.Nil, "member")
	require.NoError(t, err)
	require.NotEmpty(t, r1.Plaintext)

	r2, err := svc.Rotate(context.Background(), uuid.Nil, "admin")
	require.NoError(t, err)
	require.NotEqual(t, r1.Plaintext, r2.Plaintext, "cada rotate emite plaintext nuevo")


	var total, active int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM enrollment_tokens`).Scan(&total))
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM enrollment_tokens WHERE revoked_at IS NULL`).Scan(&active))
	require.Equal(t, 2, total)
	require.Equal(t, 1, active, "single-org: solo 1 token activo a la vez (singleton constraint)")
}

func TestGetMetadata_EmptyWhenNoToken(t *testing.T) {
	pool, cleanup := setupEnrollmentDB(t)
	defer cleanup()
	svc := &Service{Pool: pool, Audit: &audit.NopRecorder{}}

	meta, err := svc.GetMetadata(context.Background())
	require.NoError(t, err)
	require.False(t, meta.Exists)
}

func TestGetMetadata_ReturnsActive(t *testing.T) {
	pool, cleanup := setupEnrollmentDB(t)
	defer cleanup()
	svc := &Service{Pool: pool, Audit: &audit.NopRecorder{}}

	_, err := svc.Rotate(context.Background(), uuid.Nil, "maintainer")
	require.NoError(t, err)

	meta, err := svc.GetMetadata(context.Background())
	require.NoError(t, err)
	require.True(t, meta.Exists)
	require.Equal(t, "maintainer", meta.RoleOnEnroll)
}

func TestRevoke_NoActiveReturnsErrNoActive(t *testing.T) {
	pool, cleanup := setupEnrollmentDB(t)
	defer cleanup()
	svc := &Service{Pool: pool, Audit: &audit.NopRecorder{}}

	err := svc.Revoke(context.Background(), uuid.Nil)
	require.ErrorIs(t, err, ErrNoActive)
}

func TestRevoke_RevokesActive(t *testing.T) {
	pool, cleanup := setupEnrollmentDB(t)
	defer cleanup()
	svc := &Service{Pool: pool, Audit: &audit.NopRecorder{}}

	_, err := svc.Rotate(context.Background(), uuid.Nil, "member")
	require.NoError(t, err)

	require.NoError(t, svc.Revoke(context.Background(), uuid.Nil))

	meta, err := svc.GetMetadata(context.Background())
	require.NoError(t, err)
	require.False(t, meta.Exists, "tras revoke no hay token activo")
}

func TestEnroll_WithValidToken_CreatesUserAndKey(t *testing.T) {
	pool, cleanup := setupEnrollmentDB(t)
	defer cleanup()
	svc := &Service{Pool: pool, Audit: &audit.NopRecorder{}}

	r, err := svc.Rotate(context.Background(), uuid.Nil, "member")
	require.NoError(t, err)

	res, err := svc.Enroll(context.Background(), r.Plaintext, "newbie@acme.com", "Newbie")
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, "newbie@acme.com", res.Email)
	require.Equal(t, "member", res.Role)
	require.NotEmpty(t, res.APIKey, "API key plaintext devuelto UNA vez")
}

func TestEnroll_WithInvalidToken_ReturnsErrInvalidToken(t *testing.T) {
	pool, cleanup := setupEnrollmentDB(t)
	defer cleanup()
	svc := &Service{Pool: pool, Audit: &audit.NopRecorder{}}

	_, err := svc.Enroll(context.Background(), "garbage", "x@acme.com", "x")
	require.ErrorIs(t, err, ErrInvalidToken)
}
