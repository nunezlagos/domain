//go:build integration

// issue-21.6 Fase B: integration test del billing.Service que cubre S1.4
// usage_counters (PK swap a id BIGSERIAL + UNIQUE period_start) y S1.5 plans
// (sin cambios de schema, pero verificamos que GetPlan/ResolveLimits siguen
// funcionando con la FK organizations.plan_id que se dropeará en Fase C).

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

	var orgID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('Acme', 'acme') RETURNING id`).Scan(&orgID))

	return pool, orgID, func() {
		pool.Close()
		_ = pgC.Terminate(ctx)
	}
}

// --- usage_counters ---

func TestIncrementCounter_CreatesRowOnFirstCall(t *testing.T) {
	pool, orgID, cleanup := setupBillingDB(t)
	defer cleanup()
	svc := &Service{Pool: pool, now: func() time.Time {
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
	svc := &Service{Pool: pool, now: func() time.Time {
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
	svc := &Service{Pool: pool, now: func() time.Time {
		return time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	}}

	usage, err := svc.GetUsage(context.Background(), orgID)
	require.NoError(t, err)
	require.NotNil(t, usage)
	require.Equal(t, int64(0), usage.TokensUsed)
}

// --- plans ---

func TestGetPlan_ReturnsPlanBySlug(t *testing.T) {
	pool, _, cleanup := setupBillingDB(t)
	defer cleanup()
	svc := &Service{Pool: pool}

	plan, err := svc.GetPlan(context.Background(), "trial")
	require.NoError(t, err)
	require.NotNil(t, plan)
	require.Equal(t, "trial", plan.Slug)
}

func TestGetPlan_NotFound(t *testing.T) {
	pool, _, cleanup := setupBillingDB(t)
	defer cleanup()
	svc := &Service{Pool: pool}

	_, err := svc.GetPlan(context.Background(), "does-not-exist")
	require.ErrorIs(t, err, ErrPlanNotFound)
}
