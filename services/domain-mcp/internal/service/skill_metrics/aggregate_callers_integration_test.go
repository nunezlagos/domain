//go:build integration

package skill_metrics

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

func setupDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
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
	dsn, _ := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, dmigrate.Up(dsn))
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	return pool, func() {
		pool.Close()
		_ = pgC.Terminate(ctx)
	}
}

// seedUser inserta un usuario (single-tenant: users NO tiene organization_id tras
// 000142) y devuelve su id, para usarlo como created_by de skill_executions.
func seedUser(t *testing.T, pool *pgxpool.Pool, email string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(),
		`INSERT INTO users (email, name, role) VALUES ($1, $2, 'viewer') RETURNING id`,
		email, email,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func seedMetricsSkill(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(),
		`INSERT INTO skills (slug, name, description, skill_type, content, seed_managed)
		 VALUES ('uc-skill', 'uc', 'uc', 'prompt', 'x', false) RETURNING id`,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// seedExec inserta una ejecución contable (completed/exitosa) con un caller dado
// (nil => created_by NULL) en el día indicado.
func seedExec(t *testing.T, pool *pgxpool.Pool, skillID uuid.UUID, caller *uuid.UUID, day time.Time) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO skill_executions
		   (skill_id, mode, status, parameters, output, execution_time_ms, created_by, created_at)
		 VALUES ($1, 'sync', 'completed', '{}'::jsonb, 'ok', 100, $2, $3)`,
		skillID, caller, day,
	)
	require.NoError(t, err)
}

// TestAggregateDay_UniqueCallers verifica el COUNT(DISTINCT created_by) real
// (HU-52.2 deuda): cuenta callers distintos NO nulos sobre las invocaciones
// contables del día; las filas con created_by NULL (cron/webhook de sistema o
// históricas previas a 000184) NO suman.
func TestAggregateDay_UniqueCallers(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	repo := NewPgRepository(pool)
	skillID := seedMetricsSkill(t, pool)

	u1 := seedUser(t, pool, "u1@example.com")
	u2 := seedUser(t, pool, "u2@example.com")
	day := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

	// u1 dos veces, u2 una vez, dos ejecuciones de sistema (NULL).
	seedExec(t, pool, skillID, &u1, day)
	seedExec(t, pool, skillID, &u1, day)
	seedExec(t, pool, skillID, &u2, day)
	seedExec(t, pool, skillID, nil, day)
	seedExec(t, pool, skillID, nil, day)

	res, err := repo.AggregateDay(ctx, skillID, day)
	require.NoError(t, err)
	require.Equal(t, 5, res.InvocationsCount)
	require.Equal(t, 2, res.UniqueCallersCount, "u1 y u2 distintos; NULLs no cuentan")
}

// TestAggregateDay_AllNullCallers: sin ningún caller no-nulo, unique_callers=0.
func TestAggregateDay_AllNullCallers(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	repo := NewPgRepository(pool)
	skillID := seedMetricsSkill(t, pool)
	day := time.Date(2026, 6, 11, 8, 0, 0, 0, time.UTC)

	seedExec(t, pool, skillID, nil, day)
	seedExec(t, pool, skillID, nil, day)

	res, err := repo.AggregateDay(ctx, skillID, day)
	require.NoError(t, err)
	require.Equal(t, 2, res.InvocationsCount)
	require.Equal(t, 0, res.UniqueCallersCount, "todas las ejecuciones de sistema -> 0 callers")
}
