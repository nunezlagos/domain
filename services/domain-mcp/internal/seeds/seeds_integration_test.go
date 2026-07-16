//go:build integration



package seeds_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/seeds"
)

func setupSeededDB(t *testing.T) (*pgxpool.Pool, func()) {
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

type counterSeeder struct {
	name    string
	version int
	calls   atomic.Int32
}

func (c *counterSeeder) Name() string  { return c.name }
func (c *counterSeeder) Version() int  { return c.version }
func (c *counterSeeder) Order() int    { return 1 }
func (c *counterSeeder) IsDevOnly() bool { return false }
func (c *counterSeeder) Run(ctx context.Context, tx pgx.Tx, env seeds.Env) (seeds.Report, error) {
	c.calls.Add(1)
	return seeds.Report{Created: 1}, nil
}

// failingSeeder falla siempre, con Order menor que counterSeeder (1).
type failingSeeder struct{ name string }

func (f *failingSeeder) Name() string    { return f.name }
func (f *failingSeeder) Version() int    { return 1 }
func (f *failingSeeder) Order() int      { return 0 }
func (f *failingSeeder) IsDevOnly() bool { return false }
func (f *failingSeeder) Run(ctx context.Context, tx pgx.Tx, env seeds.Env) (seeds.Report, error) {
	return seeds.Report{}, errors.New("boom")
}

// DOMAINSERV-28: un seeder que falla (Order menor) NO impide correr los de Order
// mayor; el error queda en su Report y RunAll no aborta la cadena.
func TestSeeds_RunAll_SeederFalla_LosDeOrderMayorSiCorren(t *testing.T) {
	pool, cleanup := setupSeededDB(t)
	defer cleanup()
	ctx := context.Background()
	reg := seeds.NewRegistry()
	failing := &failingSeeder{name: "failing_seed"}
	ok := &counterSeeder{name: "ok_seed", version: 1}
	reg.Register(failing)
	reg.Register(ok)

	reports, err := reg.RunAll(ctx, pool, seeds.EnvProd)

	require.NoError(t, err, "el fallo de un seeder ya no aborta la cadena")
	require.Equal(t, int32(1), ok.calls.Load(), "el seeder de Order mayor corrió pese al fallo previo")
	require.NotEmpty(t, reports["failing_seed"].Errors, "el fallo se registra en el reporte del seeder roto")
}

// DOMAINSERV-36: cuando otro proceso tiene el advisory lock, RunAll NO es un
// fallo — devuelve el sentinel ErrSeedLockHeld (no-op benigno) y no corre los
// seeders. El llamador (container domain-seed) debe salir exit 0, no exit 1.
func TestSeeds_RunAll_LockTomado_DevuelveSentinelBenigno(t *testing.T) {
	pool, cleanup := setupSeededDB(t)
	defer cleanup()
	ctx := context.Background()

	holder, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer holder.Release()
	var locked bool
	require.NoError(t, holder.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", seeds.SeedLockID).Scan(&locked))
	require.True(t, locked, "el holder debe tomar el lock primero")

	reg := seeds.NewRegistry()
	s := &counterSeeder{name: "test_seed_locked", version: 1}
	reg.Register(s)

	reports, err := reg.RunAll(ctx, pool, seeds.EnvProd)

	require.ErrorIs(t, err, seeds.ErrSeedLockHeld, "perder el lock devuelve el sentinel benigno")
	require.Empty(t, reports, "no se corre ningún seeder mientras otro proceso tiene el lock")
	require.Equal(t, int32(0), s.calls.Load(), "el seeder no se ejecutó")
}

// Escenario: RunAll ejecuta primer seed.
func TestSeeds_RunAll_FirstRun(t *testing.T) {
	pool, cleanup := setupSeededDB(t)
	defer cleanup()
	ctx := context.Background()
	reg := seeds.NewRegistry()
	s := &counterSeeder{name: "test_seed", version: 1}
	reg.Register(s)

	reports, err := reg.RunAll(ctx, pool, seeds.EnvDev)
	require.NoError(t, err)
	require.Equal(t, 1, reports["test_seed"].Created)
	require.Equal(t, int32(1), s.calls.Load())


	v, ok, err := seeds.AppliedVersion(ctx, pool, "test_seed")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 1, v)
}

// Escenario: re-run idempotente (no llama Run del seeder de nuevo si version igual).
func TestSeeds_RunAll_Idempotent(t *testing.T) {
	pool, cleanup := setupSeededDB(t)
	defer cleanup()
	ctx := context.Background()
	reg := seeds.NewRegistry()
	s := &counterSeeder{name: "test_seed", version: 1}
	reg.Register(s)

	_, err := reg.RunAll(ctx, pool, seeds.EnvDev)
	require.NoError(t, err)
	_, err = reg.RunAll(ctx, pool, seeds.EnvDev)
	require.NoError(t, err)

	require.Equal(t, int32(1), s.calls.Load(), "no debe llamar Run dos veces si version igual")
}

// Escenario: bump version → re-aplica.
func TestSeeds_RunAll_VersionBump_Reapplies(t *testing.T) {
	pool, cleanup := setupSeededDB(t)
	defer cleanup()
	ctx := context.Background()
	reg := seeds.NewRegistry()
	s := &counterSeeder{name: "test_seed", version: 1}
	reg.Register(s)
	_, err := reg.RunAll(ctx, pool, seeds.EnvDev)
	require.NoError(t, err)


	s.version = 2
	_, err = reg.RunAll(ctx, pool, seeds.EnvDev)
	require.NoError(t, err)
	require.Equal(t, int32(2), s.calls.Load())

	v, _, _ := seeds.AppliedVersion(ctx, pool, "test_seed")
	require.Equal(t, 2, v)
}

// Escenario: DevOnly seeder skipped en prod.
func TestSeeds_RunAll_DevOnly_SkippedInProd(t *testing.T) {
	pool, cleanup := setupSeededDB(t)
	defer cleanup()
	ctx := context.Background()
	reg := seeds.NewRegistry()
	core := &counterSeeder{name: "core", version: 1}
	demo := &devOnlySeeder{name: "demo", version: 1}
	reg.Register(core)
	reg.Register(demo)

	reports, err := reg.RunAll(ctx, pool, seeds.EnvProd)
	require.NoError(t, err)
	require.Equal(t, int32(1), core.calls.Load(), "core debe correr en prod")
	require.Equal(t, int32(0), demo.calls.Load(), "demo dev-only NO debe correr en prod")
	require.Equal(t, 1, reports["demo"].Skipped)
}

// Escenario: DevOnly corre en dev.
func TestSeeds_RunAll_DevOnly_RunsInDev(t *testing.T) {
	pool, cleanup := setupSeededDB(t)
	defer cleanup()
	ctx := context.Background()
	reg := seeds.NewRegistry()
	demo := &devOnlySeeder{name: "demo", version: 1}
	reg.Register(demo)

	_, err := reg.RunAll(ctx, pool, seeds.EnvDev)
	require.NoError(t, err)
	require.Equal(t, int32(1), demo.calls.Load())
}

type devOnlySeeder struct {
	name    string
	version int
	calls   atomic.Int32
}

func (c *devOnlySeeder) Name() string    { return c.name }
func (c *devOnlySeeder) Version() int    { return c.version }
func (c *devOnlySeeder) Order() int      { return 2 }
func (c *devOnlySeeder) IsDevOnly() bool { return true }
func (c *devOnlySeeder) Run(ctx context.Context, tx pgx.Tx, env seeds.Env) (seeds.Report, error) {
	c.calls.Add(1)
	return seeds.Report{Created: 1}, nil
}

// Sabotaje: seeder que falla → rollback tx + no marca version.
func TestSabotage_SeederFails_NoVersionRecorded(t *testing.T) {
	pool, cleanup := setupSeededDB(t)
	defer cleanup()
	ctx := context.Background()
	reg := seeds.NewRegistry()
	bad := &errSeeder{name: "bad", version: 1}
	reg.Register(bad)

	_, err := reg.RunAll(ctx, pool, seeds.EnvDev)
	require.Error(t, err)

	v, ok, err := seeds.AppliedVersion(ctx, pool, "bad")
	require.NoError(t, err)
	require.False(t, ok, "version NO debe quedar registrada cuando seeder falla")
	require.Equal(t, 0, v)
}

type errSeeder struct {
	name    string
	version int
}

func (e *errSeeder) Name() string    { return e.name }
func (e *errSeeder) Version() int    { return e.version }
func (e *errSeeder) Order() int      { return 0 }
func (e *errSeeder) IsDevOnly() bool { return false }
func (e *errSeeder) Run(_ context.Context, _ pgx.Tx, _ seeds.Env) (seeds.Report, error) {
	return seeds.Report{}, errors.New("kaboom")
}
