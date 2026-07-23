//go:build integration

package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	dmigrate "nunezlagos/domain/internal/migrate"
)

// TestSchemaDrift_PrimaryMigrado_SinDrift ejercita el binario E2E contra un
// postgres de testcontainers (el superuser SÍ puede CREATE DATABASE): migra el
// primary, arma el expected desde las mismas migraciones en una temp DB y
// verifica que no hay drift. Smoke/regresión del binario (DOMAINSERV-88): la
// detección contra prod viva es follow-up (app_admin del VPS sin CREATE DATABASE).
func TestSchemaDrift_PrimaryMigrado_SinDrift(t *testing.T) {
	ctx := context.Background()
	pgC, err := postgres.Run(ctx,
		"pgvector/pgvector:pg16",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
	)
	require.NoError(t, err)
	defer func() { _ = pgC.Terminate(ctx) }()

	dsn, _ := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, dmigrate.Up(dsn), "migrar el primary")

	primary, err := dumpSchema(ctx, dsn)
	require.NoError(t, err)

	const tmp = "drift_check_tmp"
	require.NoError(t, createTempDB(ctx, dsn, tmp))
	defer dropTempDB(context.Background(), dsn, tmp)

	tmpDSN := replaceDBName(dsn, tmp)
	require.NoError(t, applyMigrations(tmpDSN), "migrar la temp DB (expected)")

	expected, err := dumpSchema(ctx, tmpDSN)
	require.NoError(t, err)

	diff := diffSchemas(primary, expected)
	require.Empty(t, diff, "primary migrado debe coincidir con el expected de migraciones (sin drift)")
}
