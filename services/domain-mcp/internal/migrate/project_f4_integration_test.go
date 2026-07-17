//go:build integration



package migrate_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	dmigrate "nunezlagos/domain/internal/migrate"
)

func TestMigrate_Up_ProjectCurrentBranchColumn(t *testing.T) {
	dsn, cleanup := setupPG(t)
	defer cleanup()
	require.NoError(t, dmigrate.Up(dsn))

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	defer conn.Close(ctx)

	var exists bool
	err = conn.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'projects' AND column_name = 'current_branch'
		)
	`).Scan(&exists)
	require.NoError(t, err)
	require.True(t, exists, "columna current_branch debe existir en projects")
}

func TestMigrate_Up_ProjectRulesColumn(t *testing.T) {
	dsn, cleanup := setupPG(t)
	defer cleanup()
	require.NoError(t, dmigrate.Up(dsn))

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	defer conn.Close(ctx)

	var dataType string
	err = conn.QueryRow(ctx, `
		SELECT data_type FROM information_schema.columns
		WHERE table_name = 'projects' AND column_name = 'rules'
	`).Scan(&dataType)
	require.NoError(t, err)
	require.Equal(t, "ARRAY", dataType, "rules debe ser ARRAY (text[])")
}

func TestMigrate_Up_ProjectRulesDefaultEmpty(t *testing.T) {
	dsn, cleanup := setupPG(t)
	defer cleanup()
	require.NoError(t, dmigrate.Up(dsn))

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	defer conn.Close(ctx)

	var rules []string
	err = conn.QueryRow(ctx, `
		INSERT INTO projects (name, slug)
		VALUES ('P', 'p')
		RETURNING rules
	`).Scan(&rules)
	require.NoError(t, err)
	require.Empty(t, rules, "rules por default debe ser array vacío")
}

func TestMigrate_Down_ProjectRemovesColumns(t *testing.T) {
	dsn, cleanup := setupPG(t)
	defer cleanup()



	require.NoError(t, dmigrate.MigrateTo(dsn, 87))
	require.NoError(t, dmigrate.Down(dsn, 1))

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	defer conn.Close(ctx)

	var count int
	err = conn.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.columns
		WHERE table_name = 'projects' AND column_name IN ('current_branch', 'rules')
	`).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count, "rollback debe quitar ambas columnas")
}
