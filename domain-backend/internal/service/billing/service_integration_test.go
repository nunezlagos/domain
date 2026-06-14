//go:build integration

package billing_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/db"
	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/service/billing"
	orgsvc "nunezlagos/domain/internal/service/org"
)

type fix struct {
	svc   *billing.Service
	orgID uuid.UUID
}

func setupBilling(t *testing.T) (*fix, func()) {
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
	orgS := &orgsvc.Service{Pool: pools.App, Audit: rec}
	org, _, err := orgS.Create(ctx, "Acme", "acme", "o@x.com", "O")
	require.NoError(t, err)

	svc := &billing.Service{Pool: pools.App}
	return &fix{svc: svc, orgID: org.ID}, func() {
		pools.Close()
		_ = pgC.Terminate(ctx)
	}
}

// Escenario 1: planes seed Free/Pro/Enterprise.
func TestBilling_PlansSeeded(t *testing.T) {
	f, cleanup := setupBilling(t)
	defer cleanup()
	ctx := context.Background()

	free, err := f.svc.GetPlan(ctx, "free")
	require.NoError(t, err)
	require.Equal(t, "Free", free.Name)
	require.NotNil(t, free.TokensPerMonth)
	require.EqualValues(t, 100000, *free.TokensPerMonth)

	pro, _ := f.svc.GetPlan(ctx, "pro")
	require.EqualValues(t, 5000000, *pro.TokensPerMonth)

	ent, _ := f.svc.GetPlan(ctx, "enterprise")
	require.Nil(t, ent.TokensPerMonth, "enterprise = ilimitado")
}

func TestBilling_AssignPlan(t *testing.T) {
	f, cleanup := setupBilling(t)
	defer cleanup()
	ctx := context.Background()
	require.NoError(t, f.svc.AssignPlan(ctx, f.orgID, "free"))

	limits, plan, err := f.svc.ResolveLimits(ctx, f.orgID)
	require.NoError(t, err)
	require.NotNil(t, plan)
	require.Equal(t, "free", plan.Slug)
	require.EqualValues(t, 100000, *limits.TokensPerMonth)
}

// Escenario 2: tracking de consumo
func TestBilling_IncrementTokens(t *testing.T) {
	f, cleanup := setupBilling(t)
	defer cleanup()
	ctx := context.Background()
	_ = f.svc.AssignPlan(ctx, f.orgID, "free")
	u, err := f.svc.IncrementTokens(ctx, f.orgID, 1000)
	require.NoError(t, err)
	require.EqualValues(t, 1000, u.TokensUsed)
	u2, _ := f.svc.IncrementTokens(ctx, f.orgID, 500)
	require.EqualValues(t, 1500, u2.TokensUsed)
}

// Escenario 3: soft limit warning (80%)
func TestBilling_CheckTokens_SoftLimit(t *testing.T) {
	f, cleanup := setupBilling(t)
	defer cleanup()
	ctx := context.Background()
	_ = f.svc.AssignPlan(ctx, f.orgID, "free")
	_, _ = f.svc.IncrementTokens(ctx, f.orgID, 80000)
	state, err := f.svc.CheckTokens(ctx, f.orgID, 0)
	require.NoError(t, err)
	require.True(t, state.SoftLimitHit, "80%% = soft hit")
	require.False(t, state.HardLimitHit)
}

// Escenario 4: hard limit → ErrQuotaExceeded
func TestBilling_CheckTokens_HardLimit(t *testing.T) {
	f, cleanup := setupBilling(t)
	defer cleanup()
	ctx := context.Background()
	_ = f.svc.AssignPlan(ctx, f.orgID, "free")
	_, _ = f.svc.IncrementTokens(ctx, f.orgID, 100000)
	state, err := f.svc.CheckTokens(ctx, f.orgID, 1)
	require.ErrorIs(t, err, billing.ErrQuotaExceeded)
	require.True(t, state.HardLimitHit)
}

// Enterprise = ilimitado → no hard limit nunca
func TestBilling_Enterprise_Unlimited(t *testing.T) {
	f, cleanup := setupBilling(t)
	defer cleanup()
	ctx := context.Background()
	_ = f.svc.AssignPlan(ctx, f.orgID, "enterprise")
	_, _ = f.svc.IncrementTokens(ctx, f.orgID, 10000000)
	state, err := f.svc.CheckTokens(ctx, f.orgID, 1000000)
	require.NoError(t, err)
	require.True(t, state.Unlimited)
}

// Escenario 5: reset mensual — period_start es el primer día del mes
func TestBilling_MonthlyPeriod(t *testing.T) {
	f, cleanup := setupBilling(t)
	defer cleanup()
	ctx := context.Background()
	_ = f.svc.AssignPlan(ctx, f.orgID, "free")
	u, _ := f.svc.IncrementTokens(ctx, f.orgID, 1000)
	expected := time.Date(time.Now().UTC().Year(), time.Now().UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
	require.True(t, u.PeriodStart.Equal(expected),
		"period_start debe ser primer día del mes UTC")
}

// Escenario 6: custom_limits override
func TestBilling_CustomLimitsOverride(t *testing.T) {
	f, cleanup := setupBilling(t)
	defer cleanup()
	ctx := context.Background()
	_ = f.svc.AssignPlan(ctx, f.orgID, "enterprise")
	_, err := f.svc.Pool.Exec(ctx,
		`UPDATE organizations SET custom_limits = '{"tokens_per_month": 10000000}'::jsonb
		 WHERE id = $1`, f.orgID)
	require.NoError(t, err)
	limits, _, err := f.svc.ResolveLimits(ctx, f.orgID)
	require.NoError(t, err)
	require.NotNil(t, limits.TokensPerMonth)
	require.EqualValues(t, 10000000, *limits.TokensPerMonth,
		"custom_limits debe override defaults del plan")
}

// Sabotaje: incremento atómico concurrente (UPSERT) no pierde counts
func TestSabotage_IncrementAtomic(t *testing.T) {
	f, cleanup := setupBilling(t)
	defer cleanup()
	ctx := context.Background()
	_ = f.svc.AssignPlan(ctx, f.orgID, "free")

	const N = 50
	const each = 100
	done := make(chan struct{}, N)
	for i := 0; i < N; i++ {
		go func() {
			_, _ = f.svc.IncrementTokens(ctx, f.orgID, each)
			done <- struct{}{}
		}()
	}
	for i := 0; i < N; i++ {
		<-done
	}
	u, err := f.svc.GetUsage(ctx, f.orgID)
	require.NoError(t, err)
	require.EqualValues(t, N*each, u.TokensUsed,
		"upsert ON CONFLICT debe acumular atómicamente (sin lost updates)")
}
