//go:build integration

package systemcron_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	dmigrate "nunezlagos/domain/internal/migrate"
	systemcron "nunezlagos/domain/internal/scheduler/cron/system"
)

func setupAuthDB(t *testing.T) (*pgxpool.Pool, func()) {
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
	return pool, func() { pool.Close(); _ = pgC.Terminate(ctx) }
}

// TestAuthAnomalyAuditor_Audit_DetectsBruteForce: 6 fallos mismo email+ip y 5
// fallos apikey (email NULL) mismo ip superan el umbral; éxitos y clusters
// bajo umbral se excluyen.
func TestAuthAnomalyAuditor_Audit_DetectsBruteForce(t *testing.T) {
	pool, cleanup := setupAuthDB(t)
	defer cleanup()
	ctx := context.Background()

	ins := func(email string, success bool, ip string) {
		_, e := pool.Exec(ctx,
			`INSERT INTO auth_events (kind, email_attempted, success, ip)
			 VALUES ('login_attempt', NULLIF($1,''), $2, NULLIF($3,'')::inet)`,
			email, success, ip)
		require.NoError(t, e)
	}
	for i := 0; i < 6; i++ {
		ins("victim@x.com", false, "10.0.0.1")
	}
	for i := 0; i < 5; i++ {
		ins("", false, "10.0.0.2")
	}
	ins("victim@x.com", true, "10.0.0.1") // éxito → excluido (success=FALSE)
	for i := 0; i < 3; i++ {
		ins("low@x.com", false, "10.0.0.3") // 3 < umbral 5 → excluido
	}

	auditor := &systemcron.AuthAnomalyAuditor{Pool: pool, Window: time.Hour, Threshold: 5}
	anoms, err := auditor.Audit(ctx)
	require.NoError(t, err)
	require.Len(t, anoms, 2, "solo los 2 clusters sobre umbral")

	byIP := map[string]int64{}
	hashByIP := map[string]string{}
	for _, an := range anoms {
		byIP[an.IP] = an.Count
		hashByIP[an.IP] = an.EmailHash
	}
	require.Equal(t, int64(6), byIP["10.0.0.1"])
	require.Equal(t, int64(5), byIP["10.0.0.2"])
	require.Len(t, hashByIP["10.0.0.1"], 8, "email hasheado presente")
	require.Equal(t, "", hashByIP["10.0.0.2"], "apikey failure sin email → hash vacío")
}
