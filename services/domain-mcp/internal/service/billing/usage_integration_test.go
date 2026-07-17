//go:build integration







package billing

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

	dmigrate "nunezlagos/domain/internal/migrate"
)

func setupBillingDB(t *testing.T) (*pgxpool.Pool, uuid.UUID, func()) {
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

	// org sin respaldo en BD tras 000142/000143: UUID libre en memoria.
	orgID := uuid.New()

	return pool, orgID, func() {
		pool.Close()
		_ = pgC.Terminate(ctx)
	}
}



func TestIncrementCounter_CreatesRowOnFirstCall(t *testing.T) {
	pool, orgID, cleanup := setupBillingDB(t)
	defer cleanup()
	svc := &Service{Pool: pool, Now: func() time.Time {
		return time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	}}

	usage, err := svc.IncrementTokens(context.Background(), orgID, 1000)
	require.NoError(t, err)
	require.Equal(t, int64(1000), usage.TokensUsed)
	require.Equal(t, int32(0), usage.RunsCount)
}

func TestIncrementCounter_UpdatesRowOnSamePeriod(t *testing.T) {
	pool, orgID, cleanup := setupBillingDB(t)
	defer cleanup()
	svc := &Service{Pool: pool, Now: func() time.Time {
		return time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	}}

	_, err := svc.IncrementTokens(context.Background(), orgID, 500)
	require.NoError(t, err)
	usage, err := svc.IncrementTokens(context.Background(), orgID, 300)
	require.NoError(t, err)
	require.Equal(t, int64(800), usage.TokensUsed, "segundo increment suma (ON CONFLICT UPDATE)")
}

func TestGetUsage_NoRowReturnsZeroUsage(t *testing.T) {
	pool, orgID, cleanup := setupBillingDB(t)
	defer cleanup()
	svc := &Service{Pool: pool, Now: func() time.Time {
		return time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	}}

	usage, err := svc.GetUsage(context.Background(), orgID)
	require.NoError(t, err)
	require.NotNil(t, usage)
	require.Equal(t, int64(0), usage.TokensUsed)
}
