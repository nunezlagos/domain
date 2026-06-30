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

	err, sql := log.Snapshot()
	require.NoError(t, err, "queries exitosas no deben contaminar el log")
	require.Empty(t, sql)
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

	err, sql := log.Snapshot()
	require.Error(t, err)
	require.Contains(t, err.Error(), "relation does not exist")
	require.Equal(t, "SELECT 1 FROM table_that_does_not_exist", sql)
}

// TestSQLErrorCaptureTracer_MultipleErroresLastWins verifica que si varias
// queries fallan (caso real: query1 falla, query2 tambien falla), el log
// retiene el ULTIMO (last-write-wins). Esto es lo que el wireup necesita
// — la ultima query que falla es la que importa para el diagnostico.
func TestSQLErrorCaptureTracer_MultipleErroresLastWins(t *testing.T) {
	ctx, log := withSQLErrorLog(context.Background())
	tr := sqlErrorCaptureTracer{}

	// Query 1 falla
	c1 := tr.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{SQL: "Q1"})
	tr.TraceQueryEnd(c1, nil, pgx.TraceQueryEndData{Err: errors.New("Q1 failed")})

	// Query 2 falla
	c2 := tr.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{SQL: "Q2"})
	tr.TraceQueryEnd(c2, nil, pgx.TraceQueryEndData{Err: errors.New("Q2 failed")})

	err, sql := log.Snapshot()
	require.EqualError(t, err, "Q2 failed")
	require.Equal(t, "Q2", sql)
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
