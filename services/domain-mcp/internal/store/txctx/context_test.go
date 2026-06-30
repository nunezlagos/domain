package txctx_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/store/txctx"
)

// mockTx es un stub que satisface pgx.Tx sin abrir conexión real.
// Solo necesitamos que el type assertion en TxFromContext funcione;
// los métodos no se invocan en estos tests unit (round-trip puro).
type mockTx struct{ id int }

func (mockTx) Begin(ctx context.Context) (pgx.Tx, error)                       { return nil, nil }
func (mockTx) BeginFunc(ctx context.Context, f func(pgx.Tx) error) error      { return nil }
func (mockTx) Commit(ctx context.Context) error                              { return nil }
func (mockTx) Rollback(ctx context.Context) error                            { return nil }
func (mockTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (mockTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return nil }
func (mockTx) LargeObjects() pgx.LargeObjects                          { return pgx.LargeObjects{} }
func (mockTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (mockTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (mockTx) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, nil }
func (mockTx) QueryRow(context.Context, string, ...any) pgx.Row        { return nil }
func (mockTx) Conn() *pgx.Conn                                          { return nil }

func TestWithTxContext_RoundTrip(t *testing.T) {
	ctx := context.Background()
	require.Nil(t, txctx.TxFromContext(ctx), "ctx vacío no debe tener tx")

	tx := mockTx{id: 42}
	ctx2 := txctx.WithTxContext(ctx, tx)
	got := txctx.TxFromContext(ctx2)
	require.NotNil(t, got, "tx inyectada debe estar presente")

	gotMock, ok := got.(mockTx)
	require.True(t, ok, "type assertion a mockTx debe funcionar")
	require.Equal(t, 42, gotMock.id, "tx inyectada debe ser la misma instance (mismo id)")


	require.Nil(t, txctx.TxFromContext(ctx), "ctx original no debe mutarse")
}

func TestTxFromContext_NilSafe(t *testing.T) {
	require.NotPanics(t, func() {
		var nilCtx context.Context //nolint:staticcheck
		_ = txctx.TxFromContext(nilCtx)
	})
	require.NotPanics(t, func() {
		_ = txctx.TxFromContext(context.WithValue(context.Background(), "otra", "cosa"))
	})
}

func TestMustTxFromContext_Missing(t *testing.T) {
	_, err := txctx.MustTxFromContext(context.Background())
	require.Error(t, err)
	require.True(t, errors.Is(err, txctx.ErrNoTxInContext))
}

func TestMustTxFromContext_Present(t *testing.T) {
	tx := mockTx{id: 7}
	ctx := txctx.WithTxContext(context.Background(), tx)
	got, err := txctx.MustTxFromContext(ctx)
	require.NoError(t, err)
	require.NotNil(t, got)
	gotMock := got.(mockTx)
	require.Equal(t, 7, gotMock.id)
}

func TestWithTxContext_OverwriteReturnsLatest(t *testing.T) {
	tx1 := mockTx{id: 1}
	tx2 := mockTx{id: 2}
	ctx := txctx.WithTxContext(context.Background(), tx1)
	ctx = txctx.WithTxContext(ctx, tx2)
	got := txctx.TxFromContext(ctx)
	gotMock := got.(mockTx)
	require.Equal(t, 2, gotMock.id,
		"WithTxContext debe reemplazar, no anidar (evita leaks de tx vieja)")
}
