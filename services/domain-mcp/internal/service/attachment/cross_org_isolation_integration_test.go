//go:build integration

package attachment_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/service/attachment"
	"nunezlagos/domain/internal/store/txctx"
)

// setupRLS levanta una Postgres migrada y conecta con el rol app_user (NOBYPASSRLS),
// para que las RLS policies de file_attachments (000274) apliquen de verdad. Con el
// superusuario de testcontainers RLS quedaría bypasseado y el test no probaría nada.
func setupRLS(t *testing.T) (*pgxpool.Pool, func()) {
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

	cfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)

	bootstrap, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	_, err = bootstrap.Exec(ctx, `GRANT app_user TO test`)
	require.NoError(t, err)
	bootstrap.Close()

	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, `SET ROLE app_user`)
		return err
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)

	return pool, func() {
		pool.Close()
		_ = pgC.Terminate(ctx)
	}
}

// insertAttachment inserta un adjunto scopeado a org: el DEFAULT current_org_id()
// de file_attachments toma el app.current_org_id que setea WithOrgTx.
func insertAttachment(t *testing.T, pool *pgxpool.Pool, org, entityID uuid.UUID, s3Key string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := txctx.WithOrgTx(context.Background(), pool, org, func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(),
			`INSERT INTO file_attachments (entity_type, entity_id, filename, s3_key, size_bytes, mime_type)
			 VALUES ('ticket', $1, 'f.txt', $2, 10, 'text/plain') RETURNING id`,
			entityID, s3Key).Scan(&id)
	})
	require.NoError(t, err)
	return id
}

// TestAttachment_CrossOrg_IsolatedByRLS_ReturnsNotFound verifica que un adjunto de
// otra org es invisible/intocable aunque se conozca su UUID (IDOR cerrado por RLS,
// DOMAINSERV-112). S3 queda nil a propósito: RLS corta antes de cualquier llamada a S3.
func TestAttachment_CrossOrg_IsolatedByRLS_ReturnsNotFound(t *testing.T) {
	pool, cleanup := setupRLS(t)
	defer cleanup()
	ctx := context.Background()

	orgA, orgB := uuid.New(), uuid.New()
	entityID := uuid.New() // misma entidad: prueba que ListByEntity no filtra cross-org

	attA := insertAttachment(t, pool, orgA, entityID, "attachments/ticket/a.txt")
	attB := insertAttachment(t, pool, orgB, entityID, "attachments/ticket/b.txt")

	svc := &attachment.Service{Pool: pool}

	// Bajo la org A: ve el suyo, nunca el de B.
	err := txctx.WithOrgTx(ctx, pool, orgA, func(tx pgx.Tx) error {
		cctx := txctx.WithTxContext(ctx, tx)

		items, e := svc.ListByEntity(cctx, "ticket", entityID.String())
		require.NoError(t, e)
		require.Len(t, items, 1, "org A solo ve su propio adjunto en la entidad compartida")
		require.Equal(t, attA, items[0].ID)

		_, e = svc.GetDownloadURL(cctx, attB)
		require.ErrorIs(t, e, attachment.ErrNotFound, "org A no puede get_url del adjunto de org B")

		e = svc.Delete(cctx, attB)
		require.ErrorIs(t, e, attachment.ErrNotFound, "org A no puede borrar el adjunto de org B")
		return nil
	})
	require.NoError(t, err)

	// El adjunto de B sigue vivo: el delete cross-org no lo tocó.
	err = txctx.WithOrgTx(ctx, pool, orgB, func(tx pgx.Tx) error {
		cctx := txctx.WithTxContext(ctx, tx)
		items, e := svc.ListByEntity(cctx, "ticket", entityID.String())
		require.NoError(t, e)
		require.Len(t, items, 1)
		require.Equal(t, attB, items[0].ID, "el adjunto de org B debe seguir existiendo")
		return nil
	})
	require.NoError(t, err)
}
