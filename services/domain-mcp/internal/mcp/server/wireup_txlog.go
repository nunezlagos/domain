// Package mcpserver — captura de errores SQL durante tx para diagnóstico.
//
// Contexto: pgx Commit() devuelve pgx.ErrTxCommitRollback cuando postgres
// acepta COMMIT sobre una tx abortada y el command tag es "ROLLBACK"
// (tx.go:189). pgx ya NO expone el error SQL original de la query que
// abortó la tx. Si un handler ignora errores con `_ = err`, el wireup
// recibe nil-error del handler, intenta Commit, recibe ROLLBACK, y
// devuelve "transaction aborted before commit" sin saber QUÉ falló.
//
// Solución: un pgx.QueryTracer global (instalado en el pool via
// ConnConfig.Tracer) escribe el ÚLTIMO error SQL de cada query a un
// SQLErrorLog atado al ctx. El wireup, antes de Commit, chequea
// TxStatus() del conn; si es 'E' (aborted), lee el SQLErrorLog y lo
// surface al cliente.
//
// El tracer NO reemplaza el manejo de errores del handler — sigue siendo
// responsabilidad del handler devolver err en queries criticas. Esto es
// puro observability: si una query aborta la tx, podemos decir CUAL.
package mcpserver

import (
	"context"
	"sync"

	"github.com/jackc/pgx/v5"
)

// txErrLogKey es la key de ctx para el SQLErrorLog per-call.
type txErrLogKey struct{}

// SQLErrorLog captura el ÚLTIMO error SQL dentro de la tx actual.
// Se accede desde el tracer (write) y el wireup (read). Mutex interno
// para soportar pgxpool que puede multiplexar queries a distintos conns
// bajo el mismo ctx si el caller lo permite (raro, pero Defense-in-Depth).
type SQLErrorLog struct {
	mu  sync.Mutex
	err error
	sql string
}

// Record guarda el (err, sql) si err != nil. Last-write-wins — el tracer
// corre async al Exec y la ÚLTIMA query que falla es la que importa para
// diagnosticar un rollback.
func (l *SQLErrorLog) Record(err error, sql string) {
	if err == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.err = err
	l.sql = sql
}

// Snapshot devuelve (err, sql) actuales. Err puede ser nil si ninguna
// query fallo dentro de esta tx.
func (l *SQLErrorLog) Snapshot() (error, string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.err, l.sql
}

// withSQLErrorLog devuelve un ctx donde podemos registrar errores SQL.
// Llamar ANTES de BeginTx — pgx propaga el ctx al tracer en cada query.
func withSQLErrorLog(ctx context.Context) (context.Context, *SQLErrorLog) {
	log := &SQLErrorLog{}
	return context.WithValue(ctx, txErrLogKey{}, log), log
}

// sqLErrorLogFromContext recupera el log attached via withSQLErrorLog.
// Devuelve nil si el ctx no tiene uno (p.ej. handlers que no pasaron por
// withOrgCtx).
func sqLErrorLogFromContext(ctx context.Context) *SQLErrorLog {
	log, _ := ctx.Value(txErrLogKey{}).(*SQLErrorLog)
	return log
}

// sqlErrorCaptureTracer implementa pgx.QueryTracer y captura errores en
// el SQLErrorLog asociado al ctx. Solo el último error importa — si
// dentro de la misma tx hubo queries ok y luego una falla, es la falla
// la que aborta y lo que el wireup necesita ver.
//
// Nota pgx v5.5.x: TraceQueryEndData NO incluye el campo SQL — solo
// CommandTag y Err. Para anexar el SQL al error, lo capturamos en
// TraceQueryStart y lo guardamos en el ctx; TraceQueryEnd lo lee y lo
// entrega al SQLErrorLog junto con el err.
//
// Install (en internal/db/pools.go):
//
//	cfg, _ := pgxpool.ParseConfig(dsn)
//	cfg.ConnConfig.Tracer = mcpserver.SQLErrorCaptureTracer()
//	pool, _ := pgxpool.NewWithConfig(ctx, cfg)
type sqlErrorCaptureTracer struct{}

// SQLErrorCaptureTracer devuelve la única instancia compartida del tracer.
// pgx.ConnConfig.Tracer es del tipo pgx.QueryTracer interface; el struct
// sin campos es seguro de reusar concurrentemente (no tiene estado).
func SQLErrorCaptureTracer() pgx.QueryTracer { return sqlErrorCaptureTracer{} }

// tracedSQLKey es la key de ctx donde TraceQueryStart guarda el SQL
// actual; TraceQueryEnd lo lee para anexarlo al error.
type tracedSQLKey struct{}

func (sqlErrorCaptureTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, tracedSQLKey{}, data.SQL)
}

func (sqlErrorCaptureTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	if data.Err == nil {
		return
	}
	if log := sqLErrorLogFromContext(ctx); log != nil {
		sql, _ := ctx.Value(tracedSQLKey{}).(string)
		log.Record(data.Err, sql)
	}
}
