//go:build integration

// issue-21.6 Fase B: integration tests del threshold checker (covers S1.1
// cost_alerts_sent + S1.2 org_cost_alert_thresholds). Valida el path crítico
// de alertas de costo en single-org: lee threshold → compara vs cost_logs →
// inserta en cost_alerts_sent con UNIQUE(alert_date) anti-spam.

package usagealerts

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

// setupThresholdsDB levanta PG, migra y devuelve un pool listo para usar.
// NO inserta organización: cost_alerts_sent y org_cost_alert_thresholds son
// single-org global (sin FK obligatoria a organizations).
func setupThresholdsDB(t *testing.T) (*pgxpool.Pool, func()) {
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

func TestGetCostThreshold_DefaultIfEmpty(t *testing.T) {
	pool, cleanup := setupThresholdsDB(t)
	defer cleanup()

	got, err := GetCostThreshold(context.Background(), pool, uuid.Nil)
	require.NoError(t, err)
	require.Equal(t, 100.00, got, "sin row → default 100.00")
}

func TestEnableCostThreshold_CreatesIfMissing(t *testing.T) {
	pool, cleanup := setupThresholdsDB(t)
	defer cleanup()
	ctx := context.Background()

	require.NoError(t, EnableCostThreshold(ctx, pool, uuid.Nil, 250.50))
	got, err := GetCostThreshold(ctx, pool, uuid.Nil)
	require.NoError(t, err)
	require.Equal(t, 250.50, got)
}

func TestEnableCostThreshold_UpdatesIfExists(t *testing.T) {
	pool, cleanup := setupThresholdsDB(t)
	defer cleanup()
	ctx := context.Background()

	require.NoError(t, EnableCostThreshold(ctx, pool, uuid.Nil, 100))
	require.NoError(t, EnableCostThreshold(ctx, pool, uuid.Nil, 500))
	got, err := GetCostThreshold(ctx, pool, uuid.Nil)
	require.NoError(t, err)
	require.Equal(t, 500.00, got, "segundo Enable sobreescribe (UPDATE+INSERT NOT EXISTS)")
}

func TestCheckThresholds_NoLogsNoAlert(t *testing.T) {
	pool, cleanup := setupThresholdsDB(t)
	defer cleanup()
	ctx := context.Background()

	// Threshold muy bajo, pero no hay cost_logs → no debe alertar.
	require.NoError(t, EnableCostThreshold(ctx, pool, uuid.Nil, 0.01))

	alerts, err := CheckThresholds(ctx, pool)
	require.NoError(t, err)
	require.Empty(t, alerts)
}

func TestCheckThresholds_AtOrAboveThresholdAlerts(t *testing.T) {
	pool, cleanup := setupThresholdsDB(t)
	defer cleanup()
	ctx := context.Background()

	require.NoError(t, EnableCostThreshold(ctx, pool, uuid.Nil, 10.00))
	now := time.Now().UTC()
	_, err := pool.Exec(ctx,
		`INSERT INTO cost_logs (provider, model, cost_usd, created_at)
		 VALUES ($1, $2, $3, $4)`,
		"anthropic", "claude-sonnet", 15.50, now)
	require.NoError(t, err)

	alerts, err := CheckThresholds(ctx, pool)
	require.NoError(t, err)
	require.Len(t, alerts, 1)
	require.Equal(t, 15.50, alerts[0].TotalUSD)
	require.Equal(t, 10.00, alerts[0].ThresholdUSD)
}

func TestSendAlerts_InsertsOnce_DedupesByAlertDate(t *testing.T) {
	pool, cleanup := setupThresholdsDB(t)
	defer cleanup()
	ctx := context.Background()

	date := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	alerts := []CostAlert{{
		TotalUSD:     50.50,
		ThresholdUSD: 50.00,
		AlertDate:    date,
	}}

	// sender nil → no manda email, pero sí inserta en cost_alerts_sent.
	sent, err := SendAlerts(ctx, pool, alerts, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 1, sent)

	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM cost_alerts_sent WHERE alert_date = $1`, date,
	).Scan(&count))
	require.Equal(t, 1, count, "primera vez inserta")

	// Segundo intento mismo día: ON CONFLICT(alert_date) DO NOTHING → no inserta.
	sent, err = SendAlerts(ctx, pool, alerts, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 0, sent, "segunda vez con mismo date no inserta (anti-spam)")

	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM cost_alerts_sent WHERE alert_date = $1`, date,
	).Scan(&count))
	require.Equal(t, 1, count, "sigue habiendo 1 row (la UNIQUE constraint funciona)")
}

func TestSendAlerts_DifferentDatesInsertSeparately(t *testing.T) {
	pool, cleanup := setupThresholdsDB(t)
	defer cleanup()
	ctx := context.Background()

	d1 := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)

	_, err := SendAlerts(ctx, pool, []CostAlert{{AlertDate: d1, TotalUSD: 10}}, nil, nil)
	require.NoError(t, err)
	_, err = SendAlerts(ctx, pool, []CostAlert{{AlertDate: d2, TotalUSD: 20}}, nil, nil)
	require.NoError(t, err)

	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM cost_alerts_sent`,
	).Scan(&count))
	require.Equal(t, 2, count, "fechas distintas → 2 rows (UNIQUE solo por date)")
}
