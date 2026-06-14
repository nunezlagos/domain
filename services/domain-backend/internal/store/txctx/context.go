package txctx

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// txKey es la key privada usada para inyectar la pgx.Tx en context.Context.
// Privada al package: ningún caller externo puede colisionar el slot.
type txKey struct{}

// WithTxContext inyecta la tx en el ctx para que repos y servicios la
// extraigan con TxFromContext. Usado por el middleware HTTP post-auth
// (issue-25.14) y por el MCP wireup para que la tx con SET LOCAL
// fluya transparente desde auth hasta el query.
func WithTxContext(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// TxFromContext retorna la pgx.Tx si fue inyectada por el wireup, o nil
// si no hay (legacy path o endpoints allowlisted).
//
// Patrón de uso en repos:
//
//	if tx := txctx.TxFromContext(ctx); tx != nil {
//	    return s.queryWithTx(ctx, tx, ...)
//	}
//	return s.queryWithPool(ctx, ...)
func TxFromContext(ctx context.Context) pgx.Tx {
	if ctx == nil {
		return nil
	}
	tx, _ := ctx.Value(txKey{}).(pgx.Tx)
	return tx
}

// ErrNoTxInContext se retorna por MustTxFromContext si no hay tx inyectada.
// Útil para endpoints que REQUIEREN wireup (e.g. handlers que tocan tablas
// con FORCE RLS — sin tx devuelven 0 rows y el caller no entiende por qué).
var ErrNoTxInContext = errors.New("txctx: no pgx.Tx in context (middleware wireup missing)")

// MustTxFromContext retorna la tx o error tipado. Para endpoints estrictos
// que NO deben correr sin wireup activo.
func MustTxFromContext(ctx context.Context) (pgx.Tx, error) {
	tx := TxFromContext(ctx)
	if tx == nil {
		return nil, ErrNoTxInContext
	}
	return tx, nil
}
