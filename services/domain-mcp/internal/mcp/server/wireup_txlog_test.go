package mcpserver

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

// TestSQLErrorCaptureTracer_HappyPath verifica que el tracer se comporta
// como no-op cuando las queries succeed. No usa DB real — solo valida
// la lógica de captura en aislamiento.
func TestSQLErrorCaptureTracer_HappyPath(t *testing.T) {
	ctx, log := withSQLErrorLog(context.Background())
	require.NotNil(t, log)

	tr := sqlErrorCaptureTracer{}
	// Query ok
	ctx2 := tr.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{SQL: "SELECT 1"})
	tr.TraceQueryEnd(ctx2, nil, pgx.TraceQueryEndData{Err: nil})

	errs, sqls := log.Snapshot()
	require.Empty(t, errs, "queries exitosas no deben contaminar el log")
	require.Empty(t, sqls)
}

// TestSQLErrorCaptureTracer_RecordsError verifica que el tracer captura
// el (err, sql) cuando la query falla. Validates el flujo base del fix.
func TestSQLErrorCaptureTracer_RecordsError(t *testing.T) {
	ctx, log := withSQLErrorLog(context.Background())

	tr := sqlErrorCaptureTracer{}
	ctx2 := tr.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{
		SQL:  "SELECT 1 FROM table_that_does_not_exist",
		Args: nil,
	})
	tr.TraceQueryEnd(ctx2, nil, pgx.TraceQueryEndData{
		Err: &pgconn.PgError{Code: "42P01", Message: "relation does not exist"},
	})

	errs, sqls := log.Snapshot()
	require.Len(t, errs, 1)
	require.Contains(t, errs[0].Error(), "relation does not exist")
	require.Equal(t, "SELECT 1 FROM table_that_does_not_exist", sqls[0])
}

// TestSQLErrorCaptureTracer_MultipleErrorsRecordedInOrder verifica que el log
// preserva TODOS los errores en orden de aparicion. Caso de uso principal:
// la primera query falla con error real y las siguientes con 25P02 por
// cascade; el operador ve la causa raiz (primera) y el sintoma (resto).
func TestSQLErrorCaptureTracer_MultipleErrorsRecordedInOrder(t *testing.T) {
	ctx, log := withSQLErrorLog(context.Background())
	tr := sqlErrorCaptureTracer{}

	c1 := tr.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{SQL: "Q1"})
	tr.TraceQueryEnd(c1, nil, pgx.TraceQueryEndData{Err: errors.New("Q1 failed")})

	c2 := tr.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{SQL: "Q2"})
	tr.TraceQueryEnd(c2, nil, pgx.TraceQueryEndData{Err: errors.New("Q2 failed")})

	errs, sqls := log.Snapshot()
	require.Len(t, errs, 2, "debe preservar ambos errores en orden")
	require.EqualError(t, errs[0], "Q1 failed", "primer error es la causa raiz")
	require.EqualError(t, errs[1], "Q2 failed", "segundo es el cascade")
	require.Equal(t, "Q1", sqls[0])
	require.Equal(t, "Q2", sqls[1])
}

// TestSQLErrorCaptureTracer_NilContextSafe verifica que el tracer es
// seguro aunque el ctx no tenga SQLErrorLog (p.ej. queries contra el Pool
// directo, fuera del withOrgCtx wireup).
func TestSQLErrorCaptureTracer_NilContextSafe(t *testing.T) {
	tr := sqlErrorCaptureTracer{}
	// Ctx sin withSQLErrorLog — el tracer debe no-op, no panic.
	ctx2 := tr.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{SQL: "SELECT 1"})
	tr.TraceQueryEnd(ctx2, nil, pgx.TraceQueryEndData{Err: errors.New("boom")})
	// Sin panic = pass.
}

// TestSQLErrorCaptureTracer_CascadeRecordsPrimaryFirst reproduce el caso real
// del bug: una query falla con 42P01 (relation does not exist) y las siguientes
// con 25P02 por tx aborted. El log debe preservar AMBOS en orden; el primero
// es la causa raiz y el wireup debe surfacearlo para diagnostico.
func TestSQLErrorCaptureTracer_CascadeRecordsPrimaryFirst(t *testing.T) {
	ctx, log := withSQLErrorLog(context.Background())
	tr := sqlErrorCaptureTracer{}

	c1 := tr.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{SQL: "SELECT * FROM table_dropped"})
	tr.TraceQueryEnd(c1, nil, pgx.TraceQueryEndData{
		Err: &pgconn.PgError{Code: "42P01", Message: `relation "table_dropped" does not exist`},
	})

	c2 := tr.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{SQL: "SELECT 2"})
	tr.TraceQueryEnd(c2, nil, pgx.TraceQueryEndData{
		Err: &pgconn.PgError{Code: "25P02", Message: "current transaction is aborted"},
	})

	errs, sqls := log.Snapshot()
	require.Len(t, errs, 2, "debe preservar ambos errores, no solo el ultimo")
	require.Contains(t, errs[0].Error(), "42P01", "el primer error es la causa raiz real")
	require.Contains(t, errs[1].Error(), "25P02", "el segundo es el cascade")
	require.Equal(t, "SELECT * FROM table_dropped", sqls[0])
	require.Equal(t, "SELECT 2", sqls[1])
}

// TestSQLErrorCaptureTracer_EmptySnapshotIsIterationSafe verifica que cuando
// ninguna query fallo, Snapshot() devuelve algo sobre lo que se puede hacer
// len()/range sin panic. nil slice y [] slice son equivalentes en Go para
// estos usos; el wireup hace `len(errs) > 0` y eso funciona en ambos casos.
func TestSQLErrorCaptureTracer_EmptySnapshotIsIterationSafe(t *testing.T) {
	_, log := withSQLErrorLog(context.Background())
	errs, sqls := log.Snapshot()
	require.Empty(t, errs)
	require.Empty(t, sqls)
	n := 0
	for range errs {
		n++
	}
	require.Zero(t, n, "range sobre errs vacio no debe iterar")
}

// TestTruncateSQL verifica el helper para mensajes de error.
func TestTruncateSQL(t *testing.T) {
	require.Equal(t, "abc", truncateSQL("abc", 100))
	long := make([]byte, 300)
	for i := range long {
		long[i] = 'x'
	}
	out := truncateSQL(string(long), 50)
	require.Len(t, out, 50+len("... (truncated)"))
}
