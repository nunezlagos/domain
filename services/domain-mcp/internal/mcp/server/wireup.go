
package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/store/txctx"
)

// withOrgCtx abre una tx con SET LOCAL app.current_org_id y
// app.current_user_id, e inyecta la tx en el ctx.
//
// Retorna:
//   - ctx enriquecido con la tx (repos la extraen con txctx.TxFromContext)
//     Y con un SQLErrorLog atado via ctx (ver wireup_txlog.go) — el
//     tracer global del pool escribe aqui errores SQL que el handler
//     pueda ignorar con `_ = err`.
//   - la tx misma (por si el caller quiere Commit explicito; sino defer
//     release() hace Rollback)
//   - release func: el caller DEBE llamarla (defer release()).
//     Si el caller hizo Commit, release es no-op. Si no, Rollback.
//
// Si el principal es nil o el pool es nil, retorna (ctx, nil, noop) sin
// wireup (legacy: queries usan pool directo, RLS no aplica — util para
// tools admin que corren con app_admin BYPASSRLS).
func withOrgCtx(ctx context.Context, pool *pgxpool.Pool, principal *apikey.Principal) (context.Context, pgx.Tx, func()) {
	noop := func() {}
	if pool == nil || principal == nil {
		return ctx, nil, noop
	}
	orgID, orgErr := uuid.Parse(principal.OrganizationID)
	userID, userErr := uuid.Parse(principal.UserID)
	if orgErr != nil || userErr != nil || orgID == uuid.Nil {
		return ctx, nil, noop
	}
	txCtx, sqLog := withSQLErrorLog(ctx)
	_ = sqLog
	tx, err := pool.BeginTx(txCtx, pgx.TxOptions{})
	if err != nil {
		return ctx, nil, noop
	}
	if _, err := tx.Exec(txCtx,
		`SELECT set_config('app.current_org_id', $1, true), set_config('app.current_user_id', $2, true)`,
		orgID.String(), userID.String()); err != nil {
		_ = tx.Rollback(txCtx)
		return ctx, nil, noop
	}
	release := func() { _ = tx.Rollback(txCtx) }
	return txctx.WithTxContext(txCtx, tx), tx, release
}

// withOrgTxHandler envuelve un tool handler con el wireup RLS completo
// (issue-25.14 + cierre 25.5 Tier-1): abre tx con SET LOCAL
// app.current_org_id/user_id, la inyecta en el ctx (los services la
// toman via txctx.TxFromContext) y COMMITEA si el tool termino sin
// error. Aplicar a todo tool que toque tablas con RLS FORCE
// (observations, sessions y las de 000028).
//
// Si Commit devuelve pgx.ErrTxCommitRollback (Postgres aceptó COMMIT
// sobre tx abortada y devolvió command tag "ROLLBACK") ANTES de hacer
// Commit se chequea TxStatus() del conn: si es 'E', la tx está
// abortada y se surface el ultimo error SQL capturado por el tracer
// global (ver wireup_txlog.go) via SQLErrorLog atado al ctx.
func withOrgTxHandler(d *Deps, h mcpgo.ToolHandlerFunc) mcpgo.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		txCtx, tx, release := withOrgCtx(ctx, d.Pool, d.Principal)
		defer release()
		result, err := h(txCtx, req)
		if tx != nil && err == nil && (result == nil || !result.IsError) {
			// Pre-check: si la tx ya quedó en estado aborted ('E'), devolver
			// el ultimo error SQL capturado por el tracer — el "ROLLBACK" de
			// pgx en si no dice qué falló.
			if status := tx.Conn().PgConn().TxStatus(); status == 'E' {
				if log := sqLErrorLogFromContext(txCtx); log != nil {
					if errs, sqls := log.Snapshot(); len(errs) > 0 {
						return mcp.NewToolResultError(formatSQLErrorChain(errs, sqls, "transaction aborted before commit")), nil
					}
				}
				return mcp.NewToolResultError("transaction aborted before commit; no SQL error captured by tracer (¿pool sin ConnConfig.Tracer?)"), nil
			}
			if cerr := tx.Commit(txCtx); cerr != nil {
				if errors.Is(cerr, pgx.ErrTxCommitRollback) {
					// tx aborted mid-flight (caso raro: el precheck de arriba no detectó
					// 'E' pero Commit devuelve ROLLBACK — p.ej. trigger ON COMMIT o
					// constraint deferrable). Caer al SQL log igual para diagnóstico.
					if log := sqLErrorLogFromContext(txCtx); log != nil {
						if errs, sqls := log.Snapshot(); len(errs) > 0 {
							return mcp.NewToolResultError(formatSQLErrorChain(errs, sqls, "transaction aborted before commit (Rollback)")), nil
						}
					}
					return mcp.NewToolResultError(fmt.Sprintf("transaction aborted before commit (Rollback): %v", cerr)), nil
				}
				return mcp.NewToolResultError(fmt.Sprintf("commit failed: %v", cerr)), nil
			}
		}
		return result, err
	}
}

// truncateSQL compacta whitespace en SQL para mensajes de error. SQL real
// rara vez excede 1KB; el cap de 240 chars es suficiente para reconocer
// la query en logs y dispara el ojo a "wheres" o joins.
func truncateSQL(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "... (truncated)"
}

// formatSQLErrorChain arma el mensaje que el wireup surface al cliente MCP
// cuando la tx aborta. Lista TODOS los errores capturados por el tracer en
// orden de aparicion, con indice [i/N] y SQL truncado. El primero es la
// causa raiz cuando hay cascade de 25P02 (ver HU issue-51.1).
func formatSQLErrorChain(errs []error, sqls []string, prefix string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s; %d SQL error(s) captured:\n", prefix, len(errs))
	total := len(errs)
	for i, e := range errs {
		fmt.Fprintf(&sb, "  [%d/%d] %v\n  in query: %s\n", i+1, total, e, truncateSQL(sqls[i], 240))
	}
	return sb.String()
}

// q retorna la tx del contexto (wireup activo) o el Pool como fallback.
// Para queries directas de los handlers MCP sobre tablas con RLS.
// Incluye Exec para INSERT/UPDATE/DELETE (REQ-45 session_bootstrap).
func (d *Deps) q(ctx context.Context) interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
} {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return tx
	}
	return d.Pool
}

// commitOrRollback es un helper de uso comun: si err es nil y tx no es nil,
// intenta Commit; si Commit falla, Rollback. Si err != nil, Rollback.
// Pensado para envolver la llamada al servicio desde el handler MCP.
func commitOrRollback(ctx context.Context, tx pgx.Tx, err error) error {
	if tx == nil {
		return err
	}
	if err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if cerr := tx.Commit(ctx); cerr != nil {
		return fmt.Errorf("commit: %w", cerr)
	}
	return nil
}
