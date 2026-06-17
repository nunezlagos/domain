//go:build integration

package cost_test

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
	costsvc "nunezlagos/domain/internal/service/cost"
)

func setupCost(t *testing.T) (*costsvc.Service, uuid.UUID, func()) {
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

	org, _, err := seedOrgUser(ctx, pools.App, "CostOrg", "costorg", "c@x.com", "C")
	require.NoError(t, err)

	svc := &costsvc.Service{Pool: pools.App}

	// Seed cost_logs: 3 días, 2 providers/models
	seed := []struct {
		daysAgo  int
		provider string
		model    string
		cost     float64
	}{
		{0, "anthropic", "claude-sonnet-4-6", 1.50},
		{0, "openai", "gpt-4o", 0.50},
		{1, "anthropic", "claude-sonnet-4-6", 2.00},
		{2, "openai", "gpt-4o", 1.00},
	}
	for _, s := range seed {
		_, err := pools.App.Exec(ctx, `
			INSERT INTO cost_logs (organization_id, provider, model, operation,
			  tokens_input, tokens_output, cost_usd, occurred_at)
			VALUES ($1, $2, $3, 'completion', 100, 50, $4, NOW() - make_interval(days => $5))`,
			org.ID, s.provider, s.model, s.cost, s.daysAgo)
		require.NoError(t, err)
	}

	return svc, org.ID, func() {
		pools.Close()
		_ = pgC.Terminate(ctx)
	}
}

func TestSpend_DailyAndMonthly(t *testing.T) {
	svc, orgID, cleanup := setupCost(t)
	defer cleanup()
	ctx := context.Background()

	daily, err := svc.Spend(ctx, orgID, "daily", 7)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(daily), 3, "3 días con gasto")

	var total float64
	for _, b := range daily {
		total += b.CostUSD
	}
	require.InDelta(t, 5.0, total, 0.001)

	monthly, err := svc.Spend(ctx, orgID, "monthly", 30)
	require.NoError(t, err)
	require.NotEmpty(t, monthly)

	_, err = svc.Spend(ctx, orgID, "hourly", 7)
	require.ErrorIs(t, err, costsvc.ErrInvalidGranularity)
}

func TestBreakdown_ByModelAndProvider(t *testing.T) {
	svc, orgID, cleanup := setupCost(t)
	defer cleanup()
	ctx := context.Background()

	byModel, err := svc.Breakdown(ctx, orgID, "model", 7)
	require.NoError(t, err)
	require.Len(t, byModel, 2)
	require.Equal(t, "claude-sonnet-4-6", byModel[0].Key, "ordenado por cost DESC")
	require.InDelta(t, 3.5, byModel[0].CostUSD, 0.001)

	byProvider, err := svc.Breakdown(ctx, orgID, "provider", 7)
	require.NoError(t, err)
	require.Len(t, byProvider, 2)

	_, err = svc.Breakdown(ctx, orgID, "moneda", 7)
	require.ErrorIs(t, err, costsvc.ErrInvalidDimension)
}

func TestForecast_WithAndWithoutData(t *testing.T) {
	svc, orgID, cleanup := setupCost(t)
	defer cleanup()
	ctx := context.Background()

	f, err := svc.ForecastSMA(ctx, orgID, 14)
	require.NoError(t, err)
	require.Greater(t, f.AvgDailyUSD, 0.0, "hay gasto en la ventana (días 1 y 2)")

	// Org sin datos → 0 sin panic (sabotaje del spec)
	empty, err := svc.ForecastSMA(ctx, uuid.New(), 14)
	require.NoError(t, err)
	require.Zero(t, empty.AvgDailyUSD)
	require.Zero(t, empty.MonthEndUSD)
}

func TestBudgets_CRUDAndStatus(t *testing.T) {
	svc, orgID, cleanup := setupCost(t)
	defer cleanup()
	ctx := context.Background()

	// Budget chico → exceeded (gasto MTD ≥ seed de hoy 2.0)
	small, err := svc.CreateBudget(ctx, orgID, "ajustado", 1.0, "monthly", 80)
	require.NoError(t, err)
	// Budget grande → ok
	_, err = svc.CreateBudget(ctx, orgID, "holgado", 1000.0, "monthly", 80)
	require.NoError(t, err)

	budgets, err := svc.ListBudgets(ctx, orgID)
	require.NoError(t, err)
	require.Len(t, budgets, 2)
	byName := map[string]costsvc.Budget{}
	for _, b := range budgets {
		byName[b.Name] = b
		require.GreaterOrEqual(t, b.CurrentSpendUSD, 0.0)
	}
	require.Equal(t, "exceeded", byName["ajustado"].Status)
	require.Equal(t, "ok", byName["holgado"].Status)

	// Delete + cross-org guard
	require.NoError(t, svc.DeleteBudget(ctx, orgID, small.ID))
	require.ErrorIs(t, svc.DeleteBudget(ctx, orgID, small.ID), costsvc.ErrBudgetNotFound)
	budgets, err = svc.ListBudgets(ctx, orgID)
	require.NoError(t, err)
	require.Len(t, budgets, 1)
	require.ErrorIs(t, svc.DeleteBudget(ctx, uuid.New(), budgets[0].ID), costsvc.ErrBudgetNotFound)
}

func TestExportCSV_Formats(t *testing.T) {
	svc, orgID, cleanup := setupCost(t)
	defer cleanup()
	ctx := context.Background()

	spend, err := svc.ExportCSV(ctx, orgID, "spend", 7)
	require.NoError(t, err)
	require.Equal(t, []string{"date", "runs", "cost_usd"}, spend[0])
	require.GreaterOrEqual(t, len(spend), 4, "header + 3 días")

	breakdown, err := svc.ExportCSV(ctx, orgID, "breakdown", 7)
	require.NoError(t, err)
	require.Equal(t, []string{"model", "runs", "cost_usd"}, breakdown[0])
	require.Len(t, breakdown, 3, "header + 2 modelos")

	_, err = svc.ExportCSV(ctx, orgID, "pdf", 7)
	require.Error(t, err)
}
